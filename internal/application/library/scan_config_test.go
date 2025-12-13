package library

import (
	"testing"
	"time"
)

func TestDefaultScanConfig(t *testing.T) {
	config := DefaultScanConfig()

	if config.Timeout != 24*time.Hour {
		t.Errorf("Timeout = %v, want %v", config.Timeout, 24*time.Hour)
	}
	if config.CheckpointBatchSize != 50 {
		t.Errorf("CheckpointBatchSize = %v, want 50", config.CheckpointBatchSize)
	}
	if config.BaseFileTimeout != 30*time.Second {
		t.Errorf("BaseFileTimeout = %v, want %v", config.BaseFileTimeout, 30*time.Second)
	}
}
