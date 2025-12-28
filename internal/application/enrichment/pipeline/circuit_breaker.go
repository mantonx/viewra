package pipeline

import (
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState string

const (
	// CircuitClosed means the circuit is healthy and requests flow normally.
	CircuitClosed CircuitState = "closed"
	// CircuitOpen means the circuit has tripped due to failures and requests are blocked.
	CircuitOpen CircuitState = "open"
	// CircuitHalfOpen means the circuit is testing if the service has recovered.
	CircuitHalfOpen CircuitState = "half_open"
)

// CircuitBreaker implements the circuit breaker pattern for enrichment stages.
// It tracks consecutive failures and opens the circuit when a threshold is reached,
// preventing wasted resources on a failing external service.
type CircuitBreaker struct {
	mu sync.RWMutex

	// Configuration
	stage            string
	failureThreshold int           // Consecutive failures before opening
	resetTimeout     time.Duration // Time to wait before trying half-open
	maxResetTimeout  time.Duration // Maximum reset timeout after exponential backoff

	// State
	state             CircuitState
	consecutiveFailures int
	lastFailureTime   time.Time
	lastStateChange   time.Time
	currentResetTimeout time.Duration

	// Callback for state changes (for SSE events)
	onStateChange func(stage string, oldState, newState CircuitState)
}

// CircuitBreakerConfig configures a circuit breaker.
type CircuitBreakerConfig struct {
	Stage            string
	FailureThreshold int           // Default: 10
	ResetTimeout     time.Duration // Default: 5 minutes
	MaxResetTimeout  time.Duration // Default: 1 hour
}

// NewCircuitBreaker creates a new circuit breaker for a stage.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 10
	}
	if cfg.ResetTimeout <= 0 {
		cfg.ResetTimeout = 5 * time.Minute
	}
	if cfg.MaxResetTimeout <= 0 {
		cfg.MaxResetTimeout = 1 * time.Hour
	}

	return &CircuitBreaker{
		stage:              cfg.Stage,
		failureThreshold:   cfg.FailureThreshold,
		resetTimeout:       cfg.ResetTimeout,
		maxResetTimeout:    cfg.MaxResetTimeout,
		state:              CircuitClosed,
		currentResetTimeout: cfg.ResetTimeout,
		lastStateChange:    time.Now(),
	}
}

// SetOnStateChange sets a callback for state changes.
func (cb *CircuitBreaker) SetOnStateChange(fn func(stage string, oldState, newState CircuitState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// Allow checks if a request should be allowed through.
// Returns true if the circuit is closed or half-open (for probe requests).
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true

	case CircuitOpen:
		// Check if reset timeout has elapsed
		if time.Since(cb.lastStateChange) >= cb.currentResetTimeout {
			cb.transitionTo(CircuitHalfOpen)
			return true
		}
		return false

	case CircuitHalfOpen:
		// Allow one probe request
		return true

	default:
		return true
	}
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures = 0

	if cb.state == CircuitHalfOpen {
		// Success in half-open state closes the circuit
		cb.currentResetTimeout = cb.resetTimeout // Reset backoff
		cb.transitionTo(CircuitClosed)
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		if cb.consecutiveFailures >= cb.failureThreshold {
			cb.transitionTo(CircuitOpen)
		}

	case CircuitHalfOpen:
		// Failure in half-open state reopens the circuit with exponential backoff
		cb.currentResetTimeout = min(cb.currentResetTimeout*2, cb.maxResetTimeout)
		cb.transitionTo(CircuitOpen)
	}
}

// transitionTo changes the circuit state (must be called with lock held).
func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()

	if cb.onStateChange != nil {
		// Call async to avoid blocking
		go cb.onStateChange(cb.stage, oldState, newState)
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Status returns detailed circuit breaker status.
func (cb *CircuitBreaker) Status() CircuitBreakerStatus {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	status := CircuitBreakerStatus{
		Stage:               cb.stage,
		State:               cb.state,
		ConsecutiveFailures: cb.consecutiveFailures,
		FailureThreshold:    cb.failureThreshold,
		LastStateChange:     cb.lastStateChange,
	}

	if cb.state == CircuitOpen {
		remaining := cb.currentResetTimeout - time.Since(cb.lastStateChange)
		if remaining > 0 {
			status.RetryAfter = remaining
			status.RetryAt = time.Now().Add(remaining)
		}
	}

	if !cb.lastFailureTime.IsZero() {
		status.LastFailure = cb.lastFailureTime
	}

	return status
}

// Reset manually resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures = 0
	cb.currentResetTimeout = cb.resetTimeout
	if cb.state != CircuitClosed {
		cb.transitionTo(CircuitClosed)
	}
}

// CircuitBreakerStatus contains the current status of a circuit breaker.
type CircuitBreakerStatus struct {
	Stage               string        `json:"stage"`
	State               CircuitState  `json:"state"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
	FailureThreshold    int           `json:"failure_threshold"`
	LastStateChange     time.Time     `json:"last_state_change"`
	LastFailure         time.Time     `json:"last_failure,omitempty"`
	RetryAfter          time.Duration `json:"retry_after_seconds,omitempty"`
	RetryAt             time.Time     `json:"retry_at,omitempty"`
}

// CircuitBreakerRegistry manages circuit breakers for all stages.
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   CircuitBreakerConfig // Default config for new breakers
}

// NewCircuitBreakerRegistry creates a new registry.
func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		config: CircuitBreakerConfig{
			FailureThreshold: 10,
			ResetTimeout:     5 * time.Minute,
			MaxResetTimeout:  1 * time.Hour,
		},
	}
}

// Get returns the circuit breaker for a stage, creating one if needed.
func (r *CircuitBreakerRegistry) Get(stage string) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[stage]
	r.mu.RUnlock()

	if ok {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, ok := r.breakers[stage]; ok {
		return cb
	}

	cfg := r.config
	cfg.Stage = stage
	cb = NewCircuitBreaker(cfg)
	r.breakers[stage] = cb
	return cb
}

// GetAll returns status for all circuit breakers.
func (r *CircuitBreakerRegistry) GetAll() []CircuitBreakerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statuses := make([]CircuitBreakerStatus, 0, len(r.breakers))
	for _, cb := range r.breakers {
		statuses = append(statuses, cb.Status())
	}
	return statuses
}

// SetOnStateChange sets a callback for all circuit breakers.
func (r *CircuitBreakerRegistry) SetOnStateChange(fn func(stage string, oldState, newState CircuitState)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, cb := range r.breakers {
		cb.SetOnStateChange(fn)
	}

	// Store for future breakers
	r.config.Stage = "" // Will be set per-breaker
}
