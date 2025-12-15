package recovery

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// mockScanJobCompleter implements ScanJobCompleter for testing
type mockScanJobCompleter struct {
	completeFunc func(ctx context.Context, job *scanner.ScanJob) error
	completedJob *scanner.ScanJob
}

func (m *mockScanJobCompleter) Complete(ctx context.Context, job *scanner.ScanJob) error {
	m.completedJob = job
	if m.completeFunc != nil {
		return m.completeFunc(ctx, job)
	}
	return nil
}

func TestLogPanic(t *testing.T) {
	// LogPanic should not panic when called with valid inputs
	logger := testLogger()

	// Test with string panic value
	LogPanic(logger, "test panic", "test description")

	// Test with error panic value
	LogPanic(logger, errors.New("test error"), "test description with error")

	// Test with additional fields
	LogPanic(logger, "panic value", "description", "key1", "value1", "key2", 42)
}

func TestRecoverFromPanic_NoPanic(t *testing.T) {
	logger := testLogger()
	completer := &mockScanJobCompleter{}

	// When no panic occurs, RecoverFromPanic should do nothing
	func() {
		defer RecoverFromPanic(logger, completer, 1, 1, "test")
		// No panic here
	}()

	if completer.completedJob != nil {
		t.Error("Expected no job completion when no panic occurs")
	}
}

func TestRecoverFromPanic_WithPanic(t *testing.T) {
	logger := testLogger()
	completer := &mockScanJobCompleter{}

	// When panic occurs, RecoverFromPanic should mark job as failed
	func() {
		defer RecoverFromPanic(logger, completer, 123, 456, "test panic context")
		panic("intentional panic for testing")
	}()

	if completer.completedJob == nil {
		t.Fatal("Expected job to be completed on panic")
	}

	if completer.completedJob.ID != 123 {
		t.Errorf("Job ID = %d, want 123", completer.completedJob.ID)
	}

	if completer.completedJob.Status != scanner.ScanStatusFailed {
		t.Errorf("Job status = %v, want %v", completer.completedJob.Status, scanner.ScanStatusFailed)
	}

	if completer.completedJob.ErrorMessage == "" {
		t.Error("Expected error message to be set")
	}

	if completer.completedJob.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}
}

func TestRecoverFromPanic_CompleteError(t *testing.T) {
	logger := testLogger()
	completer := &mockScanJobCompleter{
		completeFunc: func(ctx context.Context, job *scanner.ScanJob) error {
			return errors.New("complete failed")
		},
	}

	// Should not panic even when Complete fails
	func() {
		defer RecoverFromPanic(logger, completer, 1, 1, "test")
		panic("test panic")
	}()

	// Test passes if no secondary panic occurs
}

func TestRecoverFromPanicWithError_NoPanic(t *testing.T) {
	logger := testLogger()
	errChan := make(chan error, 1)

	func() {
		defer RecoverFromPanicWithError(logger, 1, 1, "test", errChan)
		// No panic
	}()

	select {
	case err := <-errChan:
		t.Errorf("Expected no error, got: %v", err)
	default:
		// Expected: no error sent
	}
}

func TestRecoverFromPanicWithError_WithPanic(t *testing.T) {
	logger := testLogger()
	errChan := make(chan error, 1)

	func() {
		defer RecoverFromPanicWithError(logger, 123, 456, "test context", errChan)
		panic("intentional panic")
	}()

	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected error to be sent")
		}
	default:
		t.Error("Expected error in channel")
	}
}

func TestRecoverFromPanicWithError_ChannelFull(t *testing.T) {
	logger := testLogger()
	// Unbuffered channel that's already blocked
	errChan := make(chan error)

	// Should not block even when channel is full/unbuffered
	done := make(chan bool)
	go func() {
		func() {
			defer RecoverFromPanicWithError(logger, 1, 1, "test", errChan)
			panic("test panic")
		}()
		done <- true
	}()

	<-done // Test passes if this doesn't hang
}

func TestRecoverWorkerPanic_NoPanic(t *testing.T) {
	logger := testLogger()

	// Should not panic when no panic occurs
	func() {
		defer RecoverWorkerPanic(logger, 1, 0)
		// No panic
	}()
}

func TestRecoverWorkerPanic_WithPanic(t *testing.T) {
	logger := testLogger()

	// Should recover from panic without re-panicking
	func() {
		defer RecoverWorkerPanic(logger, 123, 5)
		panic("worker panic")
	}()

	// Test passes if no panic propagates
}
