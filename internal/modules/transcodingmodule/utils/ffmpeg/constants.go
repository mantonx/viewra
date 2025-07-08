// Package ffmpeg provides utilities for FFmpeg-based video transcoding.
// This file contains constants and common structures used across the package.
package ffmpeg

// Common FFmpeg argument constants
var (
	InputArgs = struct {
		SeekStart []string
		Input     []string
	}{
		SeekStart: []string{"-ss"},
		Input:     []string{"-i"},
	}

	StreamMappingArgs = struct {
		Map []string
	}{
		Map: []string{"-map"},
	}

	VideoEncodingArgs = struct {
		Codec       []string
		KeyInt      []string
		KeyIntMin   []string
		ScThreshold []string
	}{
		Codec:       []string{"-c:v"},
		KeyInt:      []string{"-g"},
		KeyIntMin:   []string{"-keyint_min"},
		ScThreshold: []string{"-sc_threshold"},
	}
)
