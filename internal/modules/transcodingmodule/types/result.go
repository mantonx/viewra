// Package types provides types and interfaces for the transcoding module.
package types

import "time"

// EncodedFile represents an encoded output file
type EncodedFile struct {
	Path     string
	Profile  EncodingProfile
	Size     int64
	Duration time.Duration
}

// EncodingResult represents the result of encoding stage
type EncodingResult struct {
	OutputFiles    []EncodedFile
	ProcessingTime time.Duration
}

// PipelineResult represents the final result of the transcoding pipeline
type PipelineResult struct {
	SessionID    string
	ContentHash  string
	ManifestURL  string
	StreamURL    string
	Duration     time.Duration
	EncodedFiles []EncodedFile
	PackagedDir  string
	Metadata     PipelineMetadata
}

// PipelineMetadata contains metadata about the pipeline execution
type PipelineMetadata struct {
	ProcessingTime time.Duration
	EncodingTime   time.Duration
	PackagingTime  time.Duration
	TotalSize      int64
}