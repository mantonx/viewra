package logging

import (
	"os"
	"path/filepath"
)

// NewLogWriterForTest creates a LogWriter directly for testing purposes.
// This bypasses FFmpegLogStore and is intended only for unit tests that need
// a LogWriter without the full store infrastructure.
//
// The caller is responsible for:
//   - Creating the log directory before calling this function
//   - Calling Close() on the returned LogWriter when done
//
// This function is exported for use by tests in parent packages.
func NewLogWriterForTest(sessionID string, logDir string) (*LogWriter, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	filePath := filepath.Join(logDir, sessionID+".log")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	return &LogWriter{
		sessionID:   sessionID,
		filePath:    filePath,
		file:        file,
		subscribers: make(map[chan string]struct{}),
	}, nil
}
