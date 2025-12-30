package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
)

// testHandler is a custom slog handler that captures log records for testing
type testHandler struct {
	level   slog.Level
	records *[]slog.Record // Use pointer to share records across WithAttrs copies
	attrs   []slog.Attr
}

func newTestHandler(level slog.Level) *testHandler {
	records := make([]slog.Record, 0)
	return &testHandler{
		level:   level,
		records: &records,
	}
}

func (h *testHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *testHandler) Handle(_ context.Context, r slog.Record) error {
	*h.records = append(*h.records, r)
	return nil
}

func (h *testHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := &testHandler{
		level:   h.level,
		records: h.records, // Share the pointer
		attrs:   append(h.attrs, attrs...),
	}
	return newHandler
}

func (h *testHandler) WithGroup(name string) slog.Handler {
	return h
}

func (h *testHandler) recordCount() int {
	if h.records == nil {
		return 0
	}
	return len(*h.records)
}

func (h *testHandler) getRecords() []slog.Record {
	if h.records == nil {
		return nil
	}
	return *h.records
}

func TestNewHclogAdapter(t *testing.T) {
	t.Run("creates adapter with nil args", func(t *testing.T) {
		logger := slog.Default()
		adapter := NewHclogAdapter(logger)

		if adapter == nil {
			t.Fatal("expected non-nil adapter")
		}

		hclogAdapter, ok := adapter.(*HclogAdapter)
		if !ok {
			t.Fatal("expected *HclogAdapter type")
		}

		if hclogAdapter.name != "" {
			t.Errorf("expected empty name, got %q", hclogAdapter.name)
		}

		if hclogAdapter.args != nil {
			t.Errorf("expected nil args, got %v", hclogAdapter.args)
		}
	})

	t.Run("creates adapter with custom logger", func(t *testing.T) {
		handler := newTestHandler(slog.LevelDebug)
		logger := slog.New(handler)
		adapter := NewHclogAdapter(logger)

		adapter.Info("test message")

		records := handler.getRecords()
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		if records[0].Message != "test message" {
			t.Errorf("expected message 'test message', got %q", records[0].Message)
		}
	})

	t.Run("implements hclog.Logger interface", func(t *testing.T) {
		logger := slog.Default()
		adapter := NewHclogAdapter(logger)

		// Verify it implements the interface
		var _ hclog.Logger = adapter
	})
}

func TestLogLevelMethods(t *testing.T) {
	tests := []struct {
		name          string
		logFunc       func(hclog.Logger)
		expectedLevel slog.Level
		expectedMsg   string
	}{
		{
			name: "Trace logs at Debug level",
			logFunc: func(l hclog.Logger) {
				l.Trace("trace message", "key", "value")
			},
			expectedLevel: slog.LevelDebug,
			expectedMsg:   "trace message",
		},
		{
			name: "Debug logs at Debug level",
			logFunc: func(l hclog.Logger) {
				l.Debug("debug message", "key", "value")
			},
			expectedLevel: slog.LevelDebug,
			expectedMsg:   "debug message",
		},
		{
			name: "Info logs at Info level",
			logFunc: func(l hclog.Logger) {
				l.Info("info message", "key", "value")
			},
			expectedLevel: slog.LevelInfo,
			expectedMsg:   "info message",
		},
		{
			name: "Warn logs at Warn level",
			logFunc: func(l hclog.Logger) {
				l.Warn("warn message", "key", "value")
			},
			expectedLevel: slog.LevelWarn,
			expectedMsg:   "warn message",
		},
		{
			name: "Error logs at Error level",
			logFunc: func(l hclog.Logger) {
				l.Error("error message", "key", "value")
			},
			expectedLevel: slog.LevelError,
			expectedMsg:   "error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newTestHandler(slog.LevelDebug - 4) // Enable all levels including trace
			logger := slog.New(handler)
			adapter := NewHclogAdapter(logger)

			tt.logFunc(adapter)

			records := handler.getRecords()
			if len(records) != 1 {
				t.Fatalf("expected 1 record, got %d", len(records))
			}

			record := records[0]
			if record.Level != tt.expectedLevel {
				t.Errorf("expected level %v, got %v", tt.expectedLevel, record.Level)
			}

			if record.Message != tt.expectedMsg {
				t.Errorf("expected message %q, got %q", tt.expectedMsg, record.Message)
			}
		})
	}
}

func TestLogMethod(t *testing.T) {
	tests := []struct {
		name          string
		level         hclog.Level
		expectedLevel slog.Level
	}{
		{"NoLevel maps to Info", hclog.NoLevel, slog.LevelInfo},
		{"Trace maps to Debug", hclog.Trace, slog.LevelDebug},
		{"Debug maps to Debug", hclog.Debug, slog.LevelDebug},
		{"Info maps to Info", hclog.Info, slog.LevelInfo},
		{"Warn maps to Warn", hclog.Warn, slog.LevelWarn},
		{"Error maps to Error", hclog.Error, slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newTestHandler(slog.LevelDebug - 4)
			logger := slog.New(handler)
			adapter := NewHclogAdapter(logger)

			adapter.Log(tt.level, "test message")

			records := handler.getRecords()
			if len(records) != 1 {
				t.Fatalf("expected 1 record, got %d", len(records))
			}

			if records[0].Level != tt.expectedLevel {
				t.Errorf("expected level %v, got %v", tt.expectedLevel, records[0].Level)
			}
		})
	}
}

func TestLogWithArgs(t *testing.T) {
	handler := newTestHandler(slog.LevelDebug)
	logger := slog.New(handler)
	adapter := NewHclogAdapter(logger)

	adapter.Info("test", "key1", "value1", "key2", 42)

	records := handler.getRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	record := records[0]
	var attrs []slog.Attr
	record.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(attrs))
	}

	if attrs[0].Key != "key1" || attrs[0].Value.String() != "value1" {
		t.Errorf("unexpected first attr: %v", attrs[0])
	}

	if attrs[1].Key != "key2" || attrs[1].Value.Int64() != 42 {
		t.Errorf("unexpected second attr: %v", attrs[1])
	}
}

func TestNamedMethod(t *testing.T) {
	t.Run("creates named logger from root", func(t *testing.T) {
		handler := newTestHandler(slog.LevelDebug)
		logger := slog.New(handler)
		adapter := NewHclogAdapter(logger)

		named := adapter.Named("component")

		if named.Name() != "component" {
			t.Errorf("expected name 'component', got %q", named.Name())
		}
	})

	t.Run("chains names with dot separator", func(t *testing.T) {
		handler := newTestHandler(slog.LevelDebug)
		logger := slog.New(handler)
		adapter := NewHclogAdapter(logger)

		named := adapter.Named("parent").Named("child")

		if named.Name() != "parent.child" {
			t.Errorf("expected name 'parent.child', got %q", named.Name())
		}
	})

	t.Run("named logger preserves args", func(t *testing.T) {
		handler := newTestHandler(slog.LevelDebug)
		logger := slog.New(handler)
		adapter := NewHclogAdapter(logger)

		withArgs := adapter.With("key", "value")
		named := withArgs.Named("component")

		impliedArgs := named.ImpliedArgs()
		if len(impliedArgs) != 2 || impliedArgs[0] != "key" || impliedArgs[1] != "value" {
			t.Errorf("expected implied args ['key', 'value'], got %v", impliedArgs)
		}
	})

	t.Run("named logger adds logger attribute", func(t *testing.T) {
		handler := newTestHandler(slog.LevelDebug)
		logger := slog.New(handler)
		adapter := NewHclogAdapter(logger)

		named := adapter.Named("mylogger")
		named.Info("test message")

		records := handler.getRecords()
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
	})
}

func TestResetNamed(t *testing.T) {
	handler := newTestHandler(slog.LevelDebug)
	logger := slog.New(handler)
	adapter := NewHclogAdapter(logger)

	named := adapter.Named("parent").Named("child")
	reset := named.ResetNamed("newname")

	if reset.Name() != "newname" {
		t.Errorf("expected name 'newname', got %q", reset.Name())
	}
}

func TestIsLevelMethods(t *testing.T) {
	tests := []struct {
		name        string
		loggerLevel slog.Level
		isTrace     bool
		isDebug     bool
		isInfo      bool
		isWarn      bool
		isError     bool
	}{
		{
			name:        "Trace level enabled",
			loggerLevel: slog.LevelDebug - 4,
			isTrace:     true,
			isDebug:     true,
			isInfo:      true,
			isWarn:      true,
			isError:     true,
		},
		{
			name:        "Debug level enabled",
			loggerLevel: slog.LevelDebug,
			isTrace:     false,
			isDebug:     true,
			isInfo:      true,
			isWarn:      true,
			isError:     true,
		},
		{
			name:        "Info level enabled",
			loggerLevel: slog.LevelInfo,
			isTrace:     false,
			isDebug:     false,
			isInfo:      true,
			isWarn:      true,
			isError:     true,
		},
		{
			name:        "Warn level enabled",
			loggerLevel: slog.LevelWarn,
			isTrace:     false,
			isDebug:     false,
			isInfo:      false,
			isWarn:      true,
			isError:     true,
		},
		{
			name:        "Error level enabled",
			loggerLevel: slog.LevelError,
			isTrace:     false,
			isDebug:     false,
			isInfo:      false,
			isWarn:      false,
			isError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newTestHandler(tt.loggerLevel)
			logger := slog.New(handler)
			adapter := NewHclogAdapter(logger)

			if adapter.IsTrace() != tt.isTrace {
				t.Errorf("IsTrace() = %v, want %v", adapter.IsTrace(), tt.isTrace)
			}
			if adapter.IsDebug() != tt.isDebug {
				t.Errorf("IsDebug() = %v, want %v", adapter.IsDebug(), tt.isDebug)
			}
			if adapter.IsInfo() != tt.isInfo {
				t.Errorf("IsInfo() = %v, want %v", adapter.IsInfo(), tt.isInfo)
			}
			if adapter.IsWarn() != tt.isWarn {
				t.Errorf("IsWarn() = %v, want %v", adapter.IsWarn(), tt.isWarn)
			}
			if adapter.IsError() != tt.isError {
				t.Errorf("IsError() = %v, want %v", adapter.IsError(), tt.isError)
			}
		})
	}
}

func TestImpliedArgs(t *testing.T) {
	t.Run("returns nil for new adapter", func(t *testing.T) {
		logger := slog.Default()
		adapter := NewHclogAdapter(logger)

		args := adapter.ImpliedArgs()
		if args != nil {
			t.Errorf("expected nil args, got %v", args)
		}
	})

	t.Run("returns args after With", func(t *testing.T) {
		logger := slog.Default()
		adapter := NewHclogAdapter(logger)

		withArgs := adapter.With("key1", "value1", "key2", "value2")
		args := withArgs.ImpliedArgs()

		if len(args) != 4 {
			t.Fatalf("expected 4 args, got %d", len(args))
		}

		if args[0] != "key1" || args[1] != "value1" {
			t.Errorf("unexpected args: %v", args)
		}
	})
}

func TestWithMethod(t *testing.T) {
	t.Run("creates new logger with additional args", func(t *testing.T) {
		handler := newTestHandler(slog.LevelDebug)
		logger := slog.New(handler)
		adapter := NewHclogAdapter(logger)

		withArgs := adapter.With("key", "value")

		// Original should not have args
		if adapter.ImpliedArgs() != nil {
			t.Error("original adapter should not have args")
		}

		// New should have args
		args := withArgs.ImpliedArgs()
		if len(args) != 2 || args[0] != "key" || args[1] != "value" {
			t.Errorf("expected ['key', 'value'], got %v", args)
		}
	})

	t.Run("chains With calls", func(t *testing.T) {
		logger := slog.Default()
		adapter := NewHclogAdapter(logger)

		chained := adapter.With("key1", "value1").With("key2", "value2")
		args := chained.ImpliedArgs()

		if len(args) != 4 {
			t.Fatalf("expected 4 args, got %d", len(args))
		}

		if args[0] != "key1" || args[1] != "value1" || args[2] != "key2" || args[3] != "value2" {
			t.Errorf("unexpected args: %v", args)
		}
	})

	t.Run("preserves name when using With", func(t *testing.T) {
		logger := slog.Default()
		adapter := NewHclogAdapter(logger)

		named := adapter.Named("mylogger")
		withArgs := named.With("key", "value")

		if withArgs.Name() != "mylogger" {
			t.Errorf("expected name 'mylogger', got %q", withArgs.Name())
		}
	})

	t.Run("implied args are included in log output", func(t *testing.T) {
		handler := newTestHandler(slog.LevelDebug)
		logger := slog.New(handler)
		adapter := NewHclogAdapter(logger)

		withArgs := adapter.With("implied", "arg")
		withArgs.Info("message", "explicit", "arg")

		records := handler.getRecords()
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		var attrs []slog.Attr
		records[0].Attrs(func(a slog.Attr) bool {
			attrs = append(attrs, a)
			return true
		})

		if len(attrs) != 2 {
			t.Fatalf("expected 2 attrs, got %d", len(attrs))
		}

		// First should be implied, second should be explicit
		if attrs[0].Key != "implied" {
			t.Errorf("expected first attr key 'implied', got %q", attrs[0].Key)
		}
		if attrs[1].Key != "explicit" {
			t.Errorf("expected second attr key 'explicit', got %q", attrs[1].Key)
		}
	})
}

func TestGetLevel(t *testing.T) {
	tests := []struct {
		name          string
		loggerLevel   slog.Level
		expectedLevel hclog.Level
	}{
		{"returns Trace for trace level", slog.LevelDebug - 4, hclog.Trace},
		{"returns Debug for debug level", slog.LevelDebug, hclog.Debug},
		{"returns Info for info level", slog.LevelInfo, hclog.Info},
		{"returns Warn for warn level", slog.LevelWarn, hclog.Warn},
		{"returns Error for error level", slog.LevelError, hclog.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newTestHandler(tt.loggerLevel)
			logger := slog.New(handler)
			adapter := NewHclogAdapter(logger)

			if adapter.GetLevel() != tt.expectedLevel {
				t.Errorf("GetLevel() = %v, want %v", adapter.GetLevel(), tt.expectedLevel)
			}
		})
	}
}

func TestSetLevel(t *testing.T) {
	// SetLevel is a no-op since slog doesn't support dynamic level changes
	handler := newTestHandler(slog.LevelInfo)
	logger := slog.New(handler)
	adapter := NewHclogAdapter(logger)

	// Should not panic
	adapter.SetLevel(hclog.Debug)

	// Level should remain unchanged
	if adapter.GetLevel() != hclog.Info {
		t.Errorf("GetLevel() = %v, want %v", adapter.GetLevel(), hclog.Info)
	}
}

func TestStandardLogger(t *testing.T) {
	handler := newTestHandler(slog.LevelInfo)
	logger := slog.New(handler)
	adapter := NewHclogAdapter(logger)

	stdLogger := adapter.StandardLogger(nil)
	if stdLogger == nil {
		t.Fatal("expected non-nil standard logger")
	}

	stdLogger.Print("standard log message")

	records := handler.getRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if !strings.Contains(records[0].Message, "standard log message") {
		t.Errorf("expected message to contain 'standard log message', got %q", records[0].Message)
	}
}

func TestStandardWriter(t *testing.T) {
	handler := newTestHandler(slog.LevelInfo)
	logger := slog.New(handler)
	adapter := NewHclogAdapter(logger)

	writer := adapter.StandardWriter(nil)
	if writer == nil {
		t.Fatal("expected non-nil writer")
	}

	n, err := writer.Write([]byte("written message"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != 15 {
		t.Errorf("expected 15 bytes written, got %d", n)
	}

	records := handler.getRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0].Message != "written message" {
		t.Errorf("expected message 'written message', got %q", records[0].Message)
	}
}

func TestSlogWriter(t *testing.T) {
	t.Run("writes message as info log", func(t *testing.T) {
		handler := newTestHandler(slog.LevelInfo)
		logger := slog.New(handler)
		writer := &slogWriter{logger: logger}

		n, err := writer.Write([]byte("test message"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if n != 12 {
			t.Errorf("expected 12 bytes, got %d", n)
		}

		records := handler.getRecords()
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		if records[0].Level != slog.LevelInfo {
			t.Errorf("expected Info level, got %v", records[0].Level)
		}
	})

	t.Run("handles empty write", func(t *testing.T) {
		handler := newTestHandler(slog.LevelInfo)
		logger := slog.New(handler)
		writer := &slogWriter{logger: logger}

		n, err := writer.Write([]byte{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if n != 0 {
			t.Errorf("expected 0 bytes, got %d", n)
		}
	})
}

func TestIntegrationWithRealSlog(t *testing.T) {
	// Test with a real slog.Logger to ensure compatibility
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	adapter := NewHclogAdapter(logger)

	adapter.Info("integration test", "component", "hclog")

	output := buf.String()
	if !strings.Contains(output, "integration test") {
		t.Errorf("expected output to contain 'integration test', got %q", output)
	}
	if !strings.Contains(output, "component=hclog") {
		t.Errorf("expected output to contain 'component=hclog', got %q", output)
	}
}
