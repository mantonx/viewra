package playbackmodule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	plugins "github.com/mantonx/viewra/sdk"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// CircuitBreaker implements circuit breaker pattern for provider failures
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            CircuitBreakerState
	failures         int
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	lastFailureTime  time.Time
	nextRetryTime    time.Time
	logger           hclog.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration, logger hclog.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitBreakerClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
		logger:           logger,
	}
}

// Allow checks if a request can proceed
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		return time.Now().After(cb.nextRetryTime)
	case CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	if cb.state == CircuitBreakerHalfOpen {
		cb.logger.Info("Circuit breaker transitioning to closed state")
		cb.state = CircuitBreakerClosed
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	if cb.state == CircuitBreakerClosed && cb.failures >= cb.failureThreshold {
		cb.logger.Warn("Circuit breaker opening due to failures", "failures", cb.failures)
		cb.state = CircuitBreakerOpen
		cb.nextRetryTime = time.Now().Add(cb.timeout)
	} else if cb.state == CircuitBreakerHalfOpen {
		cb.logger.Warn("Circuit breaker returning to open state")
		cb.state = CircuitBreakerOpen
		cb.nextRetryTime = time.Now().Add(cb.timeout)
	}
}

// GetState returns the current state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	// Check if we can transition from open to half-open
	if cb.state == CircuitBreakerOpen && time.Now().After(cb.nextRetryTime) {
		cb.mu.RUnlock()
		cb.mu.Lock()
		defer cb.mu.Unlock()

		// Double-check condition after acquiring write lock
		if cb.state == CircuitBreakerOpen && time.Now().After(cb.nextRetryTime) {
			cb.logger.Info("Circuit breaker transitioning to half-open state")
			cb.state = CircuitBreakerHalfOpen
		}
		return cb.state
	}

	return cb.state
}

// ErrorRecoveryManager manages progressive fallback and circuit breaker patterns
type ErrorRecoveryManager struct {
	logger          hclog.Logger
	circuitBreakers map[string]*CircuitBreaker
	mu              sync.RWMutex

	// Progressive fallback configuration
	fallbackStrategies []FallbackStrategy
	maxRetries         int
	baseRetryDelay     time.Duration
}

// FallbackStrategy defines a fallback approach for transcoding failures
type FallbackStrategy struct {
	Name        string
	Priority    int
	Condition   func(error) bool
	Execute     func(context.Context, *plugins.TranscodeRequest) (*plugins.TranscodeRequest, error)
	Description string
}

// NewErrorRecoveryManager creates a new error recovery manager
func NewErrorRecoveryManager(logger hclog.Logger) *ErrorRecoveryManager {
	erm := &ErrorRecoveryManager{
		logger:          logger,
		circuitBreakers: make(map[string]*CircuitBreaker),
		maxRetries:      3,
		baseRetryDelay:  time.Second,
	}

	// Initialize standard fallback strategies
	erm.initializeFallbackStrategies()

	return erm
}

// initializeFallbackStrategies sets up common fallback patterns
func (erm *ErrorRecoveryManager) initializeFallbackStrategies() {
	erm.fallbackStrategies = []FallbackStrategy{
		{
			Name:     "codec_fallback",
			Priority: 1,
			Condition: func(err error) bool {
				return isCodecError(err)
			},
			Execute:     erm.fallbackToSaferCodec,
			Description: "Fall back to H.264 codec for maximum compatibility",
		},
		{
			Name:     "quality_reduction",
			Priority: 2,
			Condition: func(err error) bool {
				return isPerformanceError(err)
			},
			Execute:     erm.reduceQuality,
			Description: "Reduce encoding quality to lower computational requirements",
		},
		{
			Name:     "resolution_downscale",
			Priority: 3,
			Condition: func(err error) bool {
				return isResourceError(err)
			},
			Execute:     erm.downscaleResolution,
			Description: "Reduce resolution to conserve resources",
		},
		{
			Name:     "container_fallback",
			Priority: 4,
			Condition: func(err error) bool {
				return isContainerError(err)
			},
			Execute:     erm.fallbackToMP4,
			Description: "Switch to MP4 container for broad compatibility",
		},
		{
			Name:     "disable_abr",
			Priority: 5,
			Condition: func(err error) bool {
				return isABRError(err)
			},
			Execute:     erm.disableABR,
			Description: "Disable adaptive bitrate streaming",
		},
	}
}

// GetCircuitBreaker gets or creates a circuit breaker for a provider
func (erm *ErrorRecoveryManager) GetCircuitBreaker(providerID string) *CircuitBreaker {
	erm.mu.RLock()
	cb, exists := erm.circuitBreakers[providerID]
	erm.mu.RUnlock()

	if exists {
		return cb
	}

	erm.mu.Lock()
	defer erm.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := erm.circuitBreakers[providerID]; exists {
		return cb
	}

	// Create new circuit breaker
	cb = NewCircuitBreaker(
		3,              // failure threshold
		1,              // success threshold
		30*time.Second, // timeout
		erm.logger.Named("circuit-breaker").With("provider", providerID),
	)

	erm.circuitBreakers[providerID] = cb
	return cb
}

// ExecuteWithFallback attempts transcoding with progressive fallback
func (erm *ErrorRecoveryManager) ExecuteWithFallback(
	ctx context.Context,
	request *plugins.TranscodeRequest,
	executor func(context.Context, *plugins.TranscodeRequest) error,
) error {
	var lastErr error
	currentRequest := *request // Copy the request

	// Try initial request
	lastErr = executor(ctx, &currentRequest)
	if lastErr == nil {
		return nil
	}

	erm.logger.Warn("Initial transcoding attempt failed, trying fallback strategies", "error", lastErr)

	// Try each fallback strategy
	for _, strategy := range erm.fallbackStrategies {
		if strategy.Condition(lastErr) {
			erm.logger.Info("Applying fallback strategy", "strategy", strategy.Name, "description", strategy.Description)

			fallbackRequest, err := strategy.Execute(ctx, &currentRequest)
			if err != nil {
				erm.logger.Warn("Fallback strategy failed", "strategy", strategy.Name, "error", err)
				continue
			}

			// Try with fallback request
			lastErr = executor(ctx, fallbackRequest)
			if lastErr == nil {
				erm.logger.Info("Fallback strategy succeeded", "strategy", strategy.Name)
				return nil
			}

			erm.logger.Warn("Fallback strategy attempt failed", "strategy", strategy.Name, "error", lastErr)
			currentRequest = *fallbackRequest // Use the modified request for next fallback
		}
	}

	erm.logger.Error("All fallback strategies exhausted", "final_error", lastErr)
	return fmt.Errorf("transcoding failed after all fallback attempts: %w", lastErr)
}

// ExecuteWithRetry executes an operation with exponential backoff retry
func (erm *ErrorRecoveryManager) ExecuteWithRetry(
	ctx context.Context,
	operation func() error,
	providerID string,
) error {
	cb := erm.GetCircuitBreaker(providerID)

	for attempt := 0; attempt <= erm.maxRetries; attempt++ {
		// Check circuit breaker
		if !cb.Allow() {
			return fmt.Errorf("circuit breaker is open for provider %s", providerID)
		}

		err := operation()
		if err == nil {
			cb.RecordSuccess()
			return nil
		}

		cb.RecordFailure()

		// Don't retry on last attempt
		if attempt == erm.maxRetries {
			return fmt.Errorf("operation failed after %d attempts: %w", erm.maxRetries+1, err)
		}

		// Calculate delay with exponential backoff
		delay := erm.baseRetryDelay * time.Duration(1<<uint(attempt))
		erm.logger.Warn("Operation failed, retrying", "attempt", attempt+1, "delay", delay, "error", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("operation failed after %d attempts", erm.maxRetries+1)
}

// Fallback strategy implementations

func (erm *ErrorRecoveryManager) fallbackToSaferCodec(ctx context.Context, request *plugins.TranscodeRequest) (*plugins.TranscodeRequest, error) {
	newRequest := *request
	newRequest.VideoCodec = "h264" // Most compatible codec
	return &newRequest, nil
}

func (erm *ErrorRecoveryManager) reduceQuality(ctx context.Context, request *plugins.TranscodeRequest) (*plugins.TranscodeRequest, error) {
	newRequest := *request
	if newRequest.Quality > 30 {
		newRequest.Quality = newRequest.Quality - 20 // Reduce quality significantly
	} else {
		newRequest.Quality = 20 // Minimum reasonable quality
	}
	return &newRequest, nil
}

func (erm *ErrorRecoveryManager) downscaleResolution(ctx context.Context, request *plugins.TranscodeRequest) (*plugins.TranscodeRequest, error) {
	newRequest := *request
	if newRequest.Resolution != nil {
		// Downscale by reducing resolution
		if newRequest.Resolution.Height > 720 {
			newRequest.Resolution.Height = 720
			newRequest.Resolution.Width = int(float64(newRequest.Resolution.Height) * 16.0 / 9.0)
		} else if newRequest.Resolution.Height > 480 {
			newRequest.Resolution.Height = 480
			newRequest.Resolution.Width = int(float64(newRequest.Resolution.Height) * 16.0 / 9.0)
		}
	}
	return &newRequest, nil
}

func (erm *ErrorRecoveryManager) fallbackToMP4(ctx context.Context, request *plugins.TranscodeRequest) (*plugins.TranscodeRequest, error) {
	newRequest := *request
	newRequest.Container = "mp4" // Most compatible container
	return &newRequest, nil
}

func (erm *ErrorRecoveryManager) disableABR(ctx context.Context, request *plugins.TranscodeRequest) (*plugins.TranscodeRequest, error) {
	newRequest := *request
	newRequest.EnableABR = false
	return &newRequest, nil
}

// Error classification functions

func isCodecError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "codec", "unsupported codec", "encoding failed", "decoder")
}

func isPerformanceError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "timeout", "performance", "slow", "cpu", "memory")
}

func isResourceError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "memory", "disk", "space", "resource", "limit")
}

func isContainerError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "container", "format", "muxer", "demuxer")
}

func isABRError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "abr", "adaptive", "bitrate", "ladder", "variant")
}

// Helper function to check if error message contains any of the keywords
func contains(errStr string, keywords ...string) bool {
	errStr = fmt.Sprintf("%v", errStr) // Convert to string safely
	for _, keyword := range keywords {
		if len(errStr) > 0 && len(keyword) > 0 {
			// Simple case-insensitive substring check
			for i := 0; i <= len(errStr)-len(keyword); i++ {
				match := true
				for j := 0; j < len(keyword); j++ {
					c1 := errStr[i+j]
					c2 := keyword[j]
					// Simple case conversion
					if c1 >= 'A' && c1 <= 'Z' {
						c1 = c1 - 'A' + 'a'
					}
					if c2 >= 'A' && c2 <= 'Z' {
						c2 = c2 - 'A' + 'a'
					}
					if c1 != c2 {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}
	return false
}

// GetStats returns circuit breaker statistics
func (erm *ErrorRecoveryManager) GetStats() map[string]interface{} {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	stats := make(map[string]interface{})

	circuitBreakerStats := make(map[string]interface{})
	for providerID, cb := range erm.circuitBreakers {
		cb.mu.RLock()
		circuitBreakerStats[providerID] = map[string]interface{}{
			"state":        cb.state,
			"failures":     cb.failures,
			"last_failure": cb.lastFailureTime,
			"next_retry":   cb.nextRetryTime,
		}
		cb.mu.RUnlock()
	}

	stats["circuit_breakers"] = circuitBreakerStats
	stats["fallback_strategies"] = len(erm.fallbackStrategies)
	stats["max_retries"] = erm.maxRetries

	return stats
}
