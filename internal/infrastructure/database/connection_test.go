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
	// Save original env vars and restore after test
	originalDriver := getEnvOrDefault("DB_DRIVER", "")
	t.Cleanup(func() {
		if originalDriver != "" {
			t.Setenv("DB_DRIVER", originalDriver)
		}
	})

	// Test 1: Default SQLite configuration
	t.Run("default SQLite config", func(t *testing.T) {
		config := LoadConfigFromEnv()
		if config == nil {
			t.Fatal("LoadConfigFromEnv() returned nil")
		}
		if config.Driver != "sqlite" {
			t.Errorf("Expected default driver 'sqlite', got %s", config.Driver)
		}
		if config.DBName == "" {
			t.Error("LoadConfigFromEnv() DBName is empty")
		}
	})

	// Test 2: PostgreSQL configuration
	t.Run("postgres config from env", func(t *testing.T) {
		t.Setenv("DB_DRIVER", "postgres")
		t.Setenv("DB_HOST", "testhost")
		t.Setenv("DB_PORT", "5433")
		t.Setenv("DB_USER", "testuser")
		t.Setenv("DB_PASSWORD", "testpass")
		t.Setenv("DB_NAME", "testdb")
		t.Setenv("DB_SSL_MODE", "require")

		config := LoadConfigFromEnv()
		if config.Driver != "postgres" {
			t.Errorf("Expected driver 'postgres', got %s", config.Driver)
		}
		if config.Host != "testhost" {
			t.Errorf("Expected host 'testhost', got %s", config.Host)
		}
		if config.Port != "5433" {
			t.Errorf("Expected port '5433', got %s", config.Port)
		}
		if config.User != "testuser" {
			t.Errorf("Expected user 'testuser', got %s", config.User)
		}
		if config.Password != "testpass" {
			t.Errorf("Expected password 'testpass', got %s", config.Password)
		}
		if config.DBName != "testdb" {
			t.Errorf("Expected dbname 'testdb', got %s", config.DBName)
		}
		if config.SSLMode != "require" {
			t.Errorf("Expected sslmode 'require', got %s", config.SSLMode)
		}
	})
}

func TestConnect_SQLite(t *testing.T) {
	// Test SQLite connection with in-memory database
	config := &Config{
		Driver: "sqlite",
		DBName: ":memory:",
	}

	db, err := Connect(config)
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer db.Close()

	// Verify connection works
	if err := db.Ping(); err != nil {
		t.Errorf("Ping() failed: %v", err)
	}
}

func TestConnect_InvalidDriver(t *testing.T) {
	config := &Config{
		Driver: "mysql",
		DBName: "test.db",
	}

	_, err := Connect(config)
	if err == nil {
		t.Error("Connect() expected error for invalid driver, got nil")
	}
	if !contains(err.Error(), "unsupported database driver") {
		t.Errorf("Connect() error = %v, want error containing 'unsupported database driver'", err)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{
			name:         "env var set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			want:         "custom",
		},
		{
			name:         "env var not set",
			key:          "NONEXISTENT_VAR",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			}
			got := getEnvOrDefault(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvOrDefault() = %v, want %v", got, tt.want)
			}
		})
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
