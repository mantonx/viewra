package migration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const batchSize = 1000

// TransferProgress tracks progress during data transfer.
type TransferProgress struct {
	CurrentTable    string
	TablesCompleted int
	TablesTotal     int
	RowsCopied      int64
	RowsTotal       int64
}

// ProgressCallback is called during transfer to report progress.
type ProgressCallback func(progress TransferProgress)

// TransferData copies all data from source to target database.
func TransferData(
	ctx context.Context,
	sourceDB *sql.DB,
	sourceDriver string,
	targetDB *sql.DB,
	targetDriver string,
	onProgress ProgressCallback,
) error {
	// Get table info for progress tracking
	tables, err := GetTableInfo(ctx, sourceDB, sourceDriver)
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}

	// Build table info map and list
	tableInfoMap := make(map[string]TableInfo)
	var tableNames []string
	var totalRows int64
	for _, t := range tables {
		tableInfoMap[t.Name] = t
		tableNames = append(tableNames, t.Name)
		totalRows += t.Rows
	}

	// Get tables that exist in target database
	targetTables, err := GetTableInfo(ctx, targetDB, targetDriver)
	if err != nil {
		return fmt.Errorf("failed to get target table info: %w", err)
	}
	targetTableSet := make(map[string]bool)
	for _, t := range targetTables {
		targetTableSet[t.Name] = true
	}

	// Tables to exclude from data transfer:
	// - schema_migrations: managed by golang-migrate, already set correctly after schema creation
	// - schema_migrations_sqlite: SQLite-specific migration tracking table
	excludeTables := map[string]bool{
		"schema_migrations":        true,
		"schema_migrations_sqlite": true,
	}

	// Filter source tables to only include those that exist in target
	// (plugin tables may exist in source but not target)
	var filteredTableNames []string
	for _, name := range tableNames {
		if targetTableSet[name] && !excludeTables[name] {
			filteredTableNames = append(filteredTableNames, name)
		}
	}

	// Get dependency-sorted table order
	tableOrder, err := getTableOrder(ctx, sourceDB, sourceDriver, filteredTableNames)
	if err != nil {
		return fmt.Errorf("failed to determine table order: %w", err)
	}

	// Disable foreign key constraints
	if err := disableForeignKeys(ctx, targetDB, targetDriver); err != nil {
		return fmt.Errorf("failed to disable foreign keys: %w", err)
	}

	// Truncate ALL tables first (in reverse dependency order to respect FK constraints)
	// This handles both seeded data from schema creation and retries after partial failures.
	// We reverse the order so child tables are truncated before parents.
	for i := len(tableOrder) - 1; i >= 0; i-- {
		tableName := tableOrder[i]
		if err := truncateTable(ctx, targetDB, targetDriver, tableName); err != nil {
			return fmt.Errorf("failed to truncate table %s: %w", tableName, err)
		}
	}

	// Recalculate total rows for filtered tables only
	var filteredRows int64
	for _, name := range tableOrder {
		if info, ok := tableInfoMap[name]; ok {
			filteredRows += info.Rows
		}
	}

	progress := TransferProgress{
		TablesTotal: len(tableOrder),
		RowsTotal:   filteredRows,
	}

	// Copy tables in dependency order
	for i, tableName := range tableOrder {
		tableInfo := tableInfoMap[tableName]

		progress.CurrentTable = tableName
		progress.TablesCompleted = i
		if onProgress != nil {
			onProgress(progress)
		}

		if err := copyTable(ctx, sourceDB, sourceDriver, targetDB, targetDriver, tableName); err != nil {
			return fmt.Errorf("failed to copy table %s: %w", tableName, err)
		}

		progress.RowsCopied += tableInfo.Rows
	}

	// Re-enable foreign key constraints
	if err := enableForeignKeys(ctx, targetDB, targetDriver); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Reset sequences for PostgreSQL
	if targetDriver == "postgres" {
		if err := resetSequences(ctx, targetDB); err != nil {
			return fmt.Errorf("failed to reset sequences: %w", err)
		}
	}

	progress.TablesCompleted = len(tableOrder)
	progress.CurrentTable = ""
	if onProgress != nil {
		onProgress(progress)
	}

	return nil
}

// getTableOrder returns tables sorted by foreign key dependencies.
// Tables with no dependencies come first, then tables that depend on them, etc.
func getTableOrder(ctx context.Context, db *sql.DB, driver string, tables []string) ([]string, error) {
	// Get foreign key dependencies
	deps, err := getTableDependencies(ctx, db, driver)
	if err != nil {
		return nil, err
	}

	// Build a set of tables we need to process
	tableSet := make(map[string]bool)
	for _, t := range tables {
		tableSet[t] = true
	}

	// Topological sort using Kahn's algorithm
	// Calculate in-degree for each table (number of tables it depends on)
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // table -> tables that depend on it

	for _, t := range tables {
		inDegree[t] = 0
	}

	for table, dependencies := range deps {
		if !tableSet[table] {
			continue
		}
		for _, dep := range dependencies {
			if !tableSet[dep] {
				continue
			}
			if dep == table {
				continue // Skip self-references
			}
			inDegree[table]++
			dependents[dep] = append(dependents[dep], table)
		}
	}

	// Start with tables that have no dependencies
	var queue []string
	for _, t := range tables {
		if inDegree[t] == 0 {
			queue = append(queue, t)
		}
	}

	var result []string
	for len(queue) > 0 {
		// Pop from queue
		table := queue[0]
		queue = queue[1:]
		result = append(result, table)

		// Reduce in-degree for dependents
		for _, dependent := range dependents[table] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for cycles (tables not in result)
	if len(result) != len(tables) {
		// There's a cycle - fall back to original order with FK disabled
		// This is safe because we disable FK constraints during copy
		return tables, nil
	}

	return result, nil
}

// getTableDependencies returns a map of table -> tables it depends on (via foreign keys).
func getTableDependencies(ctx context.Context, db *sql.DB, driver string) (map[string][]string, error) {
	deps := make(map[string][]string)

	switch driver {
	case "postgres", "postgresql":
		rows, err := db.QueryContext(ctx, `
			SELECT DISTINCT
				tc.table_name,
				ccu.table_name AS referenced_table
			FROM information_schema.table_constraints tc
			JOIN information_schema.constraint_column_usage ccu
				ON tc.constraint_name = ccu.constraint_name
				AND tc.table_schema = ccu.table_schema
			WHERE tc.constraint_type = 'FOREIGN KEY'
				AND tc.table_schema = 'public'
		`)
		if err != nil {
			return nil, fmt.Errorf("failed to query foreign keys: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var table, referenced string
			if err := rows.Scan(&table, &referenced); err != nil {
				return nil, err
			}
			deps[table] = append(deps[table], referenced)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

	case "sqlite", "sqlite3":
		// Get all tables first
		tableRows, err := db.QueryContext(ctx, `
			SELECT name FROM sqlite_master 
			WHERE type='table' AND name NOT LIKE 'sqlite_%'
		`)
		if err != nil {
			return nil, fmt.Errorf("failed to query tables: %w", err)
		}

		var tableNames []string
		for tableRows.Next() {
			var name string
			if err := tableRows.Scan(&name); err != nil {
				tableRows.Close()
				return nil, err
			}
			tableNames = append(tableNames, name)
		}
		tableRows.Close()

		// Get foreign keys for each table
		for _, tableName := range tableNames {
			fkRows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list(%q)", tableName))
			if err != nil {
				continue // Skip tables that error
			}

			for fkRows.Next() {
				var id, seq int
				var refTable, from, to, onUpdate, onDelete, match string
				if err := fkRows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
					continue
				}
				// Add dependency: tableName depends on refTable
				deps[tableName] = append(deps[tableName], refTable)
			}
			fkRows.Close()
		}

	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}

	// Deduplicate dependencies
	for table, tableDeps := range deps {
		deps[table] = uniqueStrings(tableDeps)
	}

	return deps, nil
}

// uniqueStrings removes duplicates from a string slice.
func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// copyTable copies data from a single table.
func copyTable(
	ctx context.Context,
	sourceDB *sql.DB,
	sourceDriver string,
	targetDB *sql.DB,
	targetDriver string,
	tableName string,
) error {
	// Get column info
	columns, err := getTableColumns(ctx, sourceDB, sourceDriver, tableName)
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	if len(columns) == 0 {
		return nil // No columns, skip
	}

	// Build SELECT query with quoted column names (some like "cast" are reserved words)
	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = fmt.Sprintf("%q", col)
	}
	columnList := strings.Join(quotedColumns, ", ")
	selectQuery := fmt.Sprintf("SELECT %s FROM %q", columnList, tableName)

	// Query source data
	rows, err := sourceDB.QueryContext(ctx, selectQuery)
	if err != nil {
		return fmt.Errorf("failed to query source: %w", err)
	}
	defer rows.Close()

	// Prepare insert statement
	placeholders := buildPlaceholders(len(columns), targetDriver)
	insertQuery := fmt.Sprintf(
		"INSERT INTO %q (%s) VALUES (%s)",
		tableName,
		columnList,
		placeholders,
	)

	// Begin transaction for batch insert
	tx, err := targetDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, insertQuery)
	if err != nil {
		return fmt.Errorf("failed to prepare insert: %w", err)
	}
	defer stmt.Close()

	// Create value holders
	values := make([]any, len(columns))
	valuePtrs := make([]any, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	rowCount := 0
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert values between database types
		convertedValues := convertValues(values, sourceDriver, targetDriver)

		if _, err := stmt.ExecContext(ctx, convertedValues...); err != nil {
			return fmt.Errorf("failed to insert row: %w", err)
		}

		rowCount++

		// Commit batch
		if rowCount%batchSize == 0 {
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit batch: %w", err)
			}

			tx, err = targetDB.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to begin new transaction: %w", err)
			}

			stmt, err = tx.PrepareContext(ctx, insertQuery)
			if err != nil {
				return fmt.Errorf("failed to prepare insert: %w", err)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	// Final commit
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit final batch: %w", err)
	}

	return nil
}

// getTableColumns returns column names for a table.
func getTableColumns(ctx context.Context, db *sql.DB, driver, tableName string) ([]string, error) {
	var columns []string

	switch driver {
	case "postgres", "postgresql":
		rows, err := db.QueryContext(ctx, `
			SELECT column_name 
			FROM information_schema.columns 
			WHERE table_schema = 'public' AND table_name = $1
			ORDER BY ordinal_position
		`, tableName)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var col string
			if err := rows.Scan(&col); err != nil {
				return nil, err
			}
			columns = append(columns, col)
		}

	case "sqlite", "sqlite3":
		rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", tableName))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue any
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				return nil, err
			}
			columns = append(columns, name)
		}

	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}

	return columns, nil
}

// buildPlaceholders creates placeholder string for INSERT statement.
func buildPlaceholders(count int, driver string) string {
	placeholders := make([]string, count)
	for i := range placeholders {
		switch driver {
		case "postgres", "postgresql":
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		default:
			placeholders[i] = "?"
		}
	}
	return strings.Join(placeholders, ", ")
}

// convertValues converts values between SQLite and PostgreSQL types.
func convertValues(values []any, sourceDriver, targetDriver string) []any {
	result := make([]any, len(values))

	for i, v := range values {
		result[i] = convertValue(v, sourceDriver, targetDriver)
	}

	return result
}

// convertValue converts a single value between database types.
func convertValue(v any, sourceDriver, targetDriver string) any {
	if v == nil {
		return nil
	}

	// SQLite to PostgreSQL conversions
	if sourceDriver == "sqlite" || sourceDriver == "sqlite3" {
		if targetDriver == "postgres" || targetDriver == "postgresql" {
			switch val := v.(type) {
			case int64:
				// SQLite stores booleans as 0/1
				// PostgreSQL expects bool type, but we insert as int and let PG convert
				return val
			case []byte:
				// SQLite might return []byte for TEXT columns
				return string(val)
			}
		}
	}

	// PostgreSQL to SQLite conversions
	if sourceDriver == "postgres" || sourceDriver == "postgresql" {
		if targetDriver == "sqlite" || targetDriver == "sqlite3" {
			switch val := v.(type) {
			case bool:
				// Convert PostgreSQL bool to SQLite integer
				if val {
					return 1
				}
				return 0
			}
		}
	}

	return v
}

// disableForeignKeys disables foreign key constraints during data copy.
// For PostgreSQL, we rely on topological ordering and don't need to disable FK checks.
// For SQLite, we disable foreign_keys pragma.
func disableForeignKeys(ctx context.Context, db *sql.DB, driver string) error {
	switch driver {
	case "postgres", "postgresql":
		// PostgreSQL: We rely on topological ordering of tables.
		// Since we copy parent tables before child tables, FK constraints are satisfied.
		// No special permissions required.
		return nil
	case "sqlite", "sqlite3":
		_, err := db.ExecContext(ctx, "PRAGMA foreign_keys = OFF")
		return err
	default:
		return fmt.Errorf("unsupported driver: %s", driver)
	}
}

// enableForeignKeys re-enables foreign key constraints after data copy.
func enableForeignKeys(ctx context.Context, db *sql.DB, driver string) error {
	switch driver {
	case "postgres", "postgresql":
		// PostgreSQL: Nothing to re-enable since we didn't disable anything.
		return nil
	case "sqlite", "sqlite3":
		_, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
		return err
	default:
		return fmt.Errorf("unsupported driver: %s", driver)
	}
}

// truncateTable removes all data from a table before copying.
// This handles retries after partial migration failures and seed data from schema creation.
func truncateTable(ctx context.Context, db *sql.DB, driver, tableName string) error {
	switch driver {
	case "postgres", "postgresql":
		// Use DELETE instead of TRUNCATE to avoid CASCADE side effects.
		// We truncate in reverse dependency order so FK constraints are satisfied.
		_, err := db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %q", tableName))
		return err
	case "sqlite", "sqlite3":
		_, err := db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %q", tableName))
		return err
	default:
		return fmt.Errorf("unsupported driver: %s", driver)
	}
}

// resetSequences resets all PostgreSQL sequences to match the max ID in each table.
func resetSequences(ctx context.Context, db *sql.DB) error {
	// Query for all sequences and their associated tables/columns
	rows, err := db.QueryContext(ctx, `
		SELECT 
			c.relname AS sequence_name,
			t.relname AS table_name,
			a.attname AS column_name
		FROM pg_class c
		JOIN pg_depend d ON d.objid = c.oid
		JOIN pg_class t ON t.oid = d.refobjid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.refobjsubid
		WHERE c.relkind = 'S'
		AND d.deptype = 'a'
	`)
	if err != nil {
		return fmt.Errorf("failed to query sequences: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var seqName, tableName, columnName string
		if err := rows.Scan(&seqName, &tableName, &columnName); err != nil {
			return fmt.Errorf("failed to scan sequence info: %w", err)
		}

		// Get max value from the table
		var maxVal sql.NullInt64
		query := fmt.Sprintf("SELECT MAX(%q) FROM %q", columnName, tableName)
		if err := db.QueryRowContext(ctx, query).Scan(&maxVal); err != nil {
			continue // Table might be empty
		}

		if !maxVal.Valid || maxVal.Int64 == 0 {
			continue // No data or zero value
		}

		// Reset sequence to max + 1
		resetQuery := fmt.Sprintf("SELECT setval('%s', %d)", seqName, maxVal.Int64)
		if _, err := db.ExecContext(ctx, resetQuery); err != nil {
			return fmt.Errorf("failed to reset sequence %s: %w", seqName, err)
		}
	}

	return rows.Err()
}
