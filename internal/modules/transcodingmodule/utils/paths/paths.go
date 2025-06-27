// Package paths provides utilities for managing streaming transcoding directory paths.
// It ensures consistent path generation for streaming-first architecture with
// segment-based storage and real-time manifest generation.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// GenerateSessionPath creates the standard session directory path.
// Pattern: [container]_[provider]_[sessionId]
// Example: dash_ffmpeg_software_550e8400-e29b-41d4-a716-446655440000
func GenerateSessionPath(baseDir, container, provider, sessionID string) string {
	return filepath.Join(baseDir, fmt.Sprintf("%s_%s_%s", container, provider, sessionID))
}

// CreateSessionDirectories creates streaming-optimized directory structure for a session.
// This includes segments/, manifests/, init/, video/, and audio/ subdirectories.
func CreateSessionDirectories(sessionPath string) error {
	// Create main session directory
	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create streaming-specific subdirectories
	dirs := []string{
		filepath.Join(sessionPath, "segments"),  // Real-time generated segments
		filepath.Join(sessionPath, "manifests"), // Dynamic manifests
		filepath.Join(sessionPath, "init"),      // Initialization segments
		filepath.Join(sessionPath, "video"),     // Video quality variants
		filepath.Join(sessionPath, "audio"),     // Audio tracks
		filepath.Join(sessionPath, "logs"),      // Session logs
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// GetManifestFilename returns the appropriate manifest filename for the container type.
func GetManifestFilename(container string) string {
	switch container {
	case "dash":
		return "manifest.mpd"
	case "hls":
		return "playlist.m3u8"
	case "mp4":
		return "output.mp4"
	default:
		return fmt.Sprintf("output.%s", container)
	}
}

// GetManifestPath returns the full path to the manifest file.
// Manifests are always placed at the root of the session directory.
func GetManifestPath(sessionPath, container string) string {
	return filepath.Join(sessionPath, GetManifestFilename(container))
}

// GetSegmentsPath returns the path to the segments subdirectory.
func GetSegmentsPath(sessionPath string) string {
	return filepath.Join(sessionPath, "segments")
}

// GetManifestsPath returns the path to the manifests subdirectory.
func GetManifestsPath(sessionPath string) string {
	return filepath.Join(sessionPath, "manifests")
}

// GetInitPath returns the path to the init subdirectory.
func GetInitPath(sessionPath string) string {
	return filepath.Join(sessionPath, "init")
}

// GetVideoPath returns the path to the video subdirectory.
func GetVideoPath(sessionPath string) string {
	return filepath.Join(sessionPath, "video")
}

// GetAudioPath returns the path to the audio subdirectory.
func GetAudioPath(sessionPath string) string {
	return filepath.Join(sessionPath, "audio")
}

// GetStreamingPaths returns all streaming-related paths for a session.
func GetStreamingPaths(sessionPath string) StreamingPaths {
	return StreamingPaths{
		SessionPath:   sessionPath,
		SegmentsPath:  GetSegmentsPath(sessionPath),
		ManifestsPath: GetManifestsPath(sessionPath),
		InitPath:      GetInitPath(sessionPath),
		VideoPath:     GetVideoPath(sessionPath),
		AudioPath:     GetAudioPath(sessionPath),
		LogsPath:      filepath.Join(sessionPath, "logs"),
	}
}

// StreamingPaths contains all paths for a streaming session.
type StreamingPaths struct {
	SessionPath   string
	SegmentsPath  string
	ManifestsPath string
	InitPath      string
	VideoPath     string
	AudioPath     string
	LogsPath      string
}

// ParseSessionDirectory extracts components from a session directory name.
// Input: "dash_ffmpeg_software_550e8400-e29b-41d4-a716-446655440000"
// Returns: container="dash", provider="ffmpeg_software", sessionID="550e8400..."
func ParseSessionDirectory(dirName string) (container, provider, sessionID string, err error) {
	// Known multi-part provider names
	providers := []string{
		"ffmpeg_software",
		"ffmpeg_nvidia",
		"ffmpeg_pipeline",
	}

	// Try to match known providers
	for _, p := range providers {
		prefix := fmt.Sprintf("_%s_", p)
		if idx := findProviderIndex(dirName, prefix); idx != -1 {
			// Found a match
			container = dirName[:idx]
			provider = p
			sessionID = dirName[idx+len(prefix):]
			return container, provider, sessionID, nil
		}
	}

	// Fallback: assume single underscore provider (legacy)
	firstUnderscore := -1
	lastUnderscore := -1

	for i, char := range dirName {
		if char == '_' {
			if firstUnderscore == -1 {
				firstUnderscore = i
			}
			lastUnderscore = i
		}
	}

	if firstUnderscore != -1 && lastUnderscore != -1 && firstUnderscore != lastUnderscore {
		container = dirName[:firstUnderscore]
		provider = dirName[firstUnderscore+1 : lastUnderscore]
		sessionID = dirName[lastUnderscore+1:]
		return container, provider, sessionID, nil
	}

	return "", "", "", fmt.Errorf("invalid session directory format: %s", dirName)
}

// findProviderIndex finds the index where the provider pattern starts
func findProviderIndex(s, pattern string) int {
	for i := 0; i <= len(s)-len(pattern); i++ {
		if s[i:i+len(pattern)] == pattern {
			return i
		}
	}
	return -1
}

// GetTranscodingBaseDir returns the base transcoding directory from environment or default
func GetTranscodingBaseDir() string {
	if dir := os.Getenv("VIEWRA_TRANSCODING_DIR"); dir != "" {
		return dir
	}
	return "/app/viewra-data/transcoding"
}
