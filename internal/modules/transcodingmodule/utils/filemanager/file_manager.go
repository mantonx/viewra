package filemanager

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
)

// FileManager provides file system operations for transcoding
type FileManager struct {
	baseDir string
	logger  hclog.Logger
}

// NewFileManager creates a new file manager
func NewFileManager(baseDir string, logger hclog.Logger) *FileManager {
	return &FileManager{
		baseDir: baseDir,
		logger:  logger,
	}
}

// SessionDirectory represents information about a session directory
type SessionDirectory struct {
	Path         string
	SessionID    string
	Provider     string
	Container    string
	Size         int64
	LastModified time.Time
}

// GetOldestSessions returns the oldest session directories sorted by modification time
func (fm *FileManager) GetOldestSessions(limit int) ([]*SessionDirectory, error) {
	entries, err := os.ReadDir(fm.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	var sessions []*SessionDirectory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			fm.logger.Warn("failed to get info for directory", "dir", entry.Name(), "error", err)
			continue
		}

		// Parse directory name to extract session info
		// Format: container_provider_sessionid
		dirName := entry.Name()
		sessionDir := &SessionDirectory{
			Path:         filepath.Join(fm.baseDir, dirName),
			LastModified: info.ModTime(),
		}

		// Try to extract session ID from directory name
		// This is a simplified version - actual parsing would be more robust
		sessionDir.SessionID = dirName

		sessions = append(sessions, sessionDir)
	}

	// Sort by modification time (oldest first)
	// Simple bubble sort for now
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[j].LastModified.Before(sessions[i].LastModified) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	// Limit results
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}

// GetDirectorySize calculates the total size of a directory and its contents
func (fm *FileManager) GetDirectorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

// RemoveDirectory removes a directory and all its contents
func (fm *FileManager) RemoveDirectory(path string) error {
	return os.RemoveAll(path)
}

// CreateSessionDirectory creates a directory for a transcoding session
func (fm *FileManager) CreateSessionDirectory(container, provider, sessionID string) (string, error) {
	// Format: container_provider_sessionid
	dirName := fmt.Sprintf("%s_%s_%s", container, provider, sessionID)
	dirPath := filepath.Join(fm.baseDir, dirName)

	// Create directory with subdirectories
	encodedPath := filepath.Join(dirPath, "encoded")
	packagedPath := filepath.Join(dirPath, "packaged")

	for _, path := range []string{encodedPath, packagedPath} {
		if err := os.MkdirAll(path, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	fm.logger.Debug("created session directory",
		"path", dirPath,
		"container", container,
		"provider", provider,
		"session_id", sessionID)

	return dirPath, nil
}

// GetSessionDirectory returns the directory path for a session
func (fm *FileManager) GetSessionDirectory(container, provider, sessionID string) string {
	dirName := fmt.Sprintf("%s_%s_%s", container, provider, sessionID)
	return filepath.Join(fm.baseDir, dirName)
}

// CleanupEmptyDirectories removes empty directories
func (fm *FileManager) CleanupEmptyDirectories() error {
	entries, err := os.ReadDir(fm.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read base directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(fm.baseDir, entry.Name())

		// Check if directory is empty
		subEntries, err := os.ReadDir(dirPath)
		if err != nil {
			fm.logger.Warn("failed to read directory", "path", dirPath, "error", err)
			continue
		}

		if len(subEntries) == 0 {
			if err := os.Remove(dirPath); err != nil {
				fm.logger.Warn("failed to remove empty directory", "path", dirPath, "error", err)
			} else {
				fm.logger.Debug("removed empty directory", "path", dirPath)
			}
		}
	}

	return nil
}
