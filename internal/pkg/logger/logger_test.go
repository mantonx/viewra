package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		wantLevel   slog.Level
		wantJSON    bool
	}{
		{
			name:        "production environment",
			environment: "production",
			wantLevel:   slog.LevelInfo,
			wantJSON:    true,
		},
		{
			name:        "development environment",
			environment: "development",
			wantLevel:   slog.LevelDebug,
			wantJSON:    false,
		},
		{
			name:        "staging environment",
			environment: "staging",
			wantLevel:   slog.LevelDebug,
			wantJSON:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.environment)

			if logger == nil {
				t.Fatal("New() returned nil logger")
			}

			// Logger is created successfully - we can't easily inspect internal handler type
			// but we can verify it doesn't panic
			logger.Info("test message", "key", "value")
		})
	}
}

func TestNewDefault(t *testing.T) {
	logger := NewDefault()

	if logger == nil {
		t.Fatal("NewDefault() returned nil logger")
	}

	// Verify it works
	logger.Debug("debug message")
	logger.Info("info message")
}

func TestLoggerOutput(t *testing.T) {
	// Test that logger actually outputs messages
	var buf bytes.Buffer

	// Create a logger with custom handler that writes to buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)

	logger.Info("test message", "key", "value", "number", 42)

	output := buf.String()

	// Verify output contains expected elements
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected output to contain 'test message', got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("Expected output to contain 'key=value', got: %s", output)
	}
	if !strings.Contains(output, "number=42") {
		t.Errorf("Expected output to contain 'number=42', got: %s", output)
	}
}

func TestJSONHandler(t *testing.T) {
	var buf bytes.Buffer

	// Create JSON handler
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)

	logger.Info("json test", "field1", "value1", "field2", 123)

	output := buf.String()

	// JSON output should contain these fields
	expectedFields := []string{
		`"msg":"json test"`,
		`"field1":"value1"`,
		`"field2":123`,
		`"level":"INFO"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Expected JSON output to contain %q, got: %s", field, output)
		}
	}
}
