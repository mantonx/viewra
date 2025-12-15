package scanutil

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// mediaExtensions is a lookup table for supported media file extensions.
// Allocated once at startup to avoid per-call allocation overhead.
var mediaExtensions = map[string]bool{
	// Video
	"mp4": true, "mkv": true, "avi": true, "mov": true, "wmv": true, "flv": true,
	"webm": true, "m4v": true, "mpg": true, "mpeg": true, "m2ts": true, "ts": true,
	"vob": true, "3gp": true, "3g2": true, "f4v": true, "rm": true, "rmvb": true,
	"divx": true, "asf": true, "qt": true, "mts": true, "ogv": true, "mxf": true,
	// Audio
	"mp3": true, "flac": true, "m4a": true, "aac": true, "ogg": true, "opus": true,
	"wav": true, "wma": true, "ape": true, "wv": true, "tta": true, "tak": true,
	"dsf": true, "dff": true, "alac": true, "aiff": true, "aif": true,
}

// AudioExtensions is a lookup table for audio-only file extensions.
// Used to skip audio files in video-only libraries (Movie/TV).
var AudioExtensions = map[string]bool{
	"mp3": true, "flac": true, "m4a": true, "aac": true, "ogg": true, "opus": true,
	"wav": true, "wma": true, "ape": true, "wv": true, "tta": true, "tak": true,
	"dsf": true, "dff": true, "alac": true, "aiff": true, "aif": true,
}

// IsMediaFile checks if a file extension is for a media file.
func IsMediaFile(ext string) bool {
	// Remove leading dot if present
	ext = strings.TrimPrefix(ext, ".")
	return mediaExtensions[strings.ToLower(ext)]
}

// IsAudioFile checks if a file extension is for an audio file.
func IsAudioFile(ext string) bool {
	ext = strings.TrimPrefix(ext, ".")
	return AudioExtensions[strings.ToLower(ext)]
}

// IsExtra determines if a file is an extra (trailer, deleted scene, featurette, etc.)
// based on common filename patterns.
func IsExtra(filepath string) bool {
	lower := strings.ToLower(filepath)
	extraPatterns := []string{
		"-trailer.",
		"_trailer.",
		".trailer.",
		"-deleted",
		"_deleted",
		".deleted",
		"-featurette",
		"_featurette",
		".featurette",
		"-extra.",
		"_extra.",
		".extra.",
		"-bonus.",
		"_bonus.",
		".bonus.",
	}

	for _, pattern := range extraPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// TimeoutConfig holds timeout configuration for file processing.
type TimeoutConfig struct {
	BaseFileTimeout      time.Duration
	RemoteStorageTimeout time.Duration
	MaxExtraTimeout      time.Duration
	IsRemoteStorage      bool
}

// CalculateProcessingTimeout determines appropriate timeout for file processing
// based on file size and storage type to prevent worker deadlocks.
func CalculateProcessingTimeout(fileSize int64, config TimeoutConfig) time.Duration {
	// Base timeout from config (default: 30s local, 60s remote)
	baseTimeout := config.BaseFileTimeout

	// For network storage, be more generous due to latency
	if config.IsRemoteStorage {
		baseTimeout = config.RemoteStorageTimeout
	}

	// Add extra time for large files (1 second per GB)
	// This handles 4K content and large remuxes that take longer to probe
	const bytesPerGB = 1024 * 1024 * 1024
	sizeGB := fileSize / bytesPerGB
	if sizeGB > 0 {
		// Add up to MaxExtraTimeout for very large files
		extraTime := time.Duration(sizeGB) * time.Second
		if extraTime > config.MaxExtraTimeout {
			extraTime = config.MaxExtraTimeout
		}
		baseTimeout += extraTime
	}

	return baseTimeout
}

// StatWithTimeout wraps os.Stat with a timeout to prevent indefinite hangs on network storage.
// os.Stat doesn't support context cancellation, so we run it in a goroutine with a timeout.
func StatWithTimeout(ctx context.Context, path string, timeout time.Duration) (os.FileInfo, error) {
	type result struct {
		info os.FileInfo
		err  error
	}

	resultChan := make(chan result, 1)

	// Run os.Stat in a goroutine
	go func() {
		info, err := os.Stat(path)
		resultChan <- result{info: info, err: err}
	}()

	// Wait for either the stat to complete or timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("stat timeout after %v: %s", timeout, path)
	case res := <-resultChan:
		return res.info, res.err
	}
}
