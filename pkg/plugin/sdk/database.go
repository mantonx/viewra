// Package sdk provides database utilities for ViewRA plugins.
//
// The SQLClient provides managed SQL storage where the host handles table
// namespacing, connection pooling, and security enforcement. Plugins don't
// need to manage their own database connections or bundle SQLite drivers.
//
// # Usage
//
//	func (p *MyPlugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
//	    db := p.storage.SQL()
//
//	    // Run migrations
//	    err := db.Migrate(ctx, []sdk.Migration{
//	        {Version: 1, SQL: `CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`},
//	    })
//	    if err != nil {
//	        return nil, err
//	    }
//
//	    p.db = db
//	    return &pluginv1.InitResponse{Success: true}, nil
//	}
//
// # Table Namespacing
//
// All table names are automatically prefixed by the host with plugin_{id}_
// For example, if your plugin ID is "semantic-search" and you create a table
// named "embeddings", the actual table name will be "plugin_semantic_search_embeddings".
//
// You don't need to worry about this - just use your table names as-is.
//
// # Dual Database Compatibility
//
// SQL must work on both PostgreSQL and SQLite. Stick to common SQL features:
//   - Basic types: TEXT, INTEGER, REAL, BLOB
//   - CREATE TABLE, CREATE INDEX, DROP TABLE, DROP INDEX
//   - PRIMARY KEY, UNIQUE, NOT NULL, DEFAULT
//   - INSERT, UPDATE, DELETE, SELECT
//   - WHERE, ORDER BY, LIMIT, OFFSET
//   - JOIN, LEFT JOIN
//   - COUNT, MAX, MIN, SUM, AVG
//
// Avoid:
//   - RETURNING clause (use LastInsertID instead)
//   - Array types
//   - SERIAL (use INTEGER PRIMARY KEY for auto-increment)
//   - Database-specific JSON operators
package sdk

import (
	"context"
	"errors"
	"fmt"
	"sort"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// SQLClient provides managed SQL storage for plugins.
// All table names are automatically prefixed by the host with plugin_{id}_
type SQLClient struct {
	client pluginv1.HostStorageClient
}

// newSQLClient creates a new SQL client. Called internally by StorageClient.
func newSQLClient(client pluginv1.HostStorageClient) *SQLClient {
	return &SQLClient{client: client}
}

// Exec executes a DDL or DML statement (CREATE, INSERT, UPDATE, DELETE).
// Returns the number of rows affected and the last insert ID (if applicable).
//
// Example:
//
//	rowsAffected, lastID, err := db.Exec(ctx,
//	    `INSERT INTO items (name, value) VALUES (?, ?)`,
//	    "test", 42)
func (c *SQLClient) Exec(ctx context.Context, sql string, args ...any) (rowsAffected int64, lastInsertID int64, err error) {
	sqlArgs, err := toSQLValues(args)
	if err != nil {
		return 0, 0, fmt.Errorf("converting args: %w", err)
	}

	resp, err := c.client.ExecuteSQL(ctx, &pluginv1.SQLRequest{
		Sql:  sql,
		Args: sqlArgs,
	})
	if err != nil {
		return 0, 0, err
	}

	return resp.RowsAffected, resp.LastInsertId, nil
}

// Query executes a SELECT statement and returns an iterator over the results.
//
// Example:
//
//	rows, err := db.Query(ctx, `SELECT id, name FROM items WHERE value > ?`, 10)
//	if err != nil {
//	    return err
//	}
//	defer rows.Close()
//
//	for rows.Next() {
//	    var id int64
//	    var name string
//	    if err := rows.Scan(&id, &name); err != nil {
//	        return err
//	    }
//	    // use id, name
//	}
//	return rows.Err()
func (c *SQLClient) Query(ctx context.Context, sql string, args ...any) (*Rows, error) {
	sqlArgs, err := toSQLValues(args)
	if err != nil {
		return nil, fmt.Errorf("converting args: %w", err)
	}

	resp, err := c.client.QuerySQL(ctx, &pluginv1.SQLRequest{
		Sql:  sql,
		Args: sqlArgs,
	})
	if err != nil {
		return nil, err
	}

	return &Rows{
		columns: resp.Columns,
		rows:    resp.Rows,
		index:   -1,
	}, nil
}

// QueryRow executes a SELECT expected to return at most one row.
// Returns a Row that can be scanned. If the query returns no rows,
// Scan will return ErrNoRows.
//
// Example:
//
//	var name string
//	err := db.QueryRow(ctx, `SELECT name FROM items WHERE id = ?`, 1).Scan(&name)
//	if errors.Is(err, sdk.ErrNoRows) {
//	    // not found
//	}
func (c *SQLClient) QueryRow(ctx context.Context, sql string, args ...any) *Row {
	rows, err := c.Query(ctx, sql, args...)
	if err != nil {
		return &Row{err: err}
	}

	if !rows.Next() {
		return &Row{err: ErrNoRows}
	}

	return &Row{
		columns: rows.columns,
		values:  rows.rows[rows.index].Values,
	}
}

// Migration represents a schema migration.
type Migration struct {
	// Version is the migration version number. Must be unique and positive.
	// Migrations are applied in version order.
	Version int

	// SQL is the DDL statement(s) to execute for this migration.
	// Multiple statements can be separated by semicolons.
	SQL string
}

// Migrate runs schema migrations, applying any that haven't been run yet.
// Migrations are tracked in an _migrations table (auto-prefixed by host).
//
// Example:
//
//	err := db.Migrate(ctx, []sdk.Migration{
//	    {Version: 1, SQL: `CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`},
//	    {Version: 2, SQL: `ALTER TABLE items ADD COLUMN created_at TIMESTAMP`},
//	    {Version: 3, SQL: `CREATE INDEX idx_items_name ON items(name)`},
//	})
func (c *SQLClient) Migrate(ctx context.Context, migrations []Migration) error {
	if len(migrations) == 0 {
		return nil
	}

	// Ensure migrations table exists
	_, _, err := c.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Get current version
	var currentVersion int64
	row := c.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM _migrations`)
	if err := row.Scan(&currentVersion); err != nil && !errors.Is(err, ErrNoRows) {
		return fmt.Errorf("getting current version: %w", err)
	}

	// Sort migrations by version
	sorted := make([]Migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	// Apply pending migrations
	for _, m := range sorted {
		if m.Version <= 0 {
			return fmt.Errorf("migration version must be positive, got %d", m.Version)
		}
		if int64(m.Version) <= currentVersion {
			continue
		}

		// Execute migration SQL
		if _, _, err := c.Exec(ctx, m.SQL); err != nil {
			return fmt.Errorf("migration %d failed: %w", m.Version, err)
		}

		// Record migration
		if _, _, err := c.Exec(ctx, `INSERT INTO _migrations (version) VALUES (?)`, m.Version); err != nil {
			return fmt.Errorf("recording migration %d: %w", m.Version, err)
		}
	}

	return nil
}

// ErrNoRows is returned by QueryRow when no rows are found.
var ErrNoRows = errors.New("sql: no rows in result set")

// Rows is an iterator over query results.
type Rows struct {
	columns []string
	rows    []*pluginv1.SQLRow
	index   int
	err     error
}

// Next advances to the next row. Returns false when there are no more rows.
func (r *Rows) Next() bool {
	if r.err != nil {
		return false
	}
	r.index++
	return r.index < len(r.rows)
}

// Scan copies the current row's columns into the provided destinations.
// The number of destinations must match the number of columns.
//
// Supported destination types:
//   - *string
//   - *int, *int64, *int32
//   - *float64, *float32
//   - *[]byte
//   - *bool
//   - *any (receives the raw value)
func (r *Rows) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if r.index < 0 || r.index >= len(r.rows) {
		return errors.New("sql: Scan called without calling Next")
	}

	row := r.rows[r.index]
	if len(dest) != len(row.Values) {
		return fmt.Errorf("sql: expected %d destination arguments, got %d", len(row.Values), len(dest))
	}

	for i, val := range row.Values {
		if err := scanValue(val, dest[i]); err != nil {
			return fmt.Errorf("sql: scanning column %d (%s): %w", i, r.columns[i], err)
		}
	}

	return nil
}

// Columns returns the column names.
func (r *Rows) Columns() []string {
	return r.columns
}

// Close closes the rows iterator. Always call this when done.
func (r *Rows) Close() error {
	// No-op for RPC-based rows, but good practice to call
	return nil
}

// Err returns any error that occurred during iteration.
func (r *Rows) Err() error {
	return r.err
}

// Row is a single result row from QueryRow.
type Row struct {
	columns []string
	values  []*pluginv1.SQLValue
	err     error
}

// Scan copies the row's columns into the provided destinations.
func (r *Row) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}

	if len(dest) != len(r.values) {
		return fmt.Errorf("sql: expected %d destination arguments, got %d", len(r.values), len(dest))
	}

	for i, val := range r.values {
		if err := scanValue(val, dest[i]); err != nil {
			colName := ""
			if i < len(r.columns) {
				colName = r.columns[i]
			}
			return fmt.Errorf("sql: scanning column %d (%s): %w", i, colName, err)
		}
	}

	return nil
}

// toSQLValues converts Go values to proto SQLValue messages.
func toSQLValues(args []any) ([]*pluginv1.SQLValue, error) {
	result := make([]*pluginv1.SQLValue, len(args))
	for i, arg := range args {
		val, err := toSQLValue(arg)
		if err != nil {
			return nil, fmt.Errorf("arg %d: %w", i, err)
		}
		result[i] = val
	}
	return result, nil
}

func toSQLValue(v any) (*pluginv1.SQLValue, error) {
	if v == nil {
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IsNull{IsNull: true}}, nil
	}

	switch val := v.(type) {
	case string:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_StringValue{StringValue: val}}, nil
	case int:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IntValue{IntValue: int64(val)}}, nil
	case int32:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IntValue{IntValue: int64(val)}}, nil
	case int64:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IntValue{IntValue: val}}, nil
	case float32:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_DoubleValue{DoubleValue: float64(val)}}, nil
	case float64:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_DoubleValue{DoubleValue: val}}, nil
	case []byte:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_BytesValue{BytesValue: val}}, nil
	case bool:
		// SQLite doesn't have native bool, store as int
		if val {
			return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IntValue{IntValue: 1}}, nil
		}
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IntValue{IntValue: 0}}, nil
	default:
		return nil, fmt.Errorf("unsupported type %T", v)
	}
}

func scanValue(val *pluginv1.SQLValue, dest any) error {
	// Handle NULL
	if val.GetIsNull() {
		// For NULL, we leave dest unchanged (zero value)
		// This matches database/sql behavior
		return nil
	}

	switch d := dest.(type) {
	case *string:
		switch v := val.Value.(type) {
		case *pluginv1.SQLValue_StringValue:
			*d = v.StringValue
		case *pluginv1.SQLValue_IntValue:
			*d = fmt.Sprintf("%d", v.IntValue)
		case *pluginv1.SQLValue_DoubleValue:
			*d = fmt.Sprintf("%f", v.DoubleValue)
		default:
			return fmt.Errorf("cannot scan %T into *string", val.Value)
		}

	case *int:
		switch v := val.Value.(type) {
		case *pluginv1.SQLValue_IntValue:
			*d = int(v.IntValue)
		case *pluginv1.SQLValue_DoubleValue:
			*d = int(v.DoubleValue)
		default:
			return fmt.Errorf("cannot scan %T into *int", val.Value)
		}

	case *int32:
		switch v := val.Value.(type) {
		case *pluginv1.SQLValue_IntValue:
			*d = int32(v.IntValue)
		default:
			return fmt.Errorf("cannot scan %T into *int32", val.Value)
		}

	case *int64:
		switch v := val.Value.(type) {
		case *pluginv1.SQLValue_IntValue:
			*d = v.IntValue
		case *pluginv1.SQLValue_DoubleValue:
			*d = int64(v.DoubleValue)
		default:
			return fmt.Errorf("cannot scan %T into *int64", val.Value)
		}

	case *float32:
		switch v := val.Value.(type) {
		case *pluginv1.SQLValue_DoubleValue:
			*d = float32(v.DoubleValue)
		case *pluginv1.SQLValue_IntValue:
			*d = float32(v.IntValue)
		default:
			return fmt.Errorf("cannot scan %T into *float32", val.Value)
		}

	case *float64:
		switch v := val.Value.(type) {
		case *pluginv1.SQLValue_DoubleValue:
			*d = v.DoubleValue
		case *pluginv1.SQLValue_IntValue:
			*d = float64(v.IntValue)
		default:
			return fmt.Errorf("cannot scan %T into *float64", val.Value)
		}

	case *[]byte:
		switch v := val.Value.(type) {
		case *pluginv1.SQLValue_BytesValue:
			*d = v.BytesValue
		case *pluginv1.SQLValue_StringValue:
			*d = []byte(v.StringValue)
		default:
			return fmt.Errorf("cannot scan %T into *[]byte", val.Value)
		}

	case *bool:
		switch v := val.Value.(type) {
		case *pluginv1.SQLValue_IntValue:
			*d = v.IntValue != 0
		default:
			return fmt.Errorf("cannot scan %T into *bool", val.Value)
		}

	case *any:
		switch v := val.Value.(type) {
		case *pluginv1.SQLValue_StringValue:
			*d = v.StringValue
		case *pluginv1.SQLValue_IntValue:
			*d = v.IntValue
		case *pluginv1.SQLValue_DoubleValue:
			*d = v.DoubleValue
		case *pluginv1.SQLValue_BytesValue:
			*d = v.BytesValue
		case *pluginv1.SQLValue_IsNull:
			*d = nil
		}

	default:
		return fmt.Errorf("unsupported destination type %T", dest)
	}

	return nil
}
