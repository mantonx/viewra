// Package types provides types and interfaces for the transcoding module.
package types

import "time"

// PipelineStatus represents the status of the pipeline provider
type PipelineStatus struct {
	Available        bool     `json:"available"`
	ActiveJobs       int      `json:"activeJobs"`
	CompletedJobs    int      `json:"completedJobs"`
	FailedJobs       int      `json:"failedJobs"`
	FFmpegVersion    string   `json:"ffmpegVersion"`
	SupportedFormats []string `json:"supportedFormats"`
}

// PipelineConfig represents configuration for the pipeline
type PipelineConfig struct {
	BaseDir                string
	MaxRetries             int
	RetryDelay             time.Duration
	MaxConcurrentEncoding  int
	MaxConcurrentPackaging int
}