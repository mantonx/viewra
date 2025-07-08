package errors

import (
	"errors"
	"testing"
)

func TestTranscodingError(t *testing.T) {
	// Test basic error creation
	err := New(ErrorTypeSession, "create_session", errors.New("database error"))
	if err.Type != ErrorTypeSession {
		t.Errorf("expected type %s, got %s", ErrorTypeSession, err.Type)
	}
	if err.Op != "create_session" {
		t.Errorf("expected op 'create_session', got %s", err.Op)
	}

	// Test error with session
	err = err.WithSession("test-session-123")
	if err.SessionID != "test-session-123" {
		t.Errorf("expected session ID 'test-session-123', got %s", err.SessionID)
	}

	// Test error with details
	err = err.WithDetail("provider", "ffmpeg").WithDetail("format", "mp4")
	if err.Details["provider"] != "ffmpeg" {
		t.Errorf("expected provider 'ffmpeg', got %v", err.Details["provider"])
	}

	// Test error string
	errStr := err.Error()
	expectedStr := "session error in create_session for session test-session-123: database error"
	if errStr != expectedStr {
		t.Errorf("expected error string '%s', got '%s'", expectedStr, errStr)
	}
}

func TestErrorWrapping(t *testing.T) {
	// Test wrapping with sentinel errors
	err := SessionError("get_session", ErrSessionNotFound)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Error("expected error to match ErrSessionNotFound")
	}

	// Test error type extraction
	if GetType(err) != ErrorTypeSession {
		t.Errorf("expected type %s, got %s", ErrorTypeSession, GetType(err))
	}

	// Test operation extraction
	if GetOperation(err) != "get_session" {
		t.Errorf("expected operation 'get_session', got %s", GetOperation(err))
	}
}

func TestIsRecoverable(t *testing.T) {
	tests := []struct {
		name         string
		err          *TranscodingError
		recoverable  bool
	}{
		{
			name:        "timeout error",
			err:         ResourceError("transcode", ErrTimeout),
			recoverable: true,
		},
		{
			name:        "resource limit error",
			err:         ResourceError("start_session", ErrResourceLimitExceeded),
			recoverable: true,
		},
		{
			name:        "provider not available",
			err:         ProviderError("select_provider", ErrProviderNotAvailable),
			recoverable: true,
		},
		{
			name:        "validation error",
			err:         ValidationError("validate_input", ErrInvalidInput),
			recoverable: false,
		},
		{
			name:        "content not found",
			err:         StorageError("get_content", ErrContentNotFound),
			recoverable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.IsRecoverable() != tt.recoverable {
				t.Errorf("expected IsRecoverable() = %v, got %v", tt.recoverable, tt.err.IsRecoverable())
			}
		})
	}
}

func TestWrap(t *testing.T) {
	// Test wrapping nil error
	wrapped := Wrap(nil, ErrorTypeSession, "test_op")
	if wrapped != nil {
		t.Error("expected nil when wrapping nil error")
	}

	// Test wrapping regular error
	err := errors.New("test error")
	wrapped = Wrap(err, ErrorTypeStorage, "store_file")
	tErr, ok := wrapped.(*TranscodingError)
	if !ok {
		t.Fatal("expected TranscodingError type")
	}
	if tErr.Type != ErrorTypeStorage {
		t.Errorf("expected type %s, got %s", ErrorTypeStorage, tErr.Type)
	}

	// Test wrapping already wrapped error (should preserve)
	rewrapped := Wrap(wrapped, ErrorTypeInternal, "different_op")
	if rewrapped != wrapped {
		t.Error("expected wrapped error to be preserved")
	}
}