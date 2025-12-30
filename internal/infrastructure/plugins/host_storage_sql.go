package plugins

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// SQL execution configuration
const (
	// Maximum query timeout
	sqlQueryTimeout = 30 * time.Second

	// Maximum rows returned from a query
	sqlMaxRows = 10000
)

// Forbidden SQL patterns that could be security risks
var forbiddenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bATTACH\b`),
	regexp.MustCompile(`(?i)\bDETACH\b`),
	regexp.MustCompile(`(?i)\bPRAGMA\b`),
	regexp.MustCompile(`(?i)\bpg_\w+`),               // Postgres system tables
	regexp.MustCompile(`(?i)\bsqlite_\w+`),           // SQLite system tables
	regexp.MustCompile(`(?i)\binformation_schema\b`), // Standard system schema
	regexp.MustCompile(`(?i)\bVACUUM\b`),
	regexp.MustCompile(`(?i)\bANALYZE\b`),
	regexp.MustCompile(`(?i)\bREINDEX\b`),
	regexp.MustCompile(`(?i)\bALTER\s+DATABASE\b`),
	regexp.MustCompile(`(?i)\bCREATE\s+DATABASE\b`),
	regexp.MustCompile(`(?i)\bDROP\s+DATABASE\b`),
	regexp.MustCompile(`(?i)\bGRANT\b`),
	regexp.MustCompile(`(?i)\bREVOKE\b`),
}

// Table name patterns for rewriting
// These capture table names in common SQL contexts
var tablePatterns = []struct {
	pattern *regexp.Regexp
	replace func(prefix, match string) string
}{
	// CREATE TABLE [IF NOT EXISTS] name
	{
		pattern: regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z_][a-zA-Z0-9_]*)`),
		replace: func(prefix, match string) string {
			return strings.Replace(match, regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z_][a-zA-Z0-9_]*)`).FindStringSubmatch(match)[1],
				prefix+regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z_][a-zA-Z0-9_]*)`).FindStringSubmatch(match)[1], 1)
		},
	},
}

// ExecuteSQL runs DDL/DML statements on plugin's namespaced tables.
func (s *HostStorageServer) ExecuteSQL(ctx context.Context, req *pluginv1.SQLRequest) (*pluginv1.SQLExecResult, error) {
	pluginID := getPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if req.Sql == "" {
		return nil, errors.New("SQL statement is required")
	}

	s.logger.Debug("ExecuteSQL called", "plugin_id", pluginID, "sql", truncateSQL(req.Sql))

	// Check for forbidden patterns
	if err := validateSQL(req.Sql); err != nil {
		s.logger.Warn("forbidden SQL pattern detected", "plugin_id", pluginID, "error", err)
		return nil, err
	}

	// Rewrite table names with plugin prefix
	rewrittenSQL, err := rewriteTableNames(req.Sql, sanitizePluginID(pluginID))
	if err != nil {
		return nil, fmt.Errorf("failed to rewrite SQL: %w", err)
	}

	// Convert args
	args, err := sqlValuesToArgs(req.Args)
	if err != nil {
		return nil, fmt.Errorf("failed to convert args: %w", err)
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(ctx, sqlQueryTimeout)
	defer cancel()

	result, err := s.db.ExecContext(ctx, rewrittenSQL, args...)
	if err != nil {
		s.logger.Error("SQL execution failed", "plugin_id", pluginID, "error", err)
		return nil, fmt.Errorf("SQL execution failed: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	lastInsertID, _ := result.LastInsertId()

	return &pluginv1.SQLExecResult{
		RowsAffected: rowsAffected,
		LastInsertId: lastInsertID,
	}, nil
}

// QuerySQL runs SELECT queries on plugin's namespaced tables.
func (s *HostStorageServer) QuerySQL(ctx context.Context, req *pluginv1.SQLRequest) (*pluginv1.SQLQueryResult, error) {
	pluginID := getPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if req.Sql == "" {
		return nil, errors.New("SQL statement is required")
	}

	s.logger.Debug("QuerySQL called", "plugin_id", pluginID, "sql", truncateSQL(req.Sql))

	// Check for forbidden patterns
	if err := validateSQL(req.Sql); err != nil {
		s.logger.Warn("forbidden SQL pattern detected", "plugin_id", pluginID, "error", err)
		return nil, err
	}

	// Rewrite table names with plugin prefix
	rewrittenSQL, err := rewriteTableNames(req.Sql, sanitizePluginID(pluginID))
	if err != nil {
		return nil, fmt.Errorf("failed to rewrite SQL: %w", err)
	}

	// Convert args
	args, err := sqlValuesToArgs(req.Args)
	if err != nil {
		return nil, fmt.Errorf("failed to convert args: %w", err)
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(ctx, sqlQueryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, rewrittenSQL, args...)
	if err != nil {
		s.logger.Error("SQL query failed", "plugin_id", pluginID, "error", err)
		return nil, fmt.Errorf("SQL query failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Scan rows
	var resultRows []*pluginv1.SQLRow
	rowCount := 0

	for rows.Next() {
		if rowCount >= sqlMaxRows {
			s.logger.Warn("query result truncated", "plugin_id", pluginID, "max_rows", sqlMaxRows)
			break
		}

		// Create slice of interface{} to scan into
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert to SQLValue
		sqlValues := make([]*pluginv1.SQLValue, len(values))
		for i, v := range values {
			sqlValues[i] = toSQLValue(v)
		}

		resultRows = append(resultRows, &pluginv1.SQLRow{Values: sqlValues})
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return &pluginv1.SQLQueryResult{
		Columns: columns,
		Rows:    resultRows,
	}, nil
}

// validateSQL checks for forbidden SQL patterns
func validateSQL(sql string) error {
	for _, pattern := range forbiddenPatterns {
		if pattern.MatchString(sql) {
			return fmt.Errorf("forbidden SQL pattern: %s", pattern.String())
		}
	}
	return nil
}

// sanitizePluginID converts plugin ID to a safe table prefix
// e.g., "semantic-search" -> "semantic_search"
func sanitizePluginID(pluginID string) string {
	// Replace non-alphanumeric chars with underscore
	safe := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(pluginID, "_")
	// Remove consecutive underscores
	safe = regexp.MustCompile(`_+`).ReplaceAllString(safe, "_")
	// Trim underscores from ends
	safe = strings.Trim(safe, "_")
	return safe
}

// rewriteTableNames prefixes all table names with plugin_{id}_
func rewriteTableNames(sqlStr, pluginID string) (string, error) {
	prefix := "plugin_" + pluginID + "_"

	// Patterns to match table names in different SQL contexts
	// We use a simple but effective approach: find keywords followed by identifiers

	result := sqlStr

	// CREATE TABLE [IF NOT EXISTS] name
	result = regexp.MustCompile(`(?i)(\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?)([a-zA-Z_][a-zA-Z0-9_]*)`).
		ReplaceAllString(result, "${1}"+prefix+"$2")

	// DROP TABLE [IF EXISTS] name
	result = regexp.MustCompile(`(?i)(\bDROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?)([a-zA-Z_][a-zA-Z0-9_]*)`).
		ReplaceAllString(result, "${1}"+prefix+"$2")

	// CREATE [UNIQUE] INDEX [IF NOT EXISTS] name ON table
	result = regexp.MustCompile(`(?i)(\bCREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?)([a-zA-Z_][a-zA-Z0-9_]*)(\s+ON\s+)([a-zA-Z_][a-zA-Z0-9_]*)`).
		ReplaceAllString(result, "${1}"+prefix+"$2${3}"+prefix+"$4")

	// DROP INDEX [IF EXISTS] name
	result = regexp.MustCompile(`(?i)(\bDROP\s+INDEX\s+(?:IF\s+EXISTS\s+)?)([a-zA-Z_][a-zA-Z0-9_]*)`).
		ReplaceAllString(result, "${1}"+prefix+"$2")

	// FROM table (including JOIN variants)
	result = regexp.MustCompile(`(?i)(\bFROM\s+)([a-zA-Z_][a-zA-Z0-9_]*)`).
		ReplaceAllString(result, "${1}"+prefix+"$2")

	// JOIN table
	result = regexp.MustCompile(`(?i)(\bJOIN\s+)([a-zA-Z_][a-zA-Z0-9_]*)`).
		ReplaceAllString(result, "${1}"+prefix+"$2")

	// INTO table
	result = regexp.MustCompile(`(?i)(\bINTO\s+)([a-zA-Z_][a-zA-Z0-9_]*)`).
		ReplaceAllString(result, "${1}"+prefix+"$2")

	// UPDATE table
	result = regexp.MustCompile(`(?i)(\bUPDATE\s+)([a-zA-Z_][a-zA-Z0-9_]*)`).
		ReplaceAllString(result, "${1}"+prefix+"$2")

	// ALTER TABLE table (future-proofing)
	result = regexp.MustCompile(`(?i)(\bALTER\s+TABLE\s+)([a-zA-Z_][a-zA-Z0-9_]*)`).
		ReplaceAllString(result, "${1}"+prefix+"$2")

	// TRUNCATE [TABLE] table
	result = regexp.MustCompile(`(?i)(\bTRUNCATE\s+(?:TABLE\s+)?)([a-zA-Z_][a-zA-Z0-9_]*)`).
		ReplaceAllString(result, "${1}"+prefix+"$2")

	return result, nil
}

// sqlValuesToArgs converts proto SQLValue slice to []any for database/sql
func sqlValuesToArgs(values []*pluginv1.SQLValue) ([]any, error) {
	args := make([]any, len(values))
	for i, v := range values {
		args[i] = sqlValueToArg(v)
	}
	return args, nil
}

func sqlValueToArg(v *pluginv1.SQLValue) any {
	if v == nil {
		return nil
	}

	switch val := v.Value.(type) {
	case *pluginv1.SQLValue_StringValue:
		return val.StringValue
	case *pluginv1.SQLValue_IntValue:
		return val.IntValue
	case *pluginv1.SQLValue_DoubleValue:
		return val.DoubleValue
	case *pluginv1.SQLValue_BytesValue:
		return val.BytesValue
	case *pluginv1.SQLValue_IsNull:
		return nil
	default:
		return nil
	}
}

// toSQLValue converts a database value to proto SQLValue
func toSQLValue(v any) *pluginv1.SQLValue {
	if v == nil {
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IsNull{IsNull: true}}
	}

	switch val := v.(type) {
	case string:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_StringValue{StringValue: val}}
	case []byte:
		// Try to interpret as string first (common for TEXT columns)
		// If it looks like binary data, keep as bytes
		if isValidUTF8(val) {
			return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_StringValue{StringValue: string(val)}}
		}
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_BytesValue{BytesValue: val}}
	case int64:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IntValue{IntValue: val}}
	case int32:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IntValue{IntValue: int64(val)}}
	case int:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IntValue{IntValue: int64(val)}}
	case float64:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_DoubleValue{DoubleValue: val}}
	case float32:
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_DoubleValue{DoubleValue: float64(val)}}
	case bool:
		if val {
			return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IntValue{IntValue: 1}}
		}
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IntValue{IntValue: 0}}
	case sql.NullString:
		if val.Valid {
			return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_StringValue{StringValue: val.String}}
		}
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IsNull{IsNull: true}}
	case sql.NullInt64:
		if val.Valid {
			return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IntValue{IntValue: val.Int64}}
		}
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IsNull{IsNull: true}}
	case sql.NullFloat64:
		if val.Valid {
			return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_DoubleValue{DoubleValue: val.Float64}}
		}
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_IsNull{IsNull: true}}
	default:
		// Fallback: try to convert to string
		return &pluginv1.SQLValue{Value: &pluginv1.SQLValue_StringValue{StringValue: fmt.Sprintf("%v", v)}}
	}
}

// isValidUTF8 checks if bytes are valid UTF-8 text
func isValidUTF8(b []byte) bool {
	// Check for null bytes or control characters that indicate binary
	for _, c := range b {
		if c == 0 || (c < 32 && c != '\n' && c != '\r' && c != '\t') {
			return false
		}
	}
	return true
}

// truncateSQL returns a truncated version of SQL for logging
func truncateSQL(sql string) string {
	const maxLen = 200
	if len(sql) > maxLen {
		return sql[:maxLen] + "..."
	}
	return sql
}
