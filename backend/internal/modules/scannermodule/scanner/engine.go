package scanner

import (
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
	"gorm.io/gorm"
)

// ScanEngine defines the interface for all scanner implementations
type ScanEngine interface {
	// Core scanner methods
	Start(libraryID uint) error
	Resume(libraryID uint) error
	Pause()

	// Status and monitoring
	GetWorkerStats() (active, min, max, queueLen int)
	GetProgress() (progress float64, eta time.Time, rate float64)

	// Configuration
	SetConfig(config *ScanConfig)
	GetConfig() *ScanConfig
}

// ScanContext provides context and dependencies for a scan operation
type ScanContext struct {
	DB           *gorm.DB
	EventBus     events.EventBus
	PluginModule *pluginmodule.PluginModule
	JobID        uint
	LibraryID    uint
	Config       *ScanConfig
}

// ScanResult represents the outcome of scanning a single file
type ScanResult struct {
	MediaFile        *database.MediaFile
	Metadata         interface{} // Generic metadata that can be MusicMetadata, VideoMetadata, etc.
	MetadataType     string      // Type identifier: "music", "video", "image", etc.
	Path             string
	Error            error
	NeedsPluginHooks bool
	ProcessingTime   time.Duration
	BytesProcessed   int64
}

// ScanStatistics holds metrics about a scan operation
type ScanStatistics struct {
	FilesProcessed int64
	FilesSkipped   int64
	BytesProcessed int64
	ErrorsCount    int64
	StartTime      time.Time
	LastUpdateTime time.Time
	ThroughputFPS  float64
	ThroughputMBPS float64
}

// WorkerPool defines the interface for managing scan workers
type WorkerPool interface {
	Start() error
	Stop()
	Resize(newSize int) error
	GetStats() WorkerPoolStats
}

// WorkerPoolStats represents worker pool statistics
type WorkerPoolStats struct {
	ActiveWorkers   int
	IdleWorkers     int
	TotalWorkers    int
	QueueLength     int
	ProcessingRate  float64
	AverageWaitTime time.Duration
}

// QueueManager manages work distribution and queuing
type QueueManager interface {
	Enqueue(work interface{}) error
	Close()
	Length() int
	IsEmpty() bool
}

// ProgressTracker tracks and estimates scan progress
type ProgressTracker interface {
	SetTotal(files int64, bytes int64)
	Update(filesProcessed int64, bytesProcessed int64)
	GetEstimate() (progress float64, eta time.Time, rate float64)
	GetStatistics() map[string]interface{}
}

// TelemetryCollector handles events, logs, and metrics
type TelemetryCollector interface {
	EmitEvent(event events.Event)
	LogInfo(message string, fields map[string]interface{})
	LogError(message string, err error, fields map[string]interface{})
	RecordMetric(name string, value float64, tags map[string]string)
	GetMetrics() map[string]interface{}
}

// PluginHook defines the interface for plugin integration points
type PluginHook interface {
	OnScanStarted(jobID, libraryID uint, path string) error
	OnScanCompleted(jobID, libraryID uint, stats map[string]interface{}) error
	OnMediaFileScanned(mediaFile *database.MediaFile, metadata interface{}) error
}

// ScanEngineImpl provides the core scanning engine functionality
type ScanEngineImpl struct {
	DB           *gorm.DB
	PluginModule *pluginmodule.PluginModule
	Options      *ManagerOptions // Defined in safeguards.go
}
