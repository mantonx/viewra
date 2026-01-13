package transcode

import (
	"testing"
	"time"
)

func TestDefaultQueueConfig(t *testing.T) {
	config := DefaultQueueConfig()

	if config == nil {
		t.Fatal("DefaultQueueConfig returned nil")
	}
	if config.WorkerCount != 2 {
		t.Errorf("WorkerCount = %d, want 2", config.WorkerCount)
	}
	if config.PollInterval != 2*time.Second {
		t.Errorf("PollInterval = %v, want 2s", config.PollInterval)
	}
	if config.IdleTimeout != 5*time.Minute {
		t.Errorf("IdleTimeout = %v, want 5m", config.IdleTimeout)
	}
	if config.OutputBaseDir == "" {
		t.Error("OutputBaseDir should not be empty")
	}
}

func TestQueueConfigCustomValues(t *testing.T) {
	config := &QueueConfig{
		WorkerCount:   4,
		PollInterval:  5 * time.Second,
		IdleTimeout:   10 * time.Minute,
		OutputBaseDir: "/custom/output",
	}

	if config.WorkerCount != 4 {
		t.Errorf("WorkerCount = %d, want 4", config.WorkerCount)
	}
	if config.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want 5s", config.PollInterval)
	}
	if config.IdleTimeout != 10*time.Minute {
		t.Errorf("IdleTimeout = %v, want 10m", config.IdleTimeout)
	}
	if config.OutputBaseDir != "/custom/output" {
		t.Errorf("OutputBaseDir = %q, want %q", config.OutputBaseDir, "/custom/output")
	}
}

func TestQueueStatsFields(t *testing.T) {
	stats := QueueStats{
		QueuedJobs:     10,
		ProcessingJobs: 2,
		CompletedJobs:  100,
		FailedJobs:     5,
		WorkerCount:    2,
	}

	if stats.QueuedJobs != 10 {
		t.Errorf("QueuedJobs = %d, want 10", stats.QueuedJobs)
	}
	if stats.ProcessingJobs != 2 {
		t.Errorf("ProcessingJobs = %d, want 2", stats.ProcessingJobs)
	}
	if stats.CompletedJobs != 100 {
		t.Errorf("CompletedJobs = %d, want 100", stats.CompletedJobs)
	}
	if stats.FailedJobs != 5 {
		t.Errorf("FailedJobs = %d, want 5", stats.FailedJobs)
	}
	if stats.WorkerCount != 2 {
		t.Errorf("WorkerCount = %d, want 2", stats.WorkerCount)
	}
}

func TestNewQueue(t *testing.T) {
	repo := NewMockRepository()

	t.Run("with nil config uses defaults", func(t *testing.T) {
		q := NewQueue(nil, repo, nil, nil)

		if q == nil {
			t.Fatal("NewQueue returned nil")
		}
		if q.config.WorkerCount != 2 {
			t.Errorf("WorkerCount = %d, want 2 (default)", q.config.WorkerCount)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &QueueConfig{
			WorkerCount:   4,
			PollInterval:  5 * time.Second,
			IdleTimeout:   10 * time.Minute,
			OutputBaseDir: "/custom",
		}

		q := NewQueue(config, repo, nil, nil)

		if q == nil {
			t.Fatal("NewQueue returned nil")
		}
		if q.config.WorkerCount != 4 {
			t.Errorf("WorkerCount = %d, want 4", q.config.WorkerCount)
		}
	})

	t.Run("initializes maps", func(t *testing.T) {
		q := NewQueue(nil, repo, nil, nil)

		if q.activeJobs == nil {
			t.Error("activeJobs map should be initialized")
		}
		if q.lastAccessTime == nil {
			t.Error("lastAccessTime map should be initialized")
		}
	})
}

func TestQueueRecordAccess(t *testing.T) {
	repo := NewMockRepository()
	q := NewQueue(nil, repo, nil, nil)

	mediaID := int64(123)
	quality := "720p"

	// Record access
	q.RecordAccess(mediaID, quality)

	// Verify it was recorded
	key := "123:720p"
	q.lastAccessTimeMu.RLock()
	lastAccess, exists := q.lastAccessTime[key]
	q.lastAccessTimeMu.RUnlock()

	if !exists {
		t.Error("RecordAccess should record the access time")
	}

	// Verify time is recent (within last second)
	if time.Since(lastAccess) > time.Second {
		t.Error("RecordAccess should record current time")
	}
}
