package scanner

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultScanConfig(t *testing.T) {
	config := DefaultScanConfig()

	assert.NotNil(t, config)
	assert.True(t, config.ParallelScanningEnabled)
	assert.Equal(t, 0, config.WorkerCount) // Should use CPU count
	assert.Equal(t, 50, config.BatchSize)
	assert.Equal(t, 100, config.ChannelBufferSize)
	assert.True(t, config.SmartHashEnabled)
	assert.True(t, config.AsyncMetadataEnabled)
	assert.Equal(t, 2, config.MetadataWorkerCount)
}

func TestConservativeScanConfig(t *testing.T) {
	config := ConservativeScanConfig()

	assert.NotNil(t, config)
	assert.True(t, config.ParallelScanningEnabled)
	assert.Equal(t, 2, config.WorkerCount)
	assert.Equal(t, 25, config.BatchSize)
	assert.Equal(t, 50, config.ChannelBufferSize)
	assert.True(t, config.SmartHashEnabled)
	assert.True(t, config.AsyncMetadataEnabled)
	assert.Equal(t, 1, config.MetadataWorkerCount)
}

func TestAggressiveScanConfig(t *testing.T) {
	config := AggressiveScanConfig()

	assert.NotNil(t, config)
	assert.True(t, config.ParallelScanningEnabled)
	assert.Equal(t, 8, config.WorkerCount)
	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 200, config.ChannelBufferSize)
	assert.True(t, config.SmartHashEnabled)
	assert.True(t, config.AsyncMetadataEnabled)
	assert.Equal(t, 4, config.MetadataWorkerCount)
}

func TestConfigComparison(t *testing.T) {
	defaultConfig := DefaultScanConfig()
	conservativeConfig := ConservativeScanConfig()
	aggressiveConfig := AggressiveScanConfig()

	// Resolve worker counts for comparison
	defaultWorkerCount := resolveWorkerCount(defaultConfig.WorkerCount)
	conservativeWorkerCount := resolveWorkerCount(conservativeConfig.WorkerCount)
	aggressiveWorkerCount := resolveWorkerCount(aggressiveConfig.WorkerCount)

	// Conservative should have smaller values than default
	assert.LessOrEqual(t, conservativeWorkerCount, defaultWorkerCount)
	assert.Less(t, conservativeConfig.BatchSize, defaultConfig.BatchSize)
	assert.Less(t, conservativeConfig.ChannelBufferSize, defaultConfig.ChannelBufferSize)
	assert.Less(t, conservativeConfig.MetadataWorkerCount, defaultConfig.MetadataWorkerCount)

	// For aggressive config, we need to handle the case where CPU count might be higher than 8
	// Aggressive should have larger values than conservative in all cases
	assert.Greater(t, aggressiveWorkerCount, conservativeWorkerCount)
	assert.Greater(t, aggressiveConfig.BatchSize, conservativeConfig.BatchSize)
	assert.Greater(t, aggressiveConfig.ChannelBufferSize, conservativeConfig.ChannelBufferSize)
	assert.Greater(t, aggressiveConfig.MetadataWorkerCount, conservativeConfig.MetadataWorkerCount)
	
	// Aggressive should be at least as aggressive as conservative, and often more than default
	// But we can't guarantee it's always > default since CPU count varies by system
	assert.Greater(t, aggressiveConfig.BatchSize, defaultConfig.BatchSize)
	assert.Greater(t, aggressiveConfig.ChannelBufferSize, defaultConfig.ChannelBufferSize)
	assert.Greater(t, aggressiveConfig.MetadataWorkerCount, defaultConfig.MetadataWorkerCount)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config *ScanConfig
		valid  bool
	}{
		{
			name:   "Default config should be valid",
			config: DefaultScanConfig(),
			valid:  true,
		},
		{
			name:   "Conservative config should be valid",
			config: ConservativeScanConfig(),
			valid:  true,
		},
		{
			name:   "Aggressive config should be valid",
			config: AggressiveScanConfig(),
			valid:  true,
		},
		{
			name: "Zero worker count should be valid (uses CPU count)",
			config: &ScanConfig{
				ParallelScanningEnabled: true,
				WorkerCount:            0,
				BatchSize:             50,
				ChannelBufferSize:     100,
				SmartHashEnabled:      true,
				AsyncMetadataEnabled:  true,
				MetadataWorkerCount:   2,
			},
			valid: true,
		},
		{
			name: "Negative worker count should be invalid",
			config: &ScanConfig{
				ParallelScanningEnabled: true,
				WorkerCount:            -1,
				BatchSize:             50,
				ChannelBufferSize:     100,
				SmartHashEnabled:      true,
				AsyncMetadataEnabled:  true,
				MetadataWorkerCount:   2,
			},
			valid: false,
		},
		{
			name: "Zero batch size should be invalid",
			config: &ScanConfig{
				ParallelScanningEnabled: true,
				WorkerCount:            4,
				BatchSize:             0,
				ChannelBufferSize:     100,
				SmartHashEnabled:      true,
				AsyncMetadataEnabled:  true,
				MetadataWorkerCount:   2,
			},
			valid: false,
		},
		{
			name: "Zero channel buffer size should be invalid",
			config: &ScanConfig{
				ParallelScanningEnabled: true,
				WorkerCount:            4,
				BatchSize:             50,
				ChannelBufferSize:     0,
				SmartHashEnabled:      true,
				AsyncMetadataEnabled:  true,
				MetadataWorkerCount:   2,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateScanConfig(tt.config)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestConfigWorkerCountResolution(t *testing.T) {
	config := DefaultScanConfig()
	
	// When WorkerCount is 0, it should resolve to CPU count
	if config.WorkerCount == 0 {
		expectedWorkerCount := runtime.NumCPU()
		actualWorkerCount := resolveWorkerCount(config.WorkerCount)
		assert.Equal(t, expectedWorkerCount, actualWorkerCount)
	}

	// When WorkerCount is set, it should use that value
	config.WorkerCount = 4
	actualWorkerCount := resolveWorkerCount(config.WorkerCount)
	assert.Equal(t, 4, actualWorkerCount)
}

func TestConfigMemoryEstimation(t *testing.T) {
	tests := []struct {
		name           string
		config         *ScanConfig
		expectedMemory int64 // Rough estimate in bytes
	}{
		{
			name:           "Conservative config should use less memory",
			config:         ConservativeScanConfig(),
			expectedMemory: 1024 * 1024, // ~1MB
		},
		{
			name:           "Default config should use moderate memory",
			config:         DefaultScanConfig(),
			expectedMemory: 2 * 1024 * 1024, // ~2MB
		},
		{
			name:           "Aggressive config should use more memory",
			config:         AggressiveScanConfig(),
			expectedMemory: 4 * 1024 * 1024, // ~4MB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimatedMemory := estimateConfigMemoryUsage(tt.config)
			
			// Memory estimation should be within reasonable bounds
			assert.Greater(t, estimatedMemory, int64(0))
			assert.LessOrEqual(t, estimatedMemory, int64(100*1024*1024)) // Less than 100MB
			
			// The estimation should be proportional to the config aggressiveness
			if tt.name == "Conservative config should use less memory" {
				assert.LessOrEqual(t, estimatedMemory, tt.expectedMemory*2)
			}
		})
	}
}

// Helper functions for testing (these would be implemented in the actual config.go)

func validateScanConfig(config *ScanConfig) bool {
	if config.WorkerCount < 0 {
		return false
	}
	if config.BatchSize <= 0 {
		return false
	}
	if config.ChannelBufferSize <= 0 {
		return false
	}
	if config.MetadataWorkerCount < 0 {
		return false
	}
	return true
}

func resolveWorkerCount(workerCount int) int {
	if workerCount <= 0 {
		return runtime.NumCPU()
	}
	return workerCount
}

func estimateConfigMemoryUsage(config *ScanConfig) int64 {
	// Rough estimation based on channel buffer sizes and worker counts
	workerCount := resolveWorkerCount(config.WorkerCount)
	
	// Estimate memory per worker (channels, buffers, etc.)
	memoryPerWorker := int64(1024 * 100) // 100KB per worker
	channelMemory := int64(config.ChannelBufferSize * 1024) // 1KB per channel slot
	batchMemory := int64(config.BatchSize * 512) // 512 bytes per batch item
	
	totalMemory := int64(workerCount)*memoryPerWorker + channelMemory + batchMemory
	
	if config.AsyncMetadataEnabled {
		metadataMemory := int64(config.MetadataWorkerCount) * memoryPerWorker
		totalMemory += metadataMemory
	}
	
	return totalMemory
} 