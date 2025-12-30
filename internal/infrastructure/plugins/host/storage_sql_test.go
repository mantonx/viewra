package host

import (
	"context"
	"testing"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

func TestContextWithPluginID(t *testing.T) {
	ctx := context.Background()
	pluginID := "test-plugin"

	// Set plugin ID
	ctxWithID := ContextWithPluginID(ctx, pluginID)

	// Retrieve it
	got := GetPluginIDFromContext(ctxWithID)
	if got != pluginID {
		t.Errorf("GetPluginIDFromContext() = %q, want %q", got, pluginID)
	}
}

func TestGetPluginIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	// No plugin ID set
	got := GetPluginIDFromContext(ctx)
	if got != "" {
		t.Errorf("GetPluginIDFromContext() = %q, want empty string", got)
	}
}

func TestGetPluginIDFromContext_WrongType(t *testing.T) {
	// Set a non-string value with the same key type
	ctx := context.WithValue(context.Background(), pluginIDKey, 123)

	// Should return empty string for wrong type
	got := GetPluginIDFromContext(ctx)
	if got != "" {
		t.Errorf("GetPluginIDFromContext() = %q, want empty string for wrong type", got)
	}
}

func TestSanitizePluginID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"semantic-search", "semantic_search"},
		{"my_plugin", "my_plugin"},
		{"plugin123", "plugin123"},
		{"my--plugin", "my_plugin"},
		{"my.plugin.name", "my_plugin_name"},
		{"-leading-dash", "leading_dash"},
		{"trailing-dash-", "trailing_dash"},
		{"UPPERCASE", "UPPERCASE"},
		{"MixedCase123", "MixedCase123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizePluginID(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizePluginID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRewriteTableNames(t *testing.T) {
	pluginID := "test"
	prefix := "plugin_test_"

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "CREATE TABLE",
			input:    "CREATE TABLE items (id INTEGER PRIMARY KEY)",
			expected: "CREATE TABLE " + prefix + "items (id INTEGER PRIMARY KEY)",
		},
		{
			name:     "CREATE TABLE IF NOT EXISTS",
			input:    "CREATE TABLE IF NOT EXISTS items (id INTEGER)",
			expected: "CREATE TABLE IF NOT EXISTS " + prefix + "items (id INTEGER)",
		},
		{
			name:     "DROP TABLE",
			input:    "DROP TABLE items",
			expected: "DROP TABLE " + prefix + "items",
		},
		{
			name:     "DROP TABLE IF EXISTS",
			input:    "DROP TABLE IF EXISTS items",
			expected: "DROP TABLE IF EXISTS " + prefix + "items",
		},
		{
			name:     "CREATE INDEX",
			input:    "CREATE INDEX idx_items_name ON items(name)",
			expected: "CREATE INDEX " + prefix + "idx_items_name ON " + prefix + "items(name)",
		},
		{
			name:     "CREATE UNIQUE INDEX",
			input:    "CREATE UNIQUE INDEX idx_unique ON items(id)",
			expected: "CREATE UNIQUE INDEX " + prefix + "idx_unique ON " + prefix + "items(id)",
		},
		{
			name:     "DROP INDEX",
			input:    "DROP INDEX idx_items_name",
			expected: "DROP INDEX " + prefix + "idx_items_name",
		},
		{
			name:     "SELECT FROM",
			input:    "SELECT * FROM items WHERE id = 1",
			expected: "SELECT * FROM " + prefix + "items WHERE id = 1",
		},
		{
			name:     "SELECT FROM with JOIN",
			input:    "SELECT * FROM items JOIN categories ON items.cat_id = categories.id",
			expected: "SELECT * FROM " + prefix + "items JOIN " + prefix + "categories ON items.cat_id = categories.id",
		},
		{
			name:     "INSERT INTO",
			input:    "INSERT INTO items (name) VALUES ('test')",
			expected: "INSERT INTO " + prefix + "items (name) VALUES ('test')",
		},
		{
			name:     "UPDATE",
			input:    "UPDATE items SET name = 'test' WHERE id = 1",
			expected: "UPDATE " + prefix + "items SET name = 'test' WHERE id = 1",
		},
		{
			name:     "DELETE FROM",
			input:    "DELETE FROM items WHERE id = 1",
			expected: "DELETE FROM " + prefix + "items WHERE id = 1",
		},
		{
			name:     "ALTER TABLE",
			input:    "ALTER TABLE items ADD COLUMN created_at TIMESTAMP",
			expected: "ALTER TABLE " + prefix + "items ADD COLUMN created_at TIMESTAMP",
		},
		{
			name:     "case insensitive",
			input:    "select * from Items where id = 1",
			expected: "select * from " + prefix + "Items where id = 1",
		},
		{
			name:     "multiple tables",
			input:    "SELECT * FROM items, categories WHERE items.cat_id = categories.id",
			expected: "SELECT * FROM " + prefix + "items, categories WHERE items.cat_id = categories.id",
		},
		{
			name:     "LEFT JOIN",
			input:    "SELECT * FROM items LEFT JOIN categories ON items.cat_id = categories.id",
			expected: "SELECT * FROM " + prefix + "items LEFT JOIN " + prefix + "categories ON items.cat_id = categories.id",
		},
		{
			name:     "migration table underscore prefix",
			input:    "CREATE TABLE IF NOT EXISTS _migrations (version INTEGER PRIMARY KEY)",
			expected: "CREATE TABLE IF NOT EXISTS " + prefix + "_migrations (version INTEGER PRIMARY KEY)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rewriteTableNames(tt.input, pluginID)
			if err != nil {
				t.Errorf("rewriteTableNames() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("rewriteTableNames(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateSQL(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		expectErr bool
	}{
		// Valid SQL
		{"SELECT", "SELECT * FROM items", false},
		{"INSERT", "INSERT INTO items (name) VALUES ('test')", false},
		{"UPDATE", "UPDATE items SET name = 'test'", false},
		{"DELETE", "DELETE FROM items WHERE id = 1", false},
		{"CREATE TABLE", "CREATE TABLE items (id INTEGER)", false},
		{"DROP TABLE", "DROP TABLE items", false},
		{"CREATE INDEX", "CREATE INDEX idx ON items(name)", false},

		// Forbidden SQL
		{"ATTACH DATABASE", "ATTACH DATABASE 'other.db' AS other", true},
		{"DETACH DATABASE", "DETACH DATABASE other", true},
		{"PRAGMA", "PRAGMA table_info(items)", true},
		{"pg_ system table", "SELECT * FROM pg_tables", true},
		{"sqlite_ system table", "SELECT * FROM sqlite_master", true},
		{"information_schema", "SELECT * FROM information_schema.tables", true},
		{"VACUUM", "VACUUM", true},
		{"ANALYZE", "ANALYZE items", true},
		{"REINDEX", "REINDEX items", true},
		{"ALTER DATABASE", "ALTER DATABASE mydb SET timezone = 'UTC'", true},
		{"CREATE DATABASE", "CREATE DATABASE mydb", true},
		{"DROP DATABASE", "DROP DATABASE mydb", true},
		{"GRANT", "GRANT SELECT ON items TO user1", true},
		{"REVOKE", "REVOKE SELECT ON items FROM user1", true},

		// Case insensitive
		{"attach lowercase", "attach database 'other.db' as other", true},
		{"PRAGMA uppercase", "PRAGMA TABLE_INFO(items)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSQL(tt.sql)
			if tt.expectErr && err == nil {
				t.Errorf("validateSQL(%q) expected error, got nil", tt.sql)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("validateSQL(%q) unexpected error: %v", tt.sql, err)
			}
		})
	}
}

func TestToSQLValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		isNull   bool
		isString bool
		isInt    bool
		isFloat  bool
		isBytes  bool
	}{
		{"nil", nil, true, false, false, false, false},
		{"string", "hello", false, true, false, false, false},
		{"int64", int64(42), false, false, true, false, false},
		{"int", 42, false, false, true, false, false},
		{"float64", 3.14, false, false, false, true, false},
		{"bool true", true, false, false, true, false, false},   // stored as int 1
		{"bool false", false, false, false, true, false, false}, // stored as int 0
		{"bytes", []byte{0x00, 0x01, 0x02}, false, false, false, false, true},
		{"text bytes", []byte("hello"), false, true, false, false, false}, // valid UTF-8 becomes string
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toSQLValue(tt.input)

			if tt.isNull {
				if _, ok := result.Value.(*pluginv1.SQLValue_IsNull); !ok {
					t.Errorf("expected IsNull, got %T", result.Value)
				}
				return
			}

			if tt.isString {
				if _, ok := result.Value.(*pluginv1.SQLValue_StringValue); !ok {
					t.Errorf("expected StringValue, got %T", result.Value)
				}
			}
			if tt.isInt {
				if _, ok := result.Value.(*pluginv1.SQLValue_IntValue); !ok {
					t.Errorf("expected IntValue, got %T", result.Value)
				}
			}
			if tt.isFloat {
				if _, ok := result.Value.(*pluginv1.SQLValue_DoubleValue); !ok {
					t.Errorf("expected DoubleValue, got %T", result.Value)
				}
			}
			if tt.isBytes {
				if _, ok := result.Value.(*pluginv1.SQLValue_BytesValue); !ok {
					t.Errorf("expected BytesValue, got %T", result.Value)
				}
			}
		})
	}
}

func TestIsValidUTF8(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{"empty", []byte{}, true},
		{"ascii", []byte("hello world"), true},
		{"utf8", []byte("hello world"), true},
		{"newlines", []byte("hello\nworld"), true},
		{"tabs", []byte("hello\tworld"), true},
		{"null byte", []byte("hello\x00world"), false},
		{"control char", []byte("hello\x01world"), false},
		{"binary", []byte{0xFF, 0xFE, 0x00, 0x01}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidUTF8(tt.input)
			if result != tt.expected {
				t.Errorf("isValidUTF8(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"short", "SELECT * FROM items", "SELECT * FROM items"},
		{"exactly 200", string(make([]byte, 200)), string(make([]byte, 200))},
		{"long", string(make([]byte, 250)), string(make([]byte, 200)) + "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateSQL(tt.input)
			if result != tt.expected {
				t.Errorf("truncateSQL() length = %d, want %d", len(result), len(tt.expected))
			}
		})
	}
}
