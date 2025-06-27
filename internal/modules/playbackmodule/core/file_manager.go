package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

// FileManager handles file operations for transcoding sessions
type FileManager struct {
	baseDir string
	logger  hclog.Logger
}

// NewFileManager creates a new file manager
func NewFileManager(baseDir string, logger hclog.Logger) *FileManager {
	// Get base directory from environment if available
	if envDir := os.Getenv("VIEWRA_TRANSCODING_DIR"); envDir != "" {
		baseDir = envDir
	}

	return &FileManager{
		baseDir: baseDir,
		logger:  logger.Named("file-manager"),
	}
}

// CreateSessionDirectory creates a directory for a transcoding session
func (fm *FileManager) CreateSessionDirectory(sessionID, provider, container string) (string, error) {
	// Directory format: container_provider_sessionid
	dirName := fmt.Sprintf("%s_%s_%s", container, provider, sessionID)
	dirPath := filepath.Join(fm.baseDir, dirName)

	// Create main directory
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create encoded and packaged subdirectories
	encodedDir := filepath.Join(dirPath, "encoded")
	if err := os.MkdirAll(encodedDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create encoded directory: %w", err)
	}

	packagedDir := filepath.Join(dirPath, "packaged")
	if err := os.MkdirAll(packagedDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create packaged directory: %w", err)
	}

	fm.logger.Debug("created session directory with subdirectories", "path", dirPath)
	return dirPath, nil
}

// GetSessionDirectory returns the directory path for a session
func (fm *FileManager) GetSessionDirectory(sessionID string) (string, error) {
	// Search for directory containing the session ID
	entries, err := os.ReadDir(fm.baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to read base directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(entry.Name(), sessionID) {
			return filepath.Join(fm.baseDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("session directory not found: %s", sessionID)
}

// RemoveSessionDirectory removes a session's directory
func (fm *FileManager) RemoveSessionDirectory(sessionID string) error {
	dirPath, err := fm.GetSessionDirectory(sessionID)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(dirPath); err != nil {
		return fmt.Errorf("failed to remove session directory: %w", err)
	}

	fm.logger.Debug("removed session directory", "path", dirPath)
	return nil
}

// GetManifestPath returns the path to the manifest file for a session
func (fm *FileManager) GetManifestPath(sessionID string) (string, error) {
	dirPath, err := fm.GetSessionDirectory(sessionID)
	if err != nil {
		return "", err
	}

	// Check for common manifest files
	manifestNames := []string{
		"manifest.mpd",  // DASH
		"playlist.m3u8", // HLS
		"master.m3u8",   // HLS alternative
		"index.m3u8",    // HLS alternative
	}

	for _, name := range manifestNames {
		path := filepath.Join(dirPath, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("manifest file not found in session directory")
}

// ListSegments returns all segment files in a session directory
func (fm *FileManager) ListSegments(sessionID string) ([]string, error) {
	dirPath, err := fm.GetSessionDirectory(sessionID)
	if err != nil {
		return nil, err
	}

	// Look for segments in the packaged directory
	packagedDir := filepath.Join(dirPath, "packaged")
	var segments []string

	// Walk the packaged directory if it exists
	if _, err := os.Stat(packagedDir); err == nil {
		err = filepath.Walk(packagedDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				ext := strings.ToLower(filepath.Ext(path))
				// Common segment extensions
				if ext == ".ts" || ext == ".m4s" || ext == ".mp4" || ext == ".webm" {
					segments = append(segments, path)
				}
			}
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to list segments: %w", err)
		}
	}

	return segments, nil
}

// GetDirectorySize returns the total size of a directory
func (fm *FileManager) GetDirectorySize(dirPath string) (int64, error) {
	var size int64
	err := filepath.Walk(dirPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to calculate directory size: %w", err)
	}

	return size, nil
}

// GetOldestSessions returns the oldest session directories sorted by modification time
func (fm *FileManager) GetOldestSessions(limit int) ([]SessionDirectory, error) {
	entries, err := os.ReadDir(fm.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	var sessions []SessionDirectory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		sessions = append(sessions, SessionDirectory{
			Path:         filepath.Join(fm.baseDir, entry.Name()),
			Name:         entry.Name(),
			LastModified: info.ModTime(),
		})
	}

	// Sort by modification time (oldest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastModified.Before(sessions[j].LastModified)
	})

	// Return up to limit
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}

// GetTotalSize returns the total size of all transcoding directories
func (fm *FileManager) GetTotalSize() (int64, error) {
	var totalSize int64

	entries, err := os.ReadDir(fm.baseDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read base directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(fm.baseDir, entry.Name())
		size, err := fm.GetDirectorySize(dirPath)
		if err != nil {
			fm.logger.Warn("failed to get directory size", "path", dirPath, "error", err)
			continue
		}
		totalSize += size
	}

	return totalSize, nil
}

// EnsureBaseDirectory ensures the base transcoding directory exists
func (fm *FileManager) EnsureBaseDirectory() error {
	if err := os.MkdirAll(fm.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}
	return nil
}

// SessionDirectory represents a session directory
type SessionDirectory struct {
	Path         string
	Name         string
	LastModified time.Time
}
