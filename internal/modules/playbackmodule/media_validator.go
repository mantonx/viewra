package playbackmodule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

// MediaValidationResult contains the result of media file validation
type MediaValidationResult struct {
	IsValid       bool                   `json:"is_valid"`
	FileExists    bool                   `json:"file_exists"`
	IsReadable    bool                   `json:"is_readable"`
	SizeBytes     int64                  `json:"size_bytes"`
	FileFormat    string                 `json:"file_format"`
	IsCorrupted   bool                   `json:"is_corrupted"`
	HasVideoTrack bool                   `json:"has_video_track"`
	HasAudioTrack bool                   `json:"has_audio_track"`
	Duration      float64                `json:"duration_seconds"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	Warnings      []string               `json:"warnings,omitempty"`
	ValidationInfo map[string]interface{} `json:"validation_info,omitempty"`
}

// MediaValidator interface for validating media files
type MediaValidator interface {
	ValidateMedia(ctx context.Context, mediaPath string) (*MediaValidationResult, error)
	QuickValidate(mediaPath string) (*MediaValidationResult, error)
	CheckFileIntegrity(mediaPath string) (bool, error)
}

// StandardMediaValidator implements comprehensive media validation
type StandardMediaValidator struct {
	logger              hclog.Logger
	maxFileSize         int64 // Maximum file size in bytes (0 = no limit)
	minFileSize         int64 // Minimum file size in bytes
	supportedExtensions []string
	validationTimeout   time.Duration
}

// NewStandardMediaValidator creates a new media validator
func NewStandardMediaValidator(logger hclog.Logger) MediaValidator {
	return &StandardMediaValidator{
		logger:              logger,
		maxFileSize:         50 * 1024 * 1024 * 1024, // 50GB default limit
		minFileSize:         1024,                      // 1KB minimum
		validationTimeout:   30 * time.Second,
		supportedExtensions: []string{
			".mp4", ".mkv", ".avi", ".mov", ".webm", ".flv", ".wmv", ".m4v",
			".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a", ".wma",
			".ts", ".m2ts", ".mts", ".3gp", ".3g2", ".f4v", ".asf",
		},
	}
}

// ValidateMedia performs comprehensive validation of a media file
func (v *StandardMediaValidator) ValidateMedia(ctx context.Context, mediaPath string) (*MediaValidationResult, error) {
	v.logger.Debug("Starting comprehensive media validation", "path", mediaPath)
	
	result := &MediaValidationResult{
		ValidationInfo: make(map[string]interface{}),
	}
	
	// Step 1: Basic file validation
	if err := v.validateBasicFile(mediaPath, result); err != nil {
		result.ErrorMessage = err.Error()
		return result, nil
	}
	
	// Step 2: File format validation
	if err := v.validateFileFormat(mediaPath, result); err != nil {
		v.logger.Warn("File format validation failed", "path", mediaPath, "error", err)
		result.Warnings = append(result.Warnings, fmt.Sprintf("Format validation: %v", err))
	}
	
	// Step 3: File integrity check (basic corruption detection)
	if corrupted, err := v.CheckFileIntegrity(mediaPath); err != nil {
		v.logger.Warn("File integrity check failed", "path", mediaPath, "error", err)
		result.Warnings = append(result.Warnings, fmt.Sprintf("Integrity check failed: %v", err))
	} else {
		result.IsCorrupted = corrupted
	}
	
	// Step 4: Media-specific validation (if we have access to FFprobe)
	if err := v.validateMediaContent(ctx, mediaPath, result); err != nil {
		v.logger.Warn("Media content validation failed", "path", mediaPath, "error", err)
		result.Warnings = append(result.Warnings, fmt.Sprintf("Content validation: %v", err))
	}
	
	// Determine overall validity
	result.IsValid = result.FileExists && result.IsReadable && !result.IsCorrupted && 
		(result.HasVideoTrack || result.HasAudioTrack) && result.Duration > 0
	
	v.logger.Debug("Media validation completed", 
		"path", mediaPath, 
		"valid", result.IsValid, 
		"warnings", len(result.Warnings))
	
	return result, nil
}

// QuickValidate performs fast validation for immediate feedback
func (v *StandardMediaValidator) QuickValidate(mediaPath string) (*MediaValidationResult, error) {
	v.logger.Debug("Starting quick media validation", "path", mediaPath)
	
	result := &MediaValidationResult{
		ValidationInfo: make(map[string]interface{}),
	}
	
	// Basic file validation only
	if err := v.validateBasicFile(mediaPath, result); err != nil {
		result.ErrorMessage = err.Error()
		return result, nil
	}
	
	// Quick format check
	if err := v.validateFileFormat(mediaPath, result); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Format check: %v", err))
	}
	
	// Quick validation assumes valid if basic checks pass
	result.IsValid = result.FileExists && result.IsReadable && result.SizeBytes >= v.minFileSize
	
	return result, nil
}

// validateBasicFile performs basic file system validation
func (v *StandardMediaValidator) validateBasicFile(mediaPath string, result *MediaValidationResult) error {
	// Check if file exists
	fileInfo, err := os.Stat(mediaPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.FileExists = false
			return fmt.Errorf("file does not exist: %s", mediaPath)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}
	
	result.FileExists = true
	result.SizeBytes = fileInfo.Size()
	
	// Check if it's a regular file (not a directory or device)
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file: %s", mediaPath)
	}
	
	// Check file size limits
	if v.maxFileSize > 0 && result.SizeBytes > v.maxFileSize {
		return fmt.Errorf("file too large: %d bytes (limit: %d bytes)", result.SizeBytes, v.maxFileSize)
	}
	
	if result.SizeBytes < v.minFileSize {
		return fmt.Errorf("file too small: %d bytes (minimum: %d bytes)", result.SizeBytes, v.minFileSize)
	}
	
	// Check if file is readable
	file, err := os.Open(mediaPath)
	if err != nil {
		return fmt.Errorf("file is not readable: %w", err)
	}
	file.Close()
	
	result.IsReadable = true
	return nil
}

// validateFileFormat checks if the file extension is supported
func (v *StandardMediaValidator) validateFileFormat(mediaPath string, result *MediaValidationResult) error {
	ext := strings.ToLower(filepath.Ext(mediaPath))
	result.FileFormat = ext
	
	if ext == "" {
		return fmt.Errorf("file has no extension")
	}
	
	// Check if extension is supported
	supported := false
	for _, supportedExt := range v.supportedExtensions {
		if ext == supportedExt {
			supported = true
			break
		}
	}
	
	if !supported {
		return fmt.Errorf("unsupported file format: %s", ext)
	}
	
	return nil
}

// CheckFileIntegrity performs basic corruption detection
func (v *StandardMediaValidator) CheckFileIntegrity(mediaPath string) (bool, error) {
	// Open file for reading
	file, err := os.Open(mediaPath)
	if err != nil {
		return true, fmt.Errorf("cannot open file for integrity check: %w", err)
	}
	defer file.Close()
	
	// Check file header for common video/audio formats
	header := make([]byte, 32)
	n, err := file.Read(header)
	if err != nil || n < 8 {
		return true, fmt.Errorf("cannot read file header")
	}
	
	// Basic magic number checks for common formats
	if !v.hasValidMagicNumber(header) {
		return true, nil // Potentially corrupted
	}
	
	// Check if file can be read to the end (basic corruption test)
	fileInfo, err := file.Stat()
	if err != nil {
		return true, fmt.Errorf("cannot get file info")
	}
	
	// Seek to near the end and try to read
	if fileInfo.Size() > 1024 {
		_, err = file.Seek(-1024, 2) // Seek to 1KB from end
		if err != nil {
			return true, fmt.Errorf("cannot seek to end of file")
		}
		
		tail := make([]byte, 1024)
		_, err = file.Read(tail)
		if err != nil {
			return true, fmt.Errorf("cannot read end of file")
		}
	}
	
	return false, nil // File appears intact
}

// hasValidMagicNumber checks for valid magic numbers in file headers
func (v *StandardMediaValidator) hasValidMagicNumber(header []byte) bool {
	if len(header) < 8 {
		return false
	}
	
	// MP4/M4V/MOV: "ftyp" at offset 4
	if len(header) >= 8 && string(header[4:8]) == "ftyp" {
		return true
	}
	
	// AVI: "RIFF" at start, "AVI " at offset 8
	if len(header) >= 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "AVI " {
		return true
	}
	
	// MKV/WebM: EBML signature
	if len(header) >= 4 && header[0] == 0x1A && header[1] == 0x45 && header[2] == 0xDF && header[3] == 0xA3 {
		return true
	}
	
	// FLV: "FLV" at start
	if len(header) >= 3 && string(header[0:3]) == "FLV" {
		return true
	}
	
	// MP3: ID3 tag or MPEG sync
	if len(header) >= 3 && (string(header[0:3]) == "ID3" || (header[0] == 0xFF && (header[1]&0xE0) == 0xE0)) {
		return true
	}
	
	// FLAC: "fLaC"
	if len(header) >= 4 && string(header[0:4]) == "fLaC" {
		return true
	}
	
	// WAV: "RIFF" and "WAVE"
	if len(header) >= 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WAVE" {
		return true
	}
	
	// OGG: "OggS"
	if len(header) >= 4 && string(header[0:4]) == "OggS" {
		return true
	}
	
	return false
}

// validateMediaContent performs detailed media content validation
func (v *StandardMediaValidator) validateMediaContent(_ context.Context, mediaPath string, result *MediaValidationResult) error {
	// This is a placeholder for more advanced validation
	// In a real implementation, this would use FFprobe or similar tools
	// to validate the actual media streams, duration, etc.
	
	// For now, we'll use basic heuristics based on file extension
	ext := strings.ToLower(filepath.Ext(mediaPath))
	
	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".webm", ".flv", ".wmv", ".m4v", ".ts", ".m2ts", ".mts"}
	audioExts := []string{".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a", ".wma"}
	
	for _, videoExt := range videoExts {
		if ext == videoExt {
			result.HasVideoTrack = true
			result.HasAudioTrack = true // Most video files have audio
			result.Duration = 3600      // Default 1 hour assumption
			break
		}
	}
	
	for _, audioExt := range audioExts {
		if ext == audioExt {
			result.HasAudioTrack = true
			result.Duration = 300 // Default 5 minutes assumption
			break
		}
	}
	
	return nil
}

// MediaValidationError represents a media validation error
type MediaValidationError struct {
	Path     string
	Reason   string
	Original error
}

func (e *MediaValidationError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("media validation failed for %s: %s (%v)", e.Path, e.Reason, e.Original)
	}
	return fmt.Sprintf("media validation failed for %s: %s", e.Path, e.Reason)
}

func (e *MediaValidationError) Unwrap() error {
	return e.Original
}

// NewMediaValidationError creates a new media validation error
func NewMediaValidationError(path, reason string, original error) *MediaValidationError {
	return &MediaValidationError{
		Path:     path,
		Reason:   reason,
		Original: original,
	}
}