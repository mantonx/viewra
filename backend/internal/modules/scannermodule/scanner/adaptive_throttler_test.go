package scanner

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptiveThrottler_BasicFunctionality(t *testing.T) {
	config := ThrottleConfig{
		MinWorkers:              1,
		MaxWorkers:              4,
		InitialWorkers:          2,
		TargetCPUPercent:        70.0,
		MaxCPUPercent:           85.0,
		TargetMemoryPercent:     80.0,
		MaxMemoryPercent:        90.0,
		TargetNetworkThroughput: 80.0,
		MaxNetworkThroughput:    100.0,
		DefaultBatchSize:        50,
		MinBatchSize:            10,
		MaxBatchSize:            200,
		DefaultProcessingDelay:  10 * time.Millisecond,
		MaxProcessingDelay:      500 * time.Millisecond,
		AdjustmentInterval:      1 * time.Second, // Faster for testing
		EmergencyBrakeThreshold: 95.0,
		EmergencyBrakeDuration:  5 * time.Second,
	}

	throttler := NewAdaptiveThrottler(config)
	require.NotNil(t, throttler)

	// Test initial state
	limits := throttler.GetCurrentLimits()
	assert.Equal(t, 2, limits.WorkerCount)
	assert.Equal(t, 50, limits.BatchSize)
	assert.True(t, limits.Enabled)

	// Test basic throttling check
	shouldThrottle, delay := throttler.ShouldThrottle()
	// The initial delay should match the default processing delay
	assert.Equal(t, 10*time.Millisecond, delay)
	// Check if throttling is considered active (delay > 0 means throttling)
	expectedThrottle := delay > 0
	assert.Equal(t, expectedThrottle, shouldThrottle, "Throttling state should match delay value")

	t.Logf("Initial throttling state: shouldThrottle=%v, delay=%v", shouldThrottle, delay)

	throttler.Stop()
}

func TestAdaptiveThrottler_EmergencyBrake(t *testing.T) {
	config := ThrottleConfig{
		MinWorkers:              1,
		MaxWorkers:              4,
		InitialWorkers:          2,
		EmergencyBrakeThreshold: 50.0, // Low threshold for testing
		EmergencyBrakeDuration:  1 * time.Second,
		DefaultProcessingDelay:  5 * time.Millisecond, // Small default delay
	}

	throttler := NewAdaptiveThrottler(config)
	defer throttler.Stop()

	// Simulate high system load
	highLoadMetrics := SystemMetrics{
		CPUPercent:    95.0, // Above emergency threshold
		MemoryPercent: 95.0,
		IOWaitPercent: 40.0,
		LoadAverage:   float64(runtime.NumCPU()) * 3.0,
		TimestampUTC:  time.Now().UTC(),
	}

	// Trigger emergency brake
	throttler.triggerEmergencyBrake("test_high_load", highLoadMetrics)

	// Verify emergency brake is active
	shouldThrottle, delay := throttler.ShouldThrottle()
	assert.True(t, shouldThrottle)
	assert.Equal(t, 1*time.Second, delay, "Emergency brake should use emergency duration")

	// Verify worker count is reduced to minimum
	limits := throttler.GetCurrentLimits()
	assert.Equal(t, 1, limits.WorkerCount)

	// Simulate improved conditions
	normalMetrics := SystemMetrics{
		CPUPercent:    30.0,
		MemoryPercent: 40.0,
		IOWaitPercent: 5.0,
		LoadAverage:   1.0,
		TimestampUTC:  time.Now().UTC(),
	}

	// Release emergency brake
	throttler.releaseEmergencyBrake(normalMetrics)

	// Verify emergency brake is released (but default delay may still apply)
	shouldThrottle, delay = throttler.ShouldThrottle()
	assert.NotEqual(t, 1*time.Second, delay, "Should not use emergency brake duration")

	// Check that we're back to normal default processing delay
	expectedDelay := config.DefaultProcessingDelay
	expectedThrottle := expectedDelay > 0
	assert.Equal(t, expectedThrottle, shouldThrottle, "Throttling should match default delay setting")
	assert.Equal(t, expectedDelay, delay, "Should use default processing delay")
}

func TestAdaptiveThrottler_ContainerDetection(t *testing.T) {
	throttler := NewAdaptiveThrottler(ThrottleConfig{})
	defer throttler.Stop()

	// Test container detection logic
	throttler.detectContainerEnvironment()

	// The test environment may or may not be containerized
	// Just verify the detection doesn't crash and sets reasonable values
	if throttler.isContainerized {
		t.Logf("Container detected: cgroup v%d", throttler.cgroupVersion)
		assert.True(t, throttler.cgroupVersion == 1 || throttler.cgroupVersion == 2)
		assert.NotEmpty(t, throttler.cgroupBasePath)

		if throttler.containerLimits.MemoryLimitBytes > 0 {
			t.Logf("Memory limit: %.2f GB",
				float64(throttler.containerLimits.MemoryLimitBytes)/(1024*1024*1024))
		}
		if throttler.containerLimits.MaxCPUPercent > 0 {
			t.Logf("CPU limit: %.1f%%", throttler.containerLimits.MaxCPUPercent)
		}
	} else {
		t.Log("No container environment detected (running on host)")
		assert.False(t, throttler.isContainerized)
	}
}

func TestAdaptiveThrottler_MetricsGathering(t *testing.T) {
	throttler := NewAdaptiveThrottler(ThrottleConfig{})
	defer throttler.Stop()

	// Test direct metrics gathering (bypassing async loop)
	metrics := throttler.gatherSystemMetrics()

	// Validate basic metrics structure
	assert.GreaterOrEqual(t, metrics.CPUPercent, 0.0)
	assert.LessOrEqual(t, metrics.CPUPercent, 100.0)
	assert.GreaterOrEqual(t, metrics.MemoryPercent, 0.0)
	assert.LessOrEqual(t, metrics.MemoryPercent, 100.0)
	assert.GreaterOrEqual(t, metrics.LoadAverage, 0.0)
	assert.False(t, metrics.TimestampUTC.IsZero(), "Timestamp should be set")

	t.Logf("Direct metrics: CPU=%.1f%%, Memory=%.1f%%, Load=%.2f",
		metrics.CPUPercent, metrics.MemoryPercent, metrics.LoadAverage)

	// Verify timestamp is recent
	timeDiff := time.Since(metrics.TimestampUTC)
	assert.Less(t, timeDiff, 2*time.Second, "Metrics should be recent")
}

func TestAdaptiveThrottler_WorkerCountCalculation(t *testing.T) {
	throttler := NewAdaptiveThrottler(ThrottleConfig{
		MinWorkers:              1,
		MaxWorkers:              8,
		TargetCPUPercent:        70.0,
		MaxCPUPercent:           85.0,
		TargetMemoryPercent:     80.0,
		MaxMemoryPercent:        90.0,
		TargetNetworkThroughput: 80.0,
	})
	defer throttler.Stop()

	// Set initial worker count
	throttler.currentLimits.WorkerCount = 4

	// Test scale-up scenario (low resource usage)
	lowUsageMetrics := SystemMetrics{
		CPUPercent:      30.0, // Well below target
		MemoryPercent:   40.0, // Well below target
		IOWaitPercent:   5.0,
		NetworkUtilMBps: 20.0, // Well below target
	}

	optimalWorkers := throttler.calculateOptimalWorkerCount(lowUsageMetrics)
	assert.Greater(t, optimalWorkers, 4, "Should scale up with low resource usage")
	assert.LessOrEqual(t, optimalWorkers, 8, "Should not exceed max workers")

	// Test scale-down scenario (high resource usage)
	highUsageMetrics := SystemMetrics{
		CPUPercent:      90.0, // Above max threshold
		MemoryPercent:   95.0, // Above max threshold
		IOWaitPercent:   35.0, // High I/O wait
		NetworkUtilMBps: 95.0, // Near max network
	}

	optimalWorkers = throttler.calculateOptimalWorkerCount(highUsageMetrics)
	assert.Less(t, optimalWorkers, 4, "Should scale down with high resource usage")
	assert.GreaterOrEqual(t, optimalWorkers, 1, "Should not go below min workers")
}

func TestAdaptiveThrottler_NFSOptimization(t *testing.T) {
	throttler := NewAdaptiveThrottler(ThrottleConfig{
		TargetNetworkThroughput: 80.0, // 80 MB/s for 1Gbps NFS
		MaxNetworkThroughput:    100.0,
		TargetIOWaitPercent:     20.0,
		MaxIOWaitPercent:        30.0,
	})
	defer throttler.Stop()

	// Test network-heavy scenario (typical for NFS)
	nfsMetrics := SystemMetrics{
		CPUPercent:      40.0, // Moderate CPU
		MemoryPercent:   50.0, // Moderate memory
		IOWaitPercent:   25.0, // High I/O wait (typical for network storage)
		NetworkUtilMBps: 85.0, // High network usage
	}

	delay := throttler.calculateOptimalDelay(nfsMetrics)
	assert.Greater(t, delay, 10*time.Millisecond, "Should increase delay for network pressure")

	bandwidth := throttler.calculateOptimalNetworkBandwidth(nfsMetrics)
	assert.Less(t, bandwidth, 100.0, "Should limit network bandwidth under pressure")

	ioThrottle := throttler.calculateOptimalIOThrottle(nfsMetrics)
	assert.Less(t, ioThrottle, 100.0, "Should throttle I/O with high wait times")
}

func TestAdaptiveThrottler_ConfigurationUpdates(t *testing.T) {
	initialConfig := ThrottleConfig{
		MinWorkers:       1,
		MaxWorkers:       4,
		TargetCPUPercent: 70.0,
	}

	throttler := NewAdaptiveThrottler(initialConfig)
	defer throttler.Stop()

	// Test configuration retrieval
	config := throttler.GetThrottleConfig()
	assert.Equal(t, 70.0, config.TargetCPUPercent)

	// Test configuration update
	newConfig := config
	newConfig.TargetCPUPercent = 80.0
	newConfig.MaxWorkers = 8

	throttler.SetThrottleConfig(newConfig)

	// Verify update
	updatedConfig := throttler.GetThrottleConfig()
	assert.Equal(t, 80.0, updatedConfig.TargetCPUPercent)
	assert.Equal(t, 8, updatedConfig.MaxWorkers)
}

// Benchmark test for throttling overhead
func BenchmarkAdaptiveThrottler_ApplyDelay(b *testing.B) {
	throttler := NewAdaptiveThrottler(ThrottleConfig{
		DefaultProcessingDelay: 1 * time.Millisecond,
	})
	defer throttler.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		throttler.ApplyDelay()
	}
}

func BenchmarkAdaptiveThrottler_ShouldThrottle(b *testing.B) {
	throttler := NewAdaptiveThrottler(ThrottleConfig{})
	defer throttler.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		throttler.ShouldThrottle()
	}
}
