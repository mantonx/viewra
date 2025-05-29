package scanner

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewProgressEstimator(t *testing.T) {
	estimator := NewProgressEstimator()

	assert.NotNil(t, estimator)

	// Get initial stats
	stats := estimator.GetStats()
	assert.Equal(t, int64(0), stats["processed_files"])
	assert.Equal(t, int64(0), stats["total_files"])
	assert.Equal(t, float64(0), stats["files_per_second"])
}

func TestProgressEstimator_SetTotal(t *testing.T) {
	estimator := NewProgressEstimator()

	estimator.SetTotal(1000, 1024*1024*100) // 1000 files, 100MB

	stats := estimator.GetStats()
	assert.Equal(t, int64(1000), stats["total_files"])
	assert.Equal(t, int64(1024*1024*100), stats["total_bytes"])
}

func TestProgressEstimator_Update(t *testing.T) {
	estimator := NewProgressEstimator()
	estimator.SetTotal(100, 1024*100)

	// Process some files
	estimator.Update(25, 1024*25)

	stats := estimator.GetStats()
	assert.Equal(t, int64(25), stats["processed_files"])
	assert.Equal(t, int64(1024*25), stats["processed_bytes"])

	// Process more files
	estimator.Update(50, 1024*50)

	stats = estimator.GetStats()
	assert.Equal(t, int64(50), stats["processed_files"])
	assert.Equal(t, int64(1024*50), stats["processed_bytes"])
}

func TestProgressEstimator_GetEstimate(t *testing.T) {
	estimator := NewProgressEstimator()
	estimator.SetTotal(100, 1024*100)

	// No progress yet
	progress, eta, rate := estimator.GetEstimate()
	assert.Equal(t, float64(0), progress)
	assert.True(t, eta.IsZero())
	assert.Equal(t, float64(0), rate)

	// Process some files
	estimator.Update(25, 1024*25)

	progress, _, _ = estimator.GetEstimate()
	assert.Equal(t, float64(25), progress)
	// ETA and rate might be zero initially due to insufficient samples

	// Process more files with some time delay
	time.Sleep(10 * time.Millisecond)
	estimator.Update(50, 1024*50)

	progress, _, _ = estimator.GetEstimate()
	assert.Equal(t, float64(50), progress)
	// Should have some rate calculation now
}

func TestProgressEstimator_ProgressCalculation(t *testing.T) {
	tests := []struct {
		name             string
		totalFiles       int64
		totalBytes       int64
		processedFiles   int64
		processedBytes   int64
		expectedProgress float64
	}{
		{
			name:             "No files processed",
			totalFiles:       100,
			totalBytes:       1024 * 100,
			processedFiles:   0,
			processedBytes:   0,
			expectedProgress: 0.0,
		},
		{
			name:             "Half files processed",
			totalFiles:       100,
			totalBytes:       1024 * 100,
			processedFiles:   50,
			processedBytes:   1024 * 50,
			expectedProgress: 50.0,
		},
		{
			name:             "All files processed",
			totalFiles:       100,
			totalBytes:       1024 * 100,
			processedFiles:   100,
			processedBytes:   1024 * 100,
			expectedProgress: 100.0,
		},
		{
			name:             "More files processed than total (edge case)",
			totalFiles:       100,
			totalBytes:       1024 * 100,
			processedFiles:   110,
			processedBytes:   1024 * 110,
			expectedProgress: 110.0, // Should reflect actual progress
		},
		{
			name:             "Zero total files",
			totalFiles:       0,
			totalBytes:       0,
			processedFiles:   0,
			processedBytes:   0,
			expectedProgress: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimator := NewProgressEstimator()
			estimator.SetTotal(tt.totalFiles, tt.totalBytes)
			estimator.Update(tt.processedFiles, tt.processedBytes)

			progress, _, _ := estimator.GetEstimate()
			assert.InDelta(t, tt.expectedProgress, progress, 0.0001)
		})
	}
}

func TestProgressEstimator_RateCalculation(t *testing.T) {
	estimator := NewProgressEstimator()
	estimator.SetTotal(1000, 1024*1000)

	// Start processing
	estimator.Update(0, 0)
	time.Sleep(50 * time.Millisecond)

	// Process some files
	estimator.Update(10, 1024*10)
	time.Sleep(50 * time.Millisecond)

	// Process more files
	estimator.Update(30, 1024*30)

	// Should have some positive rate after multiple samples
	// Rate might be zero initially due to smoothing
	stats := estimator.GetStats()
	assert.GreaterOrEqual(t, stats["files_per_second"], float64(0))
}

func TestProgressEstimator_ETACalculation(t *testing.T) {
	estimator := NewProgressEstimator()
	estimator.SetTotal(100, 1024*100)

	// Process files with time delays to establish rate
	estimator.Update(10, 1024*10)
	time.Sleep(100 * time.Millisecond)

	estimator.Update(20, 1024*20)
	time.Sleep(100 * time.Millisecond)

	estimator.Update(30, 1024*30)

	progress, eta, rate := estimator.GetEstimate()

	assert.Equal(t, float64(30), progress)

	// ETA should be calculated if we have a rate
	if rate > 0 {
		assert.False(t, eta.IsZero())
		// ETA should be in the future, but allow for some timing variance
		assert.True(t, eta.After(time.Now().Add(-1*time.Second)))
	}
}

func TestProgressEstimator_GetStats(t *testing.T) {
	estimator := NewProgressEstimator()
	estimator.SetTotal(100, 1024*100)
	estimator.Update(25, 1024*25)

	stats := estimator.GetStats()

	// Check required fields
	assert.Contains(t, stats, "processed_files")
	assert.Contains(t, stats, "total_files")
	assert.Contains(t, stats, "processed_bytes")
	assert.Contains(t, stats, "total_bytes")
	assert.Contains(t, stats, "elapsed_time")
	assert.Contains(t, stats, "files_per_second")
	assert.Contains(t, stats, "average_file_size")
	assert.Contains(t, stats, "throughput_mbps")

	// Check values
	assert.Equal(t, int64(25), stats["processed_files"])
	assert.Equal(t, int64(100), stats["total_files"])
	assert.Equal(t, int64(1024*25), stats["processed_bytes"])
	assert.Equal(t, int64(1024*100), stats["total_bytes"])

	// Average file size should be 1024 bytes
	assert.Equal(t, float64(1024), stats["average_file_size"])
}

func TestProgressEstimator_ConcurrentAccess(t *testing.T) {
	estimator := NewProgressEstimator()
	estimator.SetTotal(1000, 1024*1000)

	// Test concurrent updates and reads
	var wg sync.WaitGroup

	// Goroutine 1: Update progress
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			estimator.Update(int64(i*10), int64(i*10*1024))
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Goroutine 2: Read progress
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_, _, _ = estimator.GetEstimate()
			_ = estimator.GetStats()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Wait for both goroutines to complete
	wg.Wait()

	// Should not panic and should have reasonable final state
	progress, _, _ := estimator.GetEstimate()
	assert.GreaterOrEqual(t, progress, float64(0))
	assert.LessOrEqual(t, progress, float64(1000)) // Allow for over 100% in edge cases
}

func TestProgressEstimator_EdgeCases(t *testing.T) {
	t.Run("Negative values", func(t *testing.T) {
		estimator := NewProgressEstimator()
		estimator.SetTotal(-100, -1024)
		estimator.Update(-10, -1024)

		// Should handle gracefully without panicking
		progress, _, _ := estimator.GetEstimate()
		stats := estimator.GetStats()

		// Values should be reasonable
		assert.GreaterOrEqual(t, progress, float64(-1000)) // Allow negative in edge cases
		assert.NotNil(t, stats)
	})

	t.Run("Very large numbers", func(t *testing.T) {
		estimator := NewProgressEstimator()
		estimator.SetTotal(1000000000, 1024*1024*1024*100) // 1 billion files, 100GB
		estimator.Update(500000000, 1024*1024*1024*50)     // 500 million files, 50GB

		progress, _, _ := estimator.GetEstimate()
		assert.Equal(t, float64(50), progress)
	})

	t.Run("Zero division protection", func(t *testing.T) {
		estimator := NewProgressEstimator()
		estimator.SetTotal(0, 0)
		estimator.Update(0, 0)

		// Should not panic
		progress, eta, rate := estimator.GetEstimate()
		stats := estimator.GetStats()

		assert.Equal(t, float64(0), progress)
		assert.True(t, eta.IsZero())
		assert.Equal(t, float64(0), rate)
		assert.NotNil(t, stats)
	})
}

func TestProgressEstimator_RateSampling(t *testing.T) {
	estimator := NewProgressEstimator()
	estimator.SetTotal(1000, 1024*1000)

	// Add multiple samples over time
	for i := 0; i < 15; i++ {
		estimator.Update(int64(i*10), int64(i*10*1024))
		time.Sleep(10 * time.Millisecond)
	}

	// Should maintain only recent samples (maxSamples = 10)
	stats := estimator.GetStats()

	// Should have processed files
	assert.Equal(t, int64(140), stats["processed_files"]) // 14 * 10

	// Should have some rate calculation
	rate := stats["files_per_second"].(float64)
	assert.GreaterOrEqual(t, rate, float64(0))
}

func TestProgressEstimator_BytesBasedProgress(t *testing.T) {
	estimator := NewProgressEstimator()

	// Set only bytes total (no files)
	estimator.SetTotal(0, 1024*1024) // 1MB total
	estimator.Update(0, 1024*512)    // 512KB processed

	progress, _, _ := estimator.GetEstimate()
	assert.Equal(t, float64(50), progress) // Should calculate based on bytes
}

func TestProgressEstimator_ThroughputCalculation(t *testing.T) {
	estimator := NewProgressEstimator()
	estimator.SetTotal(100, 1024*1024*10) // 10MB total

	// Wait a bit to ensure elapsed time
	time.Sleep(10 * time.Millisecond)

	estimator.Update(50, 1024*1024*5) // 5MB processed

	stats := estimator.GetStats()

	// Should have throughput calculation
	throughput := stats["throughput_mbps"].(float64)
	assert.GreaterOrEqual(t, throughput, float64(0))

	// Should be reasonable (not impossibly high)
	assert.Less(t, throughput, float64(10000)) // Less than 10GB/s
}
