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

func TestScanConfig_WithDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    ScanConfig
		validate func(t *testing.T, result ScanConfig)
	}{
		{
			name:  "empty config gets all defaults",
			input: ScanConfig{},
			validate: func(t *testing.T, result ScanConfig) {
				defaults := DefaultScanConfig()
				if result.Timeout != defaults.Timeout {
					t.Errorf("Timeout = %v, want %v", result.Timeout, defaults.Timeout)
				}
				if result.CheckpointBatchSize != defaults.CheckpointBatchSize {
					t.Errorf("CheckpointBatchSize = %v, want %v", result.CheckpointBatchSize, defaults.CheckpointBatchSize)
				}
				if result.MaxRetries != defaults.MaxRetries {
					t.Errorf("MaxRetries = %v, want %v", result.MaxRetries, defaults.MaxRetries)
				}
				if result.WorkerTimeout != defaults.WorkerTimeout {
					t.Errorf("WorkerTimeout = %v, want %v", result.WorkerTimeout, defaults.WorkerTimeout)
				}
				if result.BaseFileTimeout != defaults.BaseFileTimeout {
					t.Errorf("BaseFileTimeout = %v, want %v", result.BaseFileTimeout, defaults.BaseFileTimeout)
				}
				if result.RemoteStorageTimeout != defaults.RemoteStorageTimeout {
					t.Errorf("RemoteStorageTimeout = %v, want %v", result.RemoteStorageTimeout, defaults.RemoteStorageTimeout)
				}
				if result.MaxExtraTimeout != defaults.MaxExtraTimeout {
					t.Errorf("MaxExtraTimeout = %v, want %v", result.MaxExtraTimeout, defaults.MaxExtraTimeout)
				}
				if result.ProgressUpdateTick != defaults.ProgressUpdateTick {
					t.Errorf("ProgressUpdateTick = %v, want %v", result.ProgressUpdateTick, defaults.ProgressUpdateTick)
				}
				if result.BatchWriteTimeout != defaults.BatchWriteTimeout {
					t.Errorf("BatchWriteTimeout = %v, want %v", result.BatchWriteTimeout, defaults.BatchWriteTimeout)
				}
				if result.DiscoveryBufferSize != defaults.DiscoveryBufferSize {
					t.Errorf("DiscoveryBufferSize = %v, want %v", result.DiscoveryBufferSize, defaults.DiscoveryBufferSize)
				}
				if result.CheckpointBufferSize != defaults.CheckpointBufferSize {
					t.Errorf("CheckpointBufferSize = %v, want %v", result.CheckpointBufferSize, defaults.CheckpointBufferSize)
				}
				if result.HashProgressLogEvery != defaults.HashProgressLogEvery {
					t.Errorf("HashProgressLogEvery = %v, want %v", result.HashProgressLogEvery, defaults.HashProgressLogEvery)
				}
				if result.DiscoveryLogEvery != defaults.DiscoveryLogEvery {
					t.Errorf("DiscoveryLogEvery = %v, want %v", result.DiscoveryLogEvery, defaults.DiscoveryLogEvery)
				}
			},
		},
		{
			name: "explicit values are preserved",
			input: ScanConfig{
				Timeout:             1 * time.Hour,
				CheckpointBatchSize: 100,
				MaxRetries:          5,
			},
			validate: func(t *testing.T, result ScanConfig) {
				if result.Timeout != 1*time.Hour {
					t.Errorf("Timeout = %v, want %v", result.Timeout, 1*time.Hour)
				}
				if result.CheckpointBatchSize != 100 {
					t.Errorf("CheckpointBatchSize = %v, want 100", result.CheckpointBatchSize)
				}
				if result.MaxRetries != 5 {
					t.Errorf("MaxRetries = %v, want 5", result.MaxRetries)
				}
				// Other fields should get defaults
				defaults := DefaultScanConfig()
				if result.WorkerTimeout != defaults.WorkerTimeout {
					t.Errorf("WorkerTimeout = %v, want %v", result.WorkerTimeout, defaults.WorkerTimeout)
				}
			},
		},
		{
			name: "all timeout fields explicitly set",
			input: ScanConfig{
				Timeout:              48 * time.Hour,
				WorkerTimeout:        10 * time.Minute,
				BaseFileTimeout:      60 * time.Second,
				RemoteStorageTimeout: 120 * time.Second,
				MaxExtraTimeout:      240 * time.Second,
				ProgressUpdateTick:   5 * time.Second,
				BatchWriteTimeout:    1 * time.Second,
			},
			validate: func(t *testing.T, result ScanConfig) {
				if result.Timeout != 48*time.Hour {
					t.Errorf("Timeout = %v, want %v", result.Timeout, 48*time.Hour)
				}
				if result.WorkerTimeout != 10*time.Minute {
					t.Errorf("WorkerTimeout = %v, want %v", result.WorkerTimeout, 10*time.Minute)
				}
				if result.BaseFileTimeout != 60*time.Second {
					t.Errorf("BaseFileTimeout = %v, want %v", result.BaseFileTimeout, 60*time.Second)
				}
				if result.RemoteStorageTimeout != 120*time.Second {
					t.Errorf("RemoteStorageTimeout = %v, want %v", result.RemoteStorageTimeout, 120*time.Second)
				}
				if result.MaxExtraTimeout != 240*time.Second {
					t.Errorf("MaxExtraTimeout = %v, want %v", result.MaxExtraTimeout, 240*time.Second)
				}
				if result.ProgressUpdateTick != 5*time.Second {
					t.Errorf("ProgressUpdateTick = %v, want %v", result.ProgressUpdateTick, 5*time.Second)
				}
				if result.BatchWriteTimeout != 1*time.Second {
					t.Errorf("BatchWriteTimeout = %v, want %v", result.BatchWriteTimeout, 1*time.Second)
				}
			},
		},
		{
			name: "all buffer size fields explicitly set",
			input: ScanConfig{
				DiscoveryBufferSize:  200000,
				CheckpointBufferSize: 200,
				HashProgressLogEvery: 10000,
				DiscoveryLogEvery:    2000,
			},
			validate: func(t *testing.T, result ScanConfig) {
				if result.DiscoveryBufferSize != 200000 {
					t.Errorf("DiscoveryBufferSize = %v, want 200000", result.DiscoveryBufferSize)
				}
				if result.CheckpointBufferSize != 200 {
					t.Errorf("CheckpointBufferSize = %v, want 200", result.CheckpointBufferSize)
				}
				if result.HashProgressLogEvery != 10000 {
					t.Errorf("HashProgressLogEvery = %v, want 10000", result.HashProgressLogEvery)
				}
				if result.DiscoveryLogEvery != 2000 {
					t.Errorf("DiscoveryLogEvery = %v, want 2000", result.DiscoveryLogEvery)
				}
			},
		},
		{
			name: "parallel walkers and progress interval preserved as zero",
			input: ScanConfig{
				ParallelWalkers:  0, // Explicit zero should NOT be replaced (sequential)
				ProgressInterval: 0, // Explicit zero should NOT be replaced (disabled)
			},
			validate: func(t *testing.T, result ScanConfig) {
				// These fields don't have defaults applied when zero
				// because zero is a valid explicit value (disabled/sequential)
				if result.ParallelWalkers != 0 {
					t.Errorf("ParallelWalkers = %v, want 0", result.ParallelWalkers)
				}
				if result.ProgressInterval != 0 {
					t.Errorf("ProgressInterval = %v, want 0", result.ProgressInterval)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.WithDefaults()
			tt.validate(t, result)
		})
	}
}

func TestScanConfig_WithDefaults_doesNotMutateOriginal(t *testing.T) {
	original := ScanConfig{
		Timeout: 1 * time.Hour,
	}

	result := original.WithDefaults()

	// Original should not be mutated
	if original.CheckpointBatchSize != 0 {
		t.Error("original.CheckpointBatchSize was mutated")
	}

	// Result should have defaults applied
	if result.CheckpointBatchSize != 50 {
		t.Errorf("result.CheckpointBatchSize = %v, want 50", result.CheckpointBatchSize)
	}
}
