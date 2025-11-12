package database

import (
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid SQLite config",
			config: &Config{
				Driver: "sqlite",
				DBName: "test.db",
			},
			wantErr: false,
		},
		{
			name: "valid SQLite3 config",
			config: &Config{
				Driver: "sqlite3",
				DBName: "data/viewra.db",
			},
			wantErr: false,
		},
		{
			name: "valid PostgreSQL config",
			config: &Config{
				Driver:   "postgres",
				Host:     "localhost",
				Port:     "5432",
				User:     "viewra",
				Password: "secret",
				DBName:   "viewra",
				SSLMode:  "disable",
			},
			wantErr: false,
		},
		{
			name: "valid PostgreSQL config with postgresql driver",
			config: &Config{
				Driver:  "postgresql",
				Host:    "localhost",
				Port:    "5432",
				User:    "viewra",
				DBName:  "viewra",
				SSLMode: "require",
			},
			wantErr: false,
		},
		{
			name: "SQLite missing DB path",
			config: &Config{
				Driver: "sqlite",
				DBName: "",
			},
			wantErr: true,
			errMsg:  "DB_PATH cannot be empty",
		},
		{
			name: "PostgreSQL missing host",
			config: &Config{
				Driver:  "postgres",
				Host:    "",
				Port:    "5432",
				User:    "viewra",
				DBName:  "viewra",
				SSLMode: "disable",
			},
			wantErr: true,
			errMsg:  "DB_HOST cannot be empty",
		},
		{
			name: "PostgreSQL missing port",
			config: &Config{
				Driver:  "postgres",
				Host:    "localhost",
				Port:    "",
				User:    "viewra",
				DBName:  "viewra",
				SSLMode: "disable",
			},
			wantErr: true,
			errMsg:  "DB_PORT cannot be empty",
		},
		{
			name: "PostgreSQL missing user",
			config: &Config{
				Driver:  "postgres",
				Host:    "localhost",
				Port:    "5432",
				User:    "",
				DBName:  "viewra",
				SSLMode: "disable",
			},
			wantErr: true,
			errMsg:  "DB_USER cannot be empty",
		},
		{
			name: "PostgreSQL missing database name",
			config: &Config{
				Driver:  "postgres",
				Host:    "localhost",
				Port:    "5432",
				User:    "viewra",
				DBName:  "",
				SSLMode: "disable",
			},
			wantErr: true,
			errMsg:  "DB_NAME cannot be empty",
		},
		{
			name: "PostgreSQL invalid SSL mode",
			config: &Config{
				Driver:  "postgres",
				Host:    "localhost",
				Port:    "5432",
				User:    "viewra",
				DBName:  "viewra",
				SSLMode: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid DB_SSL_MODE",
		},
		{
			name: "unsupported driver",
			config: &Config{
				Driver: "mysql",
				DBName: "test",
			},
			wantErr: true,
			errMsg:  "unsupported database driver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Simple test - just verify it returns a config without panicking
	// Actual env var testing would require os.Setenv which affects global state
	config := LoadConfigFromEnv()

	if config == nil {
		t.Fatal("LoadConfigFromEnv() returned nil")
	}

	// Should have a driver (either from env or default)
	if config.Driver == "" {
		t.Error("LoadConfigFromEnv() driver is empty")
	}

	// Should have a DBName (either from env or default)
	if config.DBName == "" {
		t.Error("LoadConfigFromEnv() DBName is empty")
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func getEnv(key string) string {
	// This would use os.Getenv in real code
	return ""
}

func setEnv(key, value string) {
	// This would use os.Setenv in real code
	// For now, tests are simplified
}
