// Package session provides session state validation and transition management.
package session

import (
	"fmt"
	"sync"
	"time"
)

// StateValidator provides validation for session state transitions and concurrent access control
type StateValidator struct {
	// State transition rules
	transitions map[Status][]Status

	// Transition timing constraints
	transitionTimeouts map[Status]time.Duration

	// Concurrency control
	mu sync.RWMutex
}

// StateTransitionError represents an invalid state transition error
type StateTransitionError struct {
	SessionID  string
	FromStatus Status
	ToStatus   Status
	Reason     string
}

func (e *StateTransitionError) Error() string {
	return fmt.Sprintf("invalid state transition for session %s: %s -> %s (%s)",
		e.SessionID, e.FromStatus, e.ToStatus, e.Reason)
}

// StateTransitionRule defines rules for state transitions
type StateTransitionRule struct {
	From           Status
	To             Status
	RequiredFields []string       // Fields that must be present
	ValidationFunc ValidationFunc // Custom validation function
	MaxDuration    time.Duration  // Maximum time allowed in source state
	PreConditions  []PreCondition // Conditions that must be met
}

// ValidationFunc is a function that validates a state transition
type ValidationFunc func(session *Session, request interface{}) error

// PreCondition represents a condition that must be met for a transition
type PreCondition struct {
	Name        string
	Description string
	CheckFunc   func(session *Session) bool
}

// NewStateValidator creates a new state validator with default rules
func NewStateValidator() *StateValidator {
	validator := &StateValidator{
		transitions:        make(map[Status][]Status),
		transitionTimeouts: make(map[Status]time.Duration),
	}

	validator.initializeDefaultRules()
	return validator
}

// initializeDefaultRules sets up the default state transition rules
func (sv *StateValidator) initializeDefaultRules() {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	// Define valid transitions
	sv.transitions = map[Status][]Status{
		StatusPending: {
			StatusStarting, // Normal progression
			StatusFailed,   // Failed to start
			StatusStopped,  // Cancelled before starting
		},
		StatusStarting: {
			StatusRunning, // Successfully started
			StatusFailed,  // Failed to start properly
			StatusStopped, // Cancelled during startup
		},
		StatusRunning: {
			StatusComplete, // Successful completion
			StatusFailed,   // Runtime failure
			StatusStopped,  // User cancellation
		},
		StatusComplete: {
			// Terminal state - no further transitions allowed
		},
		StatusFailed: {
			// Terminal state - no further transitions allowed
			// Could potentially add retry logic here in the future
		},
		StatusStopped: {
			// Terminal state - no further transitions allowed
			// Could potentially add restart logic here in the future
		},
	}

	// Define maximum time allowed in each state before automatic timeout
	sv.transitionTimeouts = map[Status]time.Duration{
		StatusPending:  5 * time.Minute,  // Max time to start transcoding
		StatusStarting: 2 * time.Minute,  // Max time for startup
		StatusRunning:  30 * time.Minute, // Max time for transcoding
		StatusComplete: 0,                // No timeout for terminal state
		StatusFailed:   0,                // No timeout for terminal state
		StatusStopped:  0,                // No timeout for terminal state
	}
}

// ValidateTransition validates if a state transition is allowed
func (sv *StateValidator) ValidateTransition(session *Session, toStatus Status, request interface{}) error {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	fromStatus := session.Status

	// Check if transition is allowed
	validTransitions, exists := sv.transitions[fromStatus]
	if !exists {
		return &StateTransitionError{
			SessionID:  session.ID,
			FromStatus: fromStatus,
			ToStatus:   toStatus,
			Reason:     "no transitions defined for current state",
		}
	}

	// Check if the target state is in the list of valid transitions
	isValid := false
	for _, validState := range validTransitions {
		if validState == toStatus {
			isValid = true
			break
		}
	}

	if !isValid {
		return &StateTransitionError{
			SessionID:  session.ID,
			FromStatus: fromStatus,
			ToStatus:   toStatus,
			Reason:     "transition not allowed",
		}
	}

	// Check state timeout constraints
	if err := sv.validateStateTimeout(session, fromStatus); err != nil {
		return err
	}

	// Validate specific transition requirements
	if err := sv.validateTransitionRequirements(session, fromStatus, toStatus, request); err != nil {
		return err
	}

	return nil
}

// validateStateTimeout checks if the session has been in the current state too long
func (sv *StateValidator) validateStateTimeout(session *Session, currentStatus Status) error {
	timeout, exists := sv.transitionTimeouts[currentStatus]
	if !exists || timeout == 0 {
		return nil // No timeout for this state
	}

	timeInState := time.Since(session.StartTime)
	if timeInState > timeout {
		return &StateTransitionError{
			SessionID:  session.ID,
			FromStatus: currentStatus,
			ToStatus:   currentStatus,
			Reason:     fmt.Sprintf("state timeout exceeded: %v > %v", timeInState, timeout),
		}
	}

	return nil
}

// validateTransitionRequirements validates specific requirements for state transitions
func (sv *StateValidator) validateTransitionRequirements(session *Session, from, to Status, request interface{}) error {
	// Validate transition to StatusStarting
	if to == StatusStarting {
		if session.Handle == nil {
			return &StateTransitionError{
				SessionID:  session.ID,
				FromStatus: from,
				ToStatus:   to,
				Reason:     "transcode handle required for starting state",
			}
		}
	}

	// Validate transition to StatusRunning
	if to == StatusRunning {
		if session.Process == nil {
			return &StateTransitionError{
				SessionID:  session.ID,
				FromStatus: from,
				ToStatus:   to,
				Reason:     "process required for running state",
			}
		}
	}

	// Validate transition to StatusComplete
	if to == StatusComplete {
		// Could add validation for completion requirements
		// e.g., output files exist, progress is 100%, etc.
	}

	// Validate transition to StatusFailed
	if to == StatusFailed {
		if session.Error == nil && request == nil {
			return &StateTransitionError{
				SessionID:  session.ID,
				FromStatus: from,
				ToStatus:   to,
				Reason:     "error information required for failed state",
			}
		}
	}

	return nil
}

// GetValidTransitions returns the valid transitions from a given state
func (sv *StateValidator) GetValidTransitions(from Status) []Status {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	transitions, exists := sv.transitions[from]
	if !exists {
		return []Status{}
	}

	// Return a copy to prevent modification
	result := make([]Status, len(transitions))
	copy(result, transitions)
	return result
}

// IsTerminalState checks if a state is terminal (no further transitions)
func (sv *StateValidator) IsTerminalState(status Status) bool {
	transitions := sv.GetValidTransitions(status)
	return len(transitions) == 0
}

// GetStateTimeout returns the maximum allowed time in a state
func (sv *StateValidator) GetStateTimeout(status Status) time.Duration {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	timeout, exists := sv.transitionTimeouts[status]
	if !exists {
		return 0
	}
	return timeout
}

// AddTransitionRule adds a custom transition rule
func (sv *StateValidator) AddTransitionRule(from, to Status) error {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	if sv.transitions[from] == nil {
		sv.transitions[from] = []Status{}
	}

	// Check if transition already exists
	for _, existing := range sv.transitions[from] {
		if existing == to {
			return fmt.Errorf("transition rule %s -> %s already exists", from, to)
		}
	}

	sv.transitions[from] = append(sv.transitions[from], to)
	return nil
}

// RemoveTransitionRule removes a transition rule
func (sv *StateValidator) RemoveTransitionRule(from, to Status) error {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	transitions, exists := sv.transitions[from]
	if !exists {
		return fmt.Errorf("no transitions defined for state %s", from)
	}

	// Find and remove the transition
	for i, transition := range transitions {
		if transition == to {
			sv.transitions[from] = append(transitions[:i], transitions[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("transition rule %s -> %s not found", from, to)
}

// SetStateTimeout sets the maximum allowed time in a state
func (sv *StateValidator) SetStateTimeout(status Status, timeout time.Duration) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	sv.transitionTimeouts[status] = timeout
}

// ValidateSessionConsistency validates the overall consistency of a session
func (sv *StateValidator) ValidateSessionConsistency(session *Session) []error {
	var errors []error

	// Check if session has required fields
	if session.ID == "" {
		errors = append(errors, fmt.Errorf("session ID is required"))
	}

	// Check if status is valid
	validStatuses := []Status{StatusPending, StatusStarting, StatusRunning, StatusComplete, StatusFailed, StatusStopped}
	isValidStatus := false
	for _, status := range validStatuses {
		if session.Status == status {
			isValidStatus = true
			break
		}
	}
	if !isValidStatus {
		errors = append(errors, fmt.Errorf("invalid session status: %s", session.Status))
	}

	// Check progress consistency
	if session.Status == StatusComplete && session.Progress < 100 {
		errors = append(errors, fmt.Errorf("completed session should have 100%% progress, got %.2f%%", session.Progress))
	}

	if session.Progress < 0 || session.Progress > 100 {
		errors = append(errors, fmt.Errorf("invalid progress value: %.2f%% (must be 0-100)", session.Progress))
	}

	// Check error consistency
	if session.Status == StatusFailed && session.Error == nil {
		errors = append(errors, fmt.Errorf("failed session must have an error"))
	}

	if session.Status != StatusFailed && session.Error != nil {
		errors = append(errors, fmt.Errorf("non-failed session should not have an error"))
	}

	// Check handle consistency
	if (session.Status == StatusStarting || session.Status == StatusRunning) && session.Handle == nil {
		errors = append(errors, fmt.Errorf("active session must have a transcode handle"))
	}

	// Check process consistency
	if session.Status == StatusRunning && session.Process == nil {
		errors = append(errors, fmt.Errorf("running session must have an associated process"))
	}

	// Check timing consistency
	if session.StartTime.IsZero() {
		errors = append(errors, fmt.Errorf("session must have a start time"))
	}

	if session.StartTime.After(time.Now()) {
		errors = append(errors, fmt.Errorf("session start time cannot be in the future"))
	}

	return errors
}

// GetTransitionMatrix returns the complete state transition matrix
func (sv *StateValidator) GetTransitionMatrix() map[Status][]Status {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	// Return a deep copy to prevent modification
	result := make(map[Status][]Status)
	for from, toStates := range sv.transitions {
		result[from] = make([]Status, len(toStates))
		copy(result[from], toStates)
	}

	return result
}
