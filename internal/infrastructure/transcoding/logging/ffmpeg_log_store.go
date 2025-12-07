// Package logging provides FFmpeg log management for transcoding sessions.
//
// This package handles:
//   - Persistent log file storage with automatic cleanup
//   - Real-time log streaming via pub/sub pattern
//   - Log file reading and tailing
//   - Log metadata and error detection
package logging

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FFmpegLogStore manages persistent FFmpeg log files with automatic cleanup.
// Logs are stored in a dedicated directory and can be accessed both during
// transcoding (real-time streaming) and after completion (historical access).
type FFmpegLogStore struct {
	baseDir       string        // Base directory for log files
	retentionTime time.Duration // How long to keep logs
	logger        *slog.Logger

	// Active log writers indexed by session ID
	activeWriters map[string]*LogWriter
	writersMutex  sync.RWMutex
}

// LogWriter wraps a file writer with synchronization for concurrent writes.
// It allows real-time tailing while FFmpeg is writing.
type LogWriter struct {
	sessionID string
	filePath  string
	file      *os.File
	mu        sync.Mutex

	// Subscribers for real-time streaming
	subscribers map[chan string]struct{}
	subMutex    sync.RWMutex

	// Track file position for new subscribers
	closed bool
}

// LogEntry represents a single log line with metadata.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Line      string    `json:"line"`
	IsError   bool      `json:"is_error"` // True if line contains error indicators
}

// LogInfo contains metadata about a log file.
type LogInfo struct {
	SessionID   string    `json:"session_id"`
	MediaID     int64     `json:"media_id"`
	Quality     string    `json:"quality"`
	FilePath    string    `json:"file_path"`
	SizeBytes   int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
	ModifiedAt  time.Time `json:"modified_at"`
	IsActive    bool      `json:"is_active"` // True if still being written
	LineCount   int       `json:"line_count,omitempty"`
	HasErrors   bool      `json:"has_errors,omitempty"`
	ErrorCount  int       `json:"error_count,omitempty"`
}

// NewFFmpegLogStore creates a new log store with the specified base directory and retention.
func NewFFmpegLogStore(baseDir string, retentionTime time.Duration, logger *slog.Logger) (*FFmpegLogStore, error) {
	logDir := filepath.Join(baseDir, "logs", "ffmpeg")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	return &FFmpegLogStore{
		baseDir:       logDir,
		retentionTime: retentionTime,
		logger:        logger,
		activeWriters: make(map[string]*LogWriter),
	}, nil
}

// CreateLogWriter creates a new log writer for a transcode session.
// Returns the writer that should be used to capture FFmpeg output.
func (s *FFmpegLogStore) CreateLogWriter(sessionID string, mediaID int64, quality string) (*LogWriter, error) {
	// Create log file path: {baseDir}/{mediaID}/{sessionID}.log
	mediaDir := filepath.Join(s.baseDir, fmt.Sprintf("%d", mediaID))
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create media log directory: %w", err)
	}

	logPath := filepath.Join(mediaDir, fmt.Sprintf("%s.log", sessionID))
	file, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	// Write header with metadata
	header := fmt.Sprintf("# FFmpeg Log\n# Session: %s\n# Media ID: %d\n# Quality: %s\n# Started: %s\n# ---\n",
		sessionID, mediaID, quality, time.Now().Format(time.RFC3339))
	if _, err := file.WriteString(header); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to write log header: %w", err)
	}

	writer := &LogWriter{
		sessionID:   sessionID,
		filePath:    logPath,
		file:        file,
		subscribers: make(map[chan string]struct{}),
	}

	s.writersMutex.Lock()
	s.activeWriters[sessionID] = writer
	s.writersMutex.Unlock()

	s.logger.Debug("Created FFmpeg log writer",
		"session_id", sessionID,
		"media_id", mediaID,
		"path", logPath)

	return writer, nil
}

// GetLogWriter returns an active log writer by session ID, or nil if not found.
func (s *FFmpegLogStore) GetLogWriter(sessionID string) *LogWriter {
	s.writersMutex.RLock()
	defer s.writersMutex.RUnlock()
	return s.activeWriters[sessionID]
}

// CloseLogWriter closes and removes a log writer from the active set.
func (s *FFmpegLogStore) CloseLogWriter(sessionID string) error {
	s.writersMutex.Lock()
	writer, exists := s.activeWriters[sessionID]
	if exists {
		delete(s.activeWriters, sessionID)
	}
	s.writersMutex.Unlock()

	if !exists {
		return nil
	}

	return writer.Close()
}

// Write writes data to the log file and notifies subscribers.
func (w *LogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return 0, fmt.Errorf("log writer is closed")
	}
	n, err = w.file.Write(p)
	w.mu.Unlock()

	// Notify subscribers of new data
	if n > 0 {
		w.notifySubscribers(string(p[:n]))
	}

	return n, err
}

// WriteString writes a string to the log file.
func (w *LogWriter) WriteString(s string) (n int, err error) {
	return w.Write([]byte(s))
}

// WriteLine writes a line with timestamp to the log file.
func (w *LogWriter) WriteLine(line string) error {
	timestamped := fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05.000"), line)
	_, err := w.WriteString(timestamped)
	return err
}

// Close closes the log file and notifies subscribers.
func (w *LogWriter) Close() error {
	w.mu.Lock()
	w.closed = true
	err := w.file.Close()
	w.mu.Unlock()

	// Close all subscriber channels
	w.subMutex.Lock()
	for ch := range w.subscribers {
		close(ch)
	}
	w.subscribers = make(map[chan string]struct{})
	w.subMutex.Unlock()

	return err
}

// Subscribe returns a channel that receives new log lines in real-time.
// The channel is closed when the log writer is closed.
func (w *LogWriter) Subscribe() <-chan string {
	ch := make(chan string, 100) // Buffer to prevent blocking

	w.subMutex.Lock()
	w.subscribers[ch] = struct{}{}
	w.subMutex.Unlock()

	return ch
}

// Unsubscribe removes a subscriber channel.
func (w *LogWriter) Unsubscribe(ch <-chan string) {
	w.subMutex.Lock()
	defer w.subMutex.Unlock()

	// Find and remove the channel (need to cast back to chan string)
	for subCh := range w.subscribers {
		if subCh == ch {
			delete(w.subscribers, subCh)
			close(subCh)
			break
		}
	}
}

// notifySubscribers sends new data to all active subscribers.
func (w *LogWriter) notifySubscribers(data string) {
	w.subMutex.RLock()
	defer w.subMutex.RUnlock()

	for ch := range w.subscribers {
		select {
		case ch <- data:
		default:
			// Channel full, skip to avoid blocking
		}
	}
}

// FilePath returns the path to the log file.
func (w *LogWriter) FilePath() string {
	return w.filePath
}

// IsClosed returns true if the writer has been closed.
func (w *LogWriter) IsClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

// ReadLog reads the entire log file for a session.
// Works for both active and completed sessions.
func (s *FFmpegLogStore) ReadLog(sessionID string, mediaID int64) (string, error) {
	// Try active writer first
	writer := s.GetLogWriter(sessionID)
	var logPath string

	if writer != nil {
		logPath = writer.FilePath()
	} else {
		// Look for existing log file
		logPath = filepath.Join(s.baseDir, fmt.Sprintf("%d", mediaID), fmt.Sprintf("%s.log", sessionID))
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to read log file: %w", err)
	}

	return string(content), nil
}

// ReadLogTail reads the last N lines of a log file.
func (s *FFmpegLogStore) ReadLogTail(sessionID string, mediaID int64, lines int) ([]string, error) {
	logPath := filepath.Join(s.baseDir, fmt.Sprintf("%d", mediaID), fmt.Sprintf("%s.log", sessionID))

	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Read all lines and return last N
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan log file: %w", err)
	}

	if len(allLines) <= lines {
		return allLines, nil
	}

	return allLines[len(allLines)-lines:], nil
}

// StreamLog streams log content to a writer, optionally following new content.
// If follow is true, continues streaming until context is cancelled.
func (s *FFmpegLogStore) StreamLog(ctx context.Context, sessionID string, mediaID int64, follow bool, output io.Writer) error {
	logPath := filepath.Join(s.baseDir, fmt.Sprintf("%d", mediaID), fmt.Sprintf("%s.log", sessionID))

	// First, read and send existing content
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Copy existing content
	if _, err := io.Copy(output, file); err != nil {
		file.Close()
		return fmt.Errorf("failed to copy existing log content: %w", err)
	}
	file.Close()

	// If not following, we're done
	if !follow {
		return nil
	}

	// Try to subscribe to active writer for real-time updates
	writer := s.GetLogWriter(sessionID)
	if writer == nil {
		// Session not active, nothing more to stream
		return nil
	}

	// Subscribe to new log lines
	ch := writer.Subscribe()
	defer writer.Unsubscribe(ch)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case line, ok := <-ch:
			if !ok {
				// Channel closed, writer finished
				return nil
			}
			if _, err := output.Write([]byte(line)); err != nil {
				return fmt.Errorf("failed to write to output: %w", err)
			}
		}
	}
}

// ListLogs returns information about all logs for a media item.
func (s *FFmpegLogStore) ListLogs(mediaID int64) ([]LogInfo, error) {
	mediaDir := filepath.Join(s.baseDir, fmt.Sprintf("%d", mediaID))

	entries, err := os.ReadDir(mediaDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read log directory: %w", err)
	}

	var logs []LogInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".log")
		info, err := entry.Info()
		if err != nil {
			continue
		}

		logInfo := LogInfo{
			SessionID:  sessionID,
			MediaID:    mediaID,
			FilePath:   filepath.Join(mediaDir, entry.Name()),
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime(),
			IsActive:   s.GetLogWriter(sessionID) != nil,
		}

		// Parse quality from session ID (format: {mediaID}_{quality}_{timestamp})
		parts := strings.Split(sessionID, "_")
		if len(parts) >= 2 {
			logInfo.Quality = parts[1]
		}

		logs = append(logs, logInfo)
	}

	// Sort by modified time, newest first
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].ModifiedAt.After(logs[j].ModifiedAt)
	})

	return logs, nil
}

// GetLogInfo returns detailed information about a specific log file.
func (s *FFmpegLogStore) GetLogInfo(sessionID string, mediaID int64) (*LogInfo, error) {
	logPath := filepath.Join(s.baseDir, fmt.Sprintf("%d", mediaID), fmt.Sprintf("%s.log", sessionID))

	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("log not found")
		}
		return nil, fmt.Errorf("failed to stat log file: %w", err)
	}

	logInfo := &LogInfo{
		SessionID:  sessionID,
		MediaID:    mediaID,
		FilePath:   logPath,
		SizeBytes:  info.Size(),
		ModifiedAt: info.ModTime(),
		IsActive:   s.GetLogWriter(sessionID) != nil,
	}

	// Parse quality from session ID
	parts := strings.Split(sessionID, "_")
	if len(parts) >= 2 {
		logInfo.Quality = parts[1]
	}

	// Count lines and errors
	file, err := os.Open(logPath)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			logInfo.LineCount++
			line := strings.ToLower(scanner.Text())
			if strings.Contains(line, "error") || strings.Contains(line, "failed") {
				logInfo.HasErrors = true
				logInfo.ErrorCount++
			}
		}
	}

	return logInfo, nil
}

// CleanupOldLogs removes logs older than the retention period.
func (s *FFmpegLogStore) CleanupOldLogs() (int, int64, error) {
	cutoff := time.Now().Add(-s.retentionTime)
	var deletedCount int
	var deletedBytes int64

	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".log") {
			return nil
		}

		// Check if file is older than retention period
		if info.ModTime().Before(cutoff) {
			// Don't delete active logs
			sessionID := strings.TrimSuffix(info.Name(), ".log")
			if s.GetLogWriter(sessionID) != nil {
				return nil
			}

			size := info.Size()
			if err := os.Remove(path); err != nil {
				s.logger.Warn("Failed to delete old log file",
					"path", path,
					"error", err)
				return nil
			}

			deletedCount++
			deletedBytes += size
			s.logger.Debug("Deleted old FFmpeg log",
				"path", path,
				"age", time.Since(info.ModTime()))
		}

		return nil
	})

	// Clean up empty media directories
	entries, _ := os.ReadDir(s.baseDir)
	for _, entry := range entries {
		if entry.IsDir() {
			mediaDir := filepath.Join(s.baseDir, entry.Name())
			files, _ := os.ReadDir(mediaDir)
			if len(files) == 0 {
				os.Remove(mediaDir)
			}
		}
	}

	return deletedCount, deletedBytes, err
}

// DeleteLog deletes a specific log file.
func (s *FFmpegLogStore) DeleteLog(sessionID string, mediaID int64) error {
	// Don't delete active logs
	if s.GetLogWriter(sessionID) != nil {
		return fmt.Errorf("cannot delete active log")
	}

	logPath := filepath.Join(s.baseDir, fmt.Sprintf("%d", mediaID), fmt.Sprintf("%s.log", sessionID))
	return os.Remove(logPath)
}

// GetActiveSessionIDs returns the IDs of all sessions with active log writers.
func (s *FFmpegLogStore) GetActiveSessionIDs() []string {
	s.writersMutex.RLock()
	defer s.writersMutex.RUnlock()

	ids := make([]string, 0, len(s.activeWriters))
	for id := range s.activeWriters {
		ids = append(ids, id)
	}
	return ids
}
