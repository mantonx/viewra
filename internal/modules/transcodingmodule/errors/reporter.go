// Package errors provides error reporting mechanisms for background operations
package errors

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

// ErrorReporter provides a mechanism for background goroutines to report errors
type ErrorReporter interface {
	// ReportError reports a non-fatal error from a background operation
	ReportError(ctx context.Context, err error)
	
	// ReportPanic reports a panic from a background operation
	ReportPanic(ctx context.Context, recovered interface{}, stack []byte)
	
	// GetErrors returns all reported errors since last clear
	GetErrors() []ReportedError
	
	// ClearErrors clears the error history
	ClearErrors()
}

// ReportedError contains error information from background operations
type ReportedError struct {
	Error     error
	Operation string
	SessionID string
	IsPanic   bool
	Stack     string
	Timestamp int64
}

// DefaultErrorReporter implements ErrorReporter with logging and collection
type DefaultErrorReporter struct {
	logger    hclog.Logger
	errors    []ReportedError
	errorsMux sync.RWMutex
	maxErrors int
}

// NewErrorReporter creates a new error reporter
func NewErrorReporter(logger hclog.Logger) ErrorReporter {
	return &DefaultErrorReporter{
		logger:    logger,
		errors:    make([]ReportedError, 0),
		maxErrors: 1000, // Prevent unbounded growth
	}
}

// ReportError reports a non-fatal error from a background operation
func (r *DefaultErrorReporter) ReportError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	
	// Extract structured error information
	errType := GetType(err)
	op := GetOperation(err)
	sessionID := GetSessionID(err)
	details := GetDetails(err)
	
	// Log with appropriate level
	if tErr, ok := err.(*TranscodingError); ok && tErr.IsRecoverable() {
		r.logger.Warn("background operation error",
			"error", err,
			"type", errType,
			"operation", op,
			"session_id", sessionID,
			"details", details,
			"recoverable", true,
		)
	} else {
		r.logger.Error("background operation error",
			"error", err,
			"type", errType,
			"operation", op,
			"session_id", sessionID,
			"details", details,
			"recoverable", false,
		)
	}
	
	// Store error for retrieval
	r.errorsMux.Lock()
	defer r.errorsMux.Unlock()
	
	// Prevent unbounded growth
	if len(r.errors) >= r.maxErrors {
		r.errors = r.errors[1:] // Remove oldest
	}
	
	r.errors = append(r.errors, ReportedError{
		Error:     err,
		Operation: op,
		SessionID: sessionID,
		IsPanic:   false,
		Timestamp: timeNow().Unix(),
	})
}

// ReportPanic reports a panic from a background operation
func (r *DefaultErrorReporter) ReportPanic(ctx context.Context, recovered interface{}, stack []byte) {
	// Log panic with full stack trace
	r.logger.Error("panic in background operation",
		"panic", recovered,
		"stack", string(stack),
	)
	
	// Convert panic to error
	var err error
	switch v := recovered.(type) {
	case error:
		err = v
	case string:
		err = fmt.Errorf("panic: %s", v)
	default:
		err = fmt.Errorf("panic: %v", v)
	}
	
	// Store panic error
	r.errorsMux.Lock()
	defer r.errorsMux.Unlock()
	
	if len(r.errors) >= r.maxErrors {
		r.errors = r.errors[1:]
	}
	
	r.errors = append(r.errors, ReportedError{
		Error:     err,
		Operation: "panic",
		IsPanic:   true,
		Stack:     string(stack),
		Timestamp: timeNow().Unix(),
	})
}

// GetErrors returns all reported errors since last clear
func (r *DefaultErrorReporter) GetErrors() []ReportedError {
	r.errorsMux.RLock()
	defer r.errorsMux.RUnlock()
	
	// Return a copy to prevent data races
	result := make([]ReportedError, len(r.errors))
	copy(result, r.errors)
	return result
}

// ClearErrors clears the error history
func (r *DefaultErrorReporter) ClearErrors() {
	r.errorsMux.Lock()
	defer r.errorsMux.Unlock()
	
	r.errors = r.errors[:0]
}

// SafeGo runs a function in a goroutine with panic recovery and error reporting
func SafeGo(reporter ErrorReporter, logger hclog.Logger, name string, fn func() error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				reporter.ReportPanic(context.Background(), r, stack)
			}
		}()
		
		logger.Debug("starting background operation", "name", name)
		
		if err := fn(); err != nil {
			// Add operation context to error
			err = InternalError(name, err).
				WithDetail("goroutine", name)
			reporter.ReportError(context.Background(), err)
		}
		
		logger.Debug("completed background operation", "name", name)
	}()
}

// SafeGoContext runs a function in a goroutine with context, panic recovery, and error reporting
func SafeGoContext(ctx context.Context, reporter ErrorReporter, logger hclog.Logger, name string, fn func(context.Context) error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				reporter.ReportPanic(ctx, r, stack)
			}
		}()
		
		logger.Debug("starting background operation", "name", name)
		
		if err := fn(ctx); err != nil {
			// Don't report context cancellation as an error
			if err == context.Canceled {
				logger.Debug("background operation cancelled", "name", name)
				return
			}
			
			// Add operation context to error
			err = InternalError(name, err).
				WithDetail("goroutine", name)
			reporter.ReportError(ctx, err)
		}
		
		logger.Debug("completed background operation", "name", name)
	}()
}

// Helper for testing
var timeNow = func() timeInterface {
	return realTime{}
}

type timeInterface interface {
	Unix() int64
}

type realTime struct{}

func (realTime) Unix() int64 {
	return time.Now().Unix()
}

