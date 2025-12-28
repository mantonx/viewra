package pipeline

import (
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Stage:            "test",
		FailureThreshold: 5,
		ResetTimeout:     1 * time.Second,
	})

	if cb.State() != CircuitClosed {
		t.Errorf("expected initial state to be closed, got %s", cb.State())
	}

	if !cb.Allow() {
		t.Error("expected Allow() to return true when circuit is closed")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Stage:            "test",
		FailureThreshold: 3,
		ResetTimeout:     1 * time.Second,
	})

	// Record failures up to threshold
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitClosed {
		t.Error("circuit should still be closed before threshold")
	}

	// This should trip the circuit
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("expected circuit to be open after %d failures, got %s", 3, cb.State())
	}

	if cb.Allow() {
		t.Error("expected Allow() to return false when circuit is open")
	}
}

func TestCircuitBreaker_SuccessResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Stage:            "test",
		FailureThreshold: 3,
		ResetTimeout:     1 * time.Second,
	})

	// Record some failures
	cb.RecordFailure()
	cb.RecordFailure()

	// Success should reset the counter
	cb.RecordSuccess()

	// Now we should need 3 more failures to trip
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitClosed {
		t.Error("circuit should still be closed")
	}

	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Error("circuit should be open after 3 failures")
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Stage:            "test",
		FailureThreshold: 1,
		ResetTimeout:     50 * time.Millisecond,
	})

	// Trip the circuit
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatal("circuit should be open")
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Allow should transition to half-open
	if !cb.Allow() {
		t.Error("expected Allow() to return true after reset timeout")
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected state to be half-open, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenSuccessCloses(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Stage:            "test",
		FailureThreshold: 1,
		ResetTimeout:     50 * time.Millisecond,
	})

	// Trip the circuit
	cb.RecordFailure()

	// Wait for reset timeout and transition to half-open
	time.Sleep(60 * time.Millisecond)
	cb.Allow()

	// Success in half-open should close the circuit
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Errorf("expected circuit to be closed after success in half-open, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Stage:            "test",
		FailureThreshold: 1,
		ResetTimeout:     50 * time.Millisecond,
		MaxResetTimeout:  500 * time.Millisecond,
	})

	// Trip the circuit
	cb.RecordFailure()

	// Wait for reset timeout and transition to half-open
	time.Sleep(60 * time.Millisecond)
	cb.Allow()

	// Failure in half-open should reopen with backoff
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("expected circuit to be open after failure in half-open, got %s", cb.State())
	}

	// Check that reset timeout increased (backoff)
	status := cb.Status()
	if status.RetryAfter < 90*time.Millisecond {
		t.Errorf("expected backoff to increase reset timeout, got %v", status.RetryAfter)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Stage:            "test",
		FailureThreshold: 1,
		ResetTimeout:     1 * time.Hour, // Long timeout
	})

	// Trip the circuit
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatal("circuit should be open")
	}

	// Manual reset
	cb.Reset()
	if cb.State() != CircuitClosed {
		t.Errorf("expected circuit to be closed after reset, got %s", cb.State())
	}
	if !cb.Allow() {
		t.Error("expected Allow() to return true after reset")
	}
}

func TestCircuitBreaker_Status(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Stage:            "test-stage",
		FailureThreshold: 5,
		ResetTimeout:     1 * time.Minute,
	})

	status := cb.Status()
	if status.Stage != "test-stage" {
		t.Errorf("expected stage 'test-stage', got %s", status.Stage)
	}
	if status.State != CircuitClosed {
		t.Errorf("expected state closed, got %s", status.State)
	}
	if status.FailureThreshold != 5 {
		t.Errorf("expected threshold 5, got %d", status.FailureThreshold)
	}
	if status.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 failures, got %d", status.ConsecutiveFailures)
	}
}

func TestCircuitBreaker_StateChangeCallback(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Stage:            "test",
		FailureThreshold: 1,
		ResetTimeout:     50 * time.Millisecond,
	})

	var mu sync.Mutex
	var transitions []struct {
		stage    string
		oldState CircuitState
		newState CircuitState
	}

	cb.SetOnStateChange(func(stage string, oldState, newState CircuitState) {
		mu.Lock()
		defer mu.Unlock()
		transitions = append(transitions, struct {
			stage    string
			oldState CircuitState
			newState CircuitState
		}{stage, oldState, newState})
	})

	// Trip the circuit
	cb.RecordFailure()

	// Wait for callback
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if len(transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(transitions))
	}
	if transitions[0].oldState != CircuitClosed || transitions[0].newState != CircuitOpen {
		t.Errorf("expected closed->open, got %s->%s", transitions[0].oldState, transitions[0].newState)
	}
	mu.Unlock()
}

func TestCircuitBreakerRegistry(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	// Get creates new breaker
	cb1 := registry.Get("stage1")
	if cb1 == nil {
		t.Fatal("expected non-nil circuit breaker")
	}

	// Get returns same breaker
	cb2 := registry.Get("stage1")
	if cb1 != cb2 {
		t.Error("expected same circuit breaker instance")
	}

	// Different stage gets different breaker
	cb3 := registry.Get("stage2")
	if cb1 == cb3 {
		t.Error("expected different circuit breaker for different stage")
	}

	// GetAll returns all breakers
	statuses := registry.GetAll()
	if len(statuses) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(statuses))
	}
}
