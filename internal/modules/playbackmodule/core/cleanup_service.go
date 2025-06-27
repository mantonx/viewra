package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	plugins "github.com/mantonx/viewra/sdk"
)

// CleanupService provides centralized cleanup for all transcoding providers
type CleanupService struct {
	baseDir         string
	store           *SessionStore
	fileManager     *FileManager
	logger          hclog.Logger
	policies        map[string]RetentionPolicy
	config          CleanupConfig
	processRegistry *ProcessRegistry
}

// CleanupConfig contains cleanup configuration
type CleanupConfig struct {
	BaseDirectory      string
	RetentionHours     int
	ExtendedHours      int
	MaxTotalSizeGB     int64
	CleanupInterval    time.Duration
	LargeFileThreshold int64
	ProviderOverrides  map[string]ProviderCleanupConfig
}

// ProviderCleanupConfig contains provider-specific cleanup settings
type ProviderCleanupConfig struct {
	RetentionHours int
	MaxSessions    int
	MaxSizeGB      int64
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(config CleanupConfig, store *SessionStore, fileManager *FileManager, logger hclog.Logger) *CleanupService {
	// Get base directory from environment or config
	baseDir := config.BaseDirectory
	if envDir := os.Getenv("VIEWRA_TRANSCODING_DIR"); envDir != "" {
		baseDir = envDir
	}

	return &CleanupService{
		baseDir:         baseDir,
		store:           store,
		fileManager:     fileManager,
		logger:          logger.Named("cleanup-service"),
		config:          config,
		policies:        make(map[string]RetentionPolicy),
		processRegistry: GetProcessRegistry(logger),
	}
}

// Run starts the cleanup service
func (cs *CleanupService) Run(ctx context.Context) {
	cs.logger.Info("starting cleanup service",
		"interval", cs.config.CleanupInterval,
		"base_dir", cs.baseDir)

	ticker := time.NewTicker(cs.config.CleanupInterval)
	defer ticker.Stop()

	// Run initial cleanup including orphaned processes
	cs.CleanupOrphanedProcesses()
	cs.cleanupAllProviders()

	for {
		select {
		case <-ticker.C:
			cs.cleanupAllProviders()
			// Also check for orphaned processes every cycle
			cs.CleanupOrphanedProcesses()
		case <-ctx.Done():
			cs.logger.Info("cleanup service stopped")
			return
		}
	}
}

// cleanupAllProviders runs cleanup for all providers
func (cs *CleanupService) cleanupAllProviders() {
	cs.logger.Debug("running cleanup cycle")

	// Get cleanup statistics
	stats, err := cs.GetCleanupStats()
	if err != nil {
		cs.logger.Error("failed to get cleanup stats", "error", err)
		return
	}

	cs.logger.Info("cleanup stats",
		"total_sessions", stats.TotalSessions,
		"total_size", plugins.FormatBytes(stats.TotalSize),
		"oldest_session", stats.OldestSession)

	// Check if we need emergency cleanup due to size
	if stats.TotalSize > cs.config.MaxTotalSizeGB*1024*1024*1024 {
		cs.logger.Warn("total size exceeds limit, running emergency cleanup",
			"current_size", plugins.FormatBytes(stats.TotalSize),
			"limit", fmt.Sprintf("%dGB", cs.config.MaxTotalSizeGB))
		cs.runEmergencyCleanup(stats.TotalSize)
	}

	// Run standard cleanup based on retention policy
	policy := RetentionPolicy{
		RetentionHours:     cs.config.RetentionHours,
		ExtendedHours:      cs.config.ExtendedHours,
		MaxTotalSizeGB:     cs.config.MaxTotalSizeGB,
		LargeFileThreshold: cs.config.LargeFileThreshold,
	}

	// Clean up expired sessions from database
	dbCount, err := cs.store.CleanupExpiredSessions(policy)
	if err != nil {
		cs.logger.Error("failed to cleanup database sessions", "error", err)
	} else if dbCount > 0 {
		cs.logger.Info("cleaned up database sessions", "count", dbCount)
	}

	// Clean up stale running/queued sessions (stuck for more than 30 minutes)
	staleTimeout := 30 * time.Minute
	staleCount, err := cs.store.CleanupStaleSessions(staleTimeout)
	if err != nil {
		cs.logger.Error("failed to cleanup stale sessions", "error", err)
	} else if staleCount > 0 {
		cs.logger.Info("cleaned up stale sessions", "count", staleCount, "timeout", staleTimeout)
	}

	// Also clean up sessions that are making no progress
	noProgressCount, err := cs.cleanupNoProgressSessions()
	if err != nil {
		cs.logger.Error("failed to cleanup no-progress sessions", "error", err)
	} else if noProgressCount > 0 {
		cs.logger.Info("cleaned up sessions with no progress", "count", noProgressCount)
	}

	// Clean up orphaned directories
	orphanCount, err := cs.cleanupOrphanedDirectories()
	if err != nil {
		cs.logger.Error("failed to cleanup orphaned directories", "error", err)
	} else if orphanCount > 0 {
		cs.logger.Info("cleaned up orphaned directories", "count", orphanCount)
	}

	// Clean up orphaned processes
	cs.CleanupOrphanedProcesses()
}

// runEmergencyCleanup removes files to get under size limit
func (cs *CleanupService) runEmergencyCleanup(currentSize int64) {
	targetSize := cs.config.MaxTotalSizeGB * 1024 * 1024 * 1024 * 90 / 100 // Target 90% of limit

	// Get sessions sorted by last accessed (oldest first)
	sessions, err := cs.fileManager.GetOldestSessions(100)
	if err != nil {
		cs.logger.Error("failed to get oldest sessions", "error", err)
		return
	}

	freedSize := int64(0)
	removedCount := 0

	for _, sessionDir := range sessions {
		if currentSize-freedSize <= targetSize {
			break
		}

		// Skip very recent sessions (less than 1 hour old)
		if time.Since(sessionDir.LastModified) < time.Hour {
			continue
		}

		// Get size of this session
		size, err := cs.fileManager.GetDirectorySize(sessionDir.Path)
		if err != nil {
			cs.logger.Warn("failed to get directory size", "path", sessionDir.Path, "error", err)
			continue
		}

		// Remove the directory
		if err := os.RemoveAll(sessionDir.Path); err != nil {
			cs.logger.Error("failed to remove directory", "path", sessionDir.Path, "error", err)
			continue
		}

		freedSize += size
		removedCount++
		cs.logger.Info("removed session for emergency cleanup",
			"path", sessionDir.Path,
			"size", plugins.FormatBytes(size),
			"age", time.Since(sessionDir.LastModified))
	}

	cs.logger.Info("emergency cleanup completed",
		"removed_count", removedCount,
		"freed_size", plugins.FormatBytes(freedSize))
}

// cleanupOrphanedDirectories removes directories without database records
func (cs *CleanupService) cleanupOrphanedDirectories() (int, error) {
	cs.logger.Info("checking for orphaned directories")
	entries, err := os.ReadDir(cs.baseDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read base directory: %w", err)
	}

	orphanCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Extract session ID from directory name
		// Format: container_provider_sessionid
		dirName := entry.Name()
		sessionID := cs.extractSessionID(dirName)
		if sessionID == "" {
			continue
		}

		// Check if session exists in database
		_, err := cs.store.GetSession(sessionID)
		if err != nil {
			// Session not found, this is an orphan
			dirPath := filepath.Join(cs.baseDir, dirName)
			cs.logger.Info("found orphaned directory", "dir", dirName, "session_id", sessionID, "error", err.Error())

			// Check age before removing
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Only remove if older than 30 minutes (reduced from 1 hour for faster cleanup)
			if time.Since(info.ModTime()) > 30*time.Minute {
				if err := os.RemoveAll(dirPath); err != nil {
					cs.logger.Error("failed to remove orphaned directory", "path", dirPath, "error", err)
				} else {
					orphanCount++
					cs.logger.Info("removed orphaned directory", "path", dirPath, "age", time.Since(info.ModTime()))
				}
			}
		}
	}

	return orphanCount, nil
}

// extractSessionID extracts session ID from directory name
func (cs *CleanupService) extractSessionID(dirName string) string {
	// Directory format: container_provider_sessionid
	// Example: dash_ffmpeg_software_1234567890-abcd-...
	// The session ID is the UUID part at the end

	// Common provider names that contain underscores
	providers := []string{
		"ffmpeg_software",
		"ffmpeg_nvidia",
		"ffmpeg_pipeline",
		// Add more providers here as needed
	}

	baseName := filepath.Base(dirName)

	// Try to match against known providers
	for _, provider := range providers {
		// Pattern: container_provider_sessionid
		for _, container := range []string{"dash", "hls", "mp4"} {
			prefix := fmt.Sprintf("%s_%s_", container, provider)
			if strings.HasPrefix(baseName, prefix) {
				// Extract session ID after the prefix
				return strings.TrimPrefix(baseName, prefix)
			}
		}
	}

	// Fallback: assume it's container_singleprovider_sessionid
	parts := strings.Split(baseName, "_")
	if len(parts) >= 3 {
		return parts[len(parts)-1]
	}

	return ""
}

// GetCleanupStats returns cleanup statistics
func (cs *CleanupService) GetCleanupStats() (*CleanupStats, error) {
	stats := &CleanupStats{
		Timestamp: time.Now(),
	}

	// Get total size and count
	entries, err := os.ReadDir(cs.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var oldestTime time.Time
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		stats.TotalSessions++

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Track oldest session
		if oldestTime.IsZero() || info.ModTime().Before(oldestTime) {
			oldestTime = info.ModTime()
		}

		// Get directory size
		dirPath := filepath.Join(cs.baseDir, entry.Name())
		size, err := cs.fileManager.GetDirectorySize(dirPath)
		if err != nil {
			cs.logger.Warn("failed to get directory size", "path", dirPath, "error", err)
			continue
		}
		stats.TotalSize += size

		// Count by status (would need to query DB for accurate status)
		if time.Since(info.ModTime()) < 5*time.Minute {
			stats.ActiveSessions++
		}
	}

	if !oldestTime.IsZero() {
		stats.OldestSession = time.Since(oldestTime)
	}

	// Set policy info
	stats.RetentionHours = cs.config.RetentionHours
	stats.MaxSizeGB = cs.config.MaxTotalSizeGB

	return stats, nil
}

// CleanupSession removes a specific session's files
func (cs *CleanupService) CleanupSession(sessionID string) error {
	// Get session from store to find directory
	session, err := cs.store.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Remove directory if it exists
	if session.DirectoryPath != "" && session.DirectoryPath != "/" {
		if err := os.RemoveAll(session.DirectoryPath); err != nil {
			return fmt.Errorf("failed to remove directory: %w", err)
		}
		cs.logger.Info("cleaned up session directory", "session_id", sessionID, "path", session.DirectoryPath)
	}

	return nil
}

// CleanupOrphanedProcesses kills orphaned FFmpeg processes that are no longer tracked
func (cs *CleanupService) CleanupOrphanedProcesses() {
	cs.logger.Debug("checking for orphaned FFmpeg processes")

	// First, use the process registry to check for long-running processes
	registryKilled := cs.processRegistry.CleanupOrphaned()
	if registryKilled > 0 {
		cs.logger.Info("killed long-running processes from registry", "count", registryKilled)
	}

	// Then check for any FFmpeg processes not in the registry
	processes, err := cs.getFFmpegProcesses()
	if err != nil {
		cs.logger.Error("failed to get FFmpeg processes", "error", err)
		return
	}

	// Get all registered processes
	registeredProcesses := cs.processRegistry.GetAllProcesses()

	killedCount := 0
	for _, proc := range processes {
		// Check if this process is in the registry
		if _, registered := registeredProcesses[proc.PID]; registered {
			// Process is registered, skip it
			continue
		}

		// Process is not registered, check if it's orphaned
		sessionID, isOrphaned := cs.isProcessOrphaned(proc)
		if isOrphaned {
			cs.logger.Warn("found orphaned FFmpeg process", "pid", proc.PID, "session_id", sessionID, "cmd", proc.CmdLine)

			// Kill the orphaned process using centralized function
			if err := KillProcessGroup(proc.PID, cs.logger); err != nil {
				cs.logger.Error("failed to kill orphaned process", "pid", proc.PID, "error", err)
			} else {
				killedCount++
				cs.logger.Info("killed orphaned FFmpeg process", "pid", proc.PID)

				// If we have a session ID, mark it as failed in the database
				if sessionID != "" {
					if err := cs.store.UpdateSessionStatus(sessionID, "failed", `{"error": "Process was orphaned and killed"}`); err != nil {
						cs.logger.Error("failed to update orphaned session status", "session_id", sessionID, "error", err)
					}
				}
			}
		}
	}

	if killedCount > 0 {
		cs.logger.Info("orphaned process cleanup completed", "killed_count", killedCount)
	}
}

// ForceCleanupSession immediately cleans up a session's files and processes
func (cs *CleanupService) ForceCleanupSession(sessionID string) error {
	cs.logger.Info("force cleaning session", "session_id", sessionID)

	// First check if there's a registered process for this session
	if pid, ok := cs.processRegistry.GetProcessBySession(sessionID); ok {
		cs.logger.Info("killing registered process for session", "session_id", sessionID, "pid", pid)
		if err := KillProcessGroup(pid, cs.logger); err != nil {
			cs.logger.Error("failed to kill registered process", "pid", pid, "error", err)
		}
		cs.processRegistry.Unregister(pid)
	}

	// Clean up files
	if err := cs.CleanupSession(sessionID); err != nil {
		cs.logger.Warn("failed to cleanup session files", "session_id", sessionID, "error", err)
	}

	// Also check for any unregistered processes (fallback)
	processes, err := cs.getFFmpegProcesses()
	if err != nil {
		return fmt.Errorf("failed to get processes: %w", err)
	}

	for _, proc := range processes {
		// Check if this process is for this session
		if strings.Contains(proc.CmdLine, sessionID) {
			cs.logger.Info("killing unregistered process for session", "session_id", sessionID, "pid", proc.PID)
			if err := KillProcessGroup(proc.PID, cs.logger); err != nil {
				cs.logger.Error("failed to kill session process", "pid", proc.PID, "error", err)
			}
		}
	}

	return nil
}

// Process represents a running process
type Process struct {
	PID     int
	CmdLine string
}

// getFFmpegProcesses returns all running FFmpeg processes
func (cs *CleanupService) getFFmpegProcesses() ([]Process, error) {
	var processes []Process

	// Use ps command to find FFmpeg processes
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ps command: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Look for lines containing "ffmpeg"
		if strings.Contains(line, "ffmpeg") && !strings.Contains(line, "grep") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				pid, err := strconv.Atoi(fields[1])
				if err != nil {
					continue
				}

				// Reconstruct command line (everything from field 10 onwards)
				cmdLine := ""
				if len(fields) >= 11 {
					cmdLine = strings.Join(fields[10:], " ")
				}

				processes = append(processes, Process{
					PID:     pid,
					CmdLine: cmdLine,
				})
			}
		}
	}

	return processes, nil
}

// isProcessOrphaned checks if a process is orphaned and returns the session ID if found
func (cs *CleanupService) isProcessOrphaned(proc Process) (string, bool) {
	// Extract potential session ID from command line
	for _, part := range strings.Fields(proc.CmdLine) {
		// Look for output paths that contain session IDs
		if strings.Contains(part, cs.baseDir) {
			// Extract session ID from path
			dirName := filepath.Base(filepath.Dir(part))
			sessionID := cs.extractSessionID(dirName)
			if sessionID != "" {
				// Check if this session exists in the database
				session, err := cs.store.GetSession(sessionID)
				if err != nil {
					// Session not found, this is an orphan
					return sessionID, true
				}
				// Also check if session is in a stuck state
				if session.Status == "running" || session.Status == "queued" {
					// Check if it's been too long without update
					if time.Since(session.UpdatedAt) > 30*time.Minute {
						return sessionID, true
					}
				}
				return sessionID, false
			}
		}
	}

	// If we can't determine the session, assume it's not orphaned to be safe
	return "", false
}

// cleanupNoProgressSessions finds and kills sessions that have been running with 0% progress
func (cs *CleanupService) cleanupNoProgressSessions() (int, error) {
	// Find running sessions
	var runningSessions []*database.TranscodeSession
	if err := cs.store.db.Where("status = ?", "running").Find(&runningSessions).Error; err != nil {
		return 0, fmt.Errorf("failed to find running sessions: %w", err)
	}

	killedCount := 0
	for _, session := range runningSessions {
		// Parse progress data
		progressData := make(map[string]interface{})
		if session.Progress != "" {
			if err := json.Unmarshal([]byte(session.Progress), &progressData); err != nil {
				cs.logger.Warn("failed to parse progress data", "session_id", session.ID, "error", err)
				continue
			}
		}

		// Check progress percentage
		progressPercent, _ := progressData["percent_complete"].(float64)
		timeElapsed, _ := progressData["time_elapsed"].(float64)
		bytesWritten, _ := progressData["bytes_written"].(float64)

		// If process has been running for more than 10 minutes with 0% progress, it's stuck
		// Convert nanoseconds to seconds
		timeElapsedSeconds := timeElapsed / 1e9

		// Check if the session is truly stuck:
		// 1. For ABR transcoding, FFmpeg may report 0% progress while actively writing segments
		// 2. Check if bytes are being written as an alternative indicator of activity
		// 3. Also check if the directory is being updated recently
		isStuck := false

		if timeElapsedSeconds > 600 && progressPercent == 0 { // 10 minutes
			// Before considering it stuck, check if files are being written
			if session.DirectoryPath != "" {
				// Check if directory has been modified recently (within last 2 minutes)
				if info, err := os.Stat(session.DirectoryPath); err == nil {
					if time.Since(info.ModTime()) < 2*time.Minute {
						cs.logger.Debug("session shows 0% progress but directory is active",
							"session_id", session.ID,
							"elapsed_seconds", timeElapsedSeconds,
							"dir_mod_time", info.ModTime())
						continue // Skip this session, it's still active
					}
				}

				// Also check for recent file activity inside the directory
				hasRecentActivity := false
				entries, err := os.ReadDir(session.DirectoryPath)
				if err == nil {
					for _, entry := range entries {
						info, err := entry.Info()
						if err == nil && time.Since(info.ModTime()) < 2*time.Minute {
							hasRecentActivity = true
							break
						}
					}
				}

				if hasRecentActivity {
					cs.logger.Debug("session shows 0% progress but has recent file activity",
						"session_id", session.ID,
						"elapsed_seconds", timeElapsedSeconds)
					continue // Skip this session, it's still active
				}
			}

			// Also check if bytes written is increasing (even if progress is 0%)
			if bytesWritten > 0 {
				cs.logger.Debug("session shows 0% progress but is writing bytes",
					"session_id", session.ID,
					"elapsed_seconds", timeElapsedSeconds,
					"bytes_written", bytesWritten)
				continue // Skip this session, it's still active
			}

			// If we get here, the session is truly stuck
			isStuck = true
		}

		if isStuck {
			cs.logger.Warn("found stuck session with no progress",
				"session_id", session.ID,
				"elapsed_seconds", timeElapsedSeconds,
				"progress_percent", progressPercent)

			// Force cleanup this session
			if err := cs.ForceCleanupSession(session.ID); err != nil {
				cs.logger.Error("failed to force cleanup session", "session_id", session.ID, "error", err)
			} else {
				// Mark session as failed
				if err := cs.store.UpdateSessionStatus(session.ID, "failed", `{"error": "Process stuck with no progress"}`); err != nil {
					cs.logger.Error("failed to update session status", "session_id", session.ID, "error", err)
				}
				killedCount++
			}
		}
	}

	return killedCount, nil
}

// CleanupStats contains cleanup statistics
type CleanupStats struct {
	TotalSessions  int
	ActiveSessions int
	TotalSize      int64
	OldestSession  time.Duration
	RetentionHours int
	MaxSizeGB      int64
	Timestamp      time.Time
}
