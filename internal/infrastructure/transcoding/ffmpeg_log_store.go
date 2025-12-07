package transcoding

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/transcoding/logging"
)

// FFmpegLogStore is re-exported from the logging subpackage for backward compatibility.
type FFmpegLogStore = logging.FFmpegLogStore

// LogWriter is re-exported from the logging subpackage for backward compatibility.
type LogWriter = logging.LogWriter

// LogEntry is re-exported from the logging subpackage for backward compatibility.
type LogEntry = logging.LogEntry

// LogInfo is re-exported from the logging subpackage for backward compatibility.
type LogInfo = logging.LogInfo

// NewFFmpegLogStore creates a new log store with the specified base directory and retention.
// Re-exported from logging subpackage for backward compatibility.
func NewFFmpegLogStore(baseDir string, retentionTime time.Duration, logger *slog.Logger) (*FFmpegLogStore, error) {
	return logging.NewFFmpegLogStore(baseDir, retentionTime, logger)
}

// NewLogWriterForTest creates a LogWriter directly for testing purposes.
// Re-exported from logging subpackage for backward compatibility.
func NewLogWriterForTest(sessionID string, logDir string) (*LogWriter, error) {
	return logging.NewLogWriterForTest(sessionID, logDir)
}

// The following interfaces document the methods available on re-exported types.
// They're defined here for documentation purposes and to ensure API compatibility.

// FFmpegLogStoreInterface documents the FFmpegLogStore API.
type FFmpegLogStoreInterface interface {
	CreateLogWriter(sessionID string, mediaID int64, quality string) (*LogWriter, error)
	GetLogWriter(sessionID string) *LogWriter
	CloseLogWriter(sessionID string) error
	ReadLog(sessionID string, mediaID int64) (string, error)
	ReadLogTail(sessionID string, mediaID int64, lines int) ([]string, error)
	StreamLog(ctx context.Context, sessionID string, mediaID int64, follow bool, output io.Writer) error
	ListLogs(mediaID int64) ([]LogInfo, error)
	GetLogInfo(sessionID string, mediaID int64) (*LogInfo, error)
	CleanupOldLogs() (int, int64, error)
	DeleteLog(sessionID string, mediaID int64) error
	GetActiveSessionIDs() []string
}

// LogWriterInterface documents the LogWriter API.
type LogWriterInterface interface {
	io.Writer
	WriteString(s string) (n int, err error)
	WriteLine(line string) error
	Close() error
	Subscribe() <-chan string
	Unsubscribe(ch <-chan string)
	FilePath() string
	IsClosed() bool
}

// Compile-time interface compliance checks
var (
	_ FFmpegLogStoreInterface = (*FFmpegLogStore)(nil)
	_ LogWriterInterface      = (*LogWriter)(nil)
)
