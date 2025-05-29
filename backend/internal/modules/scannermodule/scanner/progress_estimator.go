package scanner

import (
	"sync"
	"time"
)

// ProgressEstimator provides accurate progress estimation and ETA calculation
type ProgressEstimator struct {
	mu             sync.RWMutex
	startTime      time.Time
	lastUpdateTime time.Time

	// Metrics
	totalFiles     int64
	processedFiles int64
	totalBytes     int64
	processedBytes int64

	// Rate calculation
	recentSamples []rateSample
	maxSamples    int

	// Smoothing
	smoothingFactor float64
	currentRate     float64
}

type rateSample struct {
	timestamp time.Time
	files     int64
	bytes     int64
}

// NewProgressEstimator creates a new progress estimator
func NewProgressEstimator() *ProgressEstimator {
	return &ProgressEstimator{
		startTime:       time.Now(),
		lastUpdateTime:  time.Now(),
		maxSamples:      10,
		smoothingFactor: 0.3,
		recentSamples:   make([]rateSample, 0, 10),
	}
}

// SetTotal sets the total expected files and bytes
func (pe *ProgressEstimator) SetTotal(files, bytes int64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	pe.totalFiles = files
	pe.totalBytes = bytes
}

// Update updates the progress with current processed counts
func (pe *ProgressEstimator) Update(processedFiles, processedBytes int64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	now := time.Now()

	// Add new sample
	pe.recentSamples = append(pe.recentSamples, rateSample{
		timestamp: now,
		files:     processedFiles,
		bytes:     processedBytes,
	})

	// Keep only recent samples
	if len(pe.recentSamples) > pe.maxSamples {
		pe.recentSamples = pe.recentSamples[len(pe.recentSamples)-pe.maxSamples:]
	}

	// Update counters
	pe.processedFiles = processedFiles
	pe.processedBytes = processedBytes
	pe.lastUpdateTime = now

	// Calculate rate
	pe.calculateRate()
}

// calculateRate calculates the current processing rate
func (pe *ProgressEstimator) calculateRate() {
	if len(pe.recentSamples) < 2 {
		return
	}

	// Calculate rate from recent samples
	oldest := pe.recentSamples[0]
	newest := pe.recentSamples[len(pe.recentSamples)-1]

	duration := newest.timestamp.Sub(oldest.timestamp).Seconds()
	if duration <= 0 {
		return
	}

	filesPerSecond := float64(newest.files-oldest.files) / duration

	// Apply exponential smoothing
	if pe.currentRate == 0 {
		pe.currentRate = filesPerSecond
	} else {
		pe.currentRate = pe.smoothingFactor*filesPerSecond + (1-pe.smoothingFactor)*pe.currentRate
	}
}

// GetEstimate returns the current progress, ETA, and processing rate
func (pe *ProgressEstimator) GetEstimate() (progress float64, eta time.Time, filesPerSecond float64) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	// Calculate progress
	if pe.totalFiles > 0 {
		progress = float64(pe.processedFiles) / float64(pe.totalFiles) * 100
	} else if pe.totalBytes > 0 {
		progress = float64(pe.processedBytes) / float64(pe.totalBytes) * 100
	}

	// Calculate ETA
	if pe.currentRate > 0 && pe.totalFiles > 0 && pe.processedFiles < pe.totalFiles {
		remainingFiles := pe.totalFiles - pe.processedFiles
		if remainingFiles > 0 {
			remainingSeconds := float64(remainingFiles) / pe.currentRate
			eta = time.Now().Add(time.Duration(remainingSeconds) * time.Second)
		}
	} else if pe.processedFiles > 0 && pe.totalFiles > 0 && pe.processedFiles < pe.totalFiles {
		// Fallback: simple linear estimation based on elapsed time
		elapsed := time.Since(pe.startTime)
		if elapsed.Seconds() > 0 {
			avgRate := float64(pe.processedFiles) / elapsed.Seconds()
			if avgRate > 0 {
				remainingFiles := pe.totalFiles - pe.processedFiles
				remainingSeconds := float64(remainingFiles) / avgRate
				eta = time.Now().Add(time.Duration(remainingSeconds) * time.Second)
			}
		}
	}

	// If ETA is still zero and we have progress, use simple percentage-based estimation
	if eta.IsZero() && progress > 0 && progress < 100 {
		elapsed := time.Since(pe.startTime)
		if elapsed.Seconds() > 0 {
			totalDuration := elapsed.Seconds() * (100 / progress)
			remainingDuration := totalDuration - elapsed.Seconds()
			if remainingDuration > 0 {
				eta = time.Now().Add(time.Duration(remainingDuration) * time.Second)
			}
		}
	}

	return progress, eta, pe.currentRate
}

// GetStats returns detailed statistics
func (pe *ProgressEstimator) GetStats() map[string]interface{} {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	elapsed := time.Since(pe.startTime)

	stats := map[string]interface{}{
		"processed_files":  pe.processedFiles,
		"total_files":      pe.totalFiles,
		"processed_bytes":  pe.processedBytes,
		"total_bytes":      pe.totalBytes,
		"elapsed_time":     elapsed.String(),
		"files_per_second": pe.currentRate,
	}

	// Calculate average file size safely
	if pe.processedFiles > 0 {
		stats["average_file_size"] = float64(pe.processedBytes) / float64(pe.processedFiles)
	} else {
		stats["average_file_size"] = 0.0
	}

	// Add throughput in MB/s
	if elapsed.Seconds() > 0 {
		throughputMBps := float64(pe.processedBytes) / elapsed.Seconds() / (1024 * 1024)
		stats["throughput_mbps"] = throughputMBps
	}

	return stats
}
