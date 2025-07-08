// Package types provides types and interfaces for the transcoding module.
package types

import "time"

// Config holds configuration for the transcoding module
type Config struct {
	// TranscodingDir is the base directory for transcoding operations
	TranscodingDir string

	// MaxConcurrentSessions limits concurrent transcoding sessions
	MaxConcurrentSessions int

	// SessionTimeout is the maximum duration for a transcoding session
	SessionTimeout time.Duration

	// CleanupInterval is how often to run cleanup
	CleanupInterval time.Duration

	// RetentionPeriod is how long to keep completed sessions
	RetentionPeriod time.Duration
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		TranscodingDir:        "/app/viewra-data/transcoding",
		MaxConcurrentSessions: 5,
		SessionTimeout:        2 * time.Hour,
		CleanupInterval:       30 * time.Minute,
		RetentionPeriod:       24 * time.Hour,
	}
}