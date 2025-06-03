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

	// Rate calculation - simplified approach
	recentSamples []rateSample
	maxSamples    int

	// Discovery phase tracking for stable progress
	discoveryComplete bool
	initialFileCount  int64
}

type rateSample struct {
	timestamp time.Time
	files     int64
	bytes     int64
}

// NewProgressEstimator creates a new progress estimator
func NewProgressEstimator() *ProgressEstimator {
	return &ProgressEstimator{
		startTime:         time.Now(),
		lastUpdateTime:    time.Now(),
		maxSamples:        5, // Reduced for simpler calculation
		recentSamples:     make([]rateSample, 0, 5),
		discoveryComplete: false,
	}
}

// SetTotal sets the total expected files and bytes
func (pe *ProgressEstimator) SetTotal(files, bytes int64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	// Only update total if discovery is not complete to prevent constant changes
	if !pe.discoveryComplete {
		pe.totalFiles = files
		pe.totalBytes = bytes

		// If this is the first meaningful count, save it as initial
		if pe.initialFileCount == 0 && files > 100 {
			pe.initialFileCount = files
		}
	}
}

// SetDiscoveryComplete marks that file discovery is finished and total won't change
func (pe *ProgressEstimator) SetDiscoveryComplete() {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.discoveryComplete = true
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
		pe.recentSamples = pe.recentSamples[1:]
	}

	// Update counters
	pe.processedFiles = processedFiles
	pe.processedBytes = processedBytes
	pe.lastUpdateTime = now
}

// GetEstimate returns the current progress, ETA, and processing rate
func (pe *ProgressEstimator) GetEstimate() (progress float64, eta time.Time, filesPerSecond float64) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	// Calculate progress
	if pe.totalFiles > 0 {
		progress = float64(pe.processedFiles) / float64(pe.totalFiles) * 100
		// Cap progress at 99% if discovery is not complete to prevent >100%
		if !pe.discoveryComplete && progress > 99.0 {
			progress = 99.0
		}
	}

	// Calculate processing rate using simple average over recent samples
	filesPerSecond = pe.calculateSimpleRate()

	// Calculate ETA using multiple methods with fallbacks
	remainingFiles := pe.totalFiles - pe.processedFiles
	if remainingFiles <= 0 || pe.totalFiles <= 0 || filesPerSecond <= 0 {
		return progress, time.Time{}, filesPerSecond
	}

	now := time.Now()
	elapsed := now.Sub(pe.startTime)

	// Method 1: Recent rate (preferred for short-term accuracy)
	if filesPerSecond > 0.01 && elapsed.Seconds() > 10 {
		remainingSeconds := float64(remainingFiles) / filesPerSecond
		if remainingSeconds > 0 && remainingSeconds < (24*3600) { // Max 24 hours
			eta = now.Add(time.Duration(remainingSeconds) * time.Second)
			return progress, eta, filesPerSecond
		}
	}

	// Method 2: Overall average rate (more stable for long scans)
	if elapsed.Seconds() > 30 && pe.processedFiles > 0 {
		avgRate := float64(pe.processedFiles) / elapsed.Seconds()
		if avgRate > 0.001 {
			remainingSeconds := float64(remainingFiles) / avgRate
			if remainingSeconds > 0 && remainingSeconds < (48*3600) { // Max 48 hours
				eta = now.Add(time.Duration(remainingSeconds) * time.Second)
				return progress, eta, avgRate
			}
		}
	}

	// Method 3: Linear extrapolation from progress (most conservative)
	if progress > 5 && progress < 95 && elapsed.Seconds() > 60 {
		totalEstimatedSeconds := elapsed.Seconds() * (100.0 / progress)
		remainingSeconds := totalEstimatedSeconds - elapsed.Seconds()
		if remainingSeconds > 0 && remainingSeconds < (72*3600) { // Max 72 hours
			eta = now.Add(time.Duration(remainingSeconds) * time.Second)
			return progress, eta, filesPerSecond
		}
	}

	// If all methods fail, return no ETA
	return progress, time.Time{}, filesPerSecond
}

// calculateSimpleRate calculates processing rate using simple average
func (pe *ProgressEstimator) calculateSimpleRate() float64 {
	if len(pe.recentSamples) < 2 {
		return 0
	}

	// Use first and last samples for simple rate calculation
	oldest := pe.recentSamples[0]
	newest := pe.recentSamples[len(pe.recentSamples)-1]

	duration := newest.timestamp.Sub(oldest.timestamp).Seconds()
	if duration <= 0 {
		return 0
	}

	filesProcessed := newest.files - oldest.files
	if filesProcessed <= 0 {
		return 0
	}

	return float64(filesProcessed) / duration
}

// GetTotal returns the current total files estimate
func (pe *ProgressEstimator) GetTotal() int64 {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return pe.totalFiles
}

// GetTotalBytes returns the current total bytes estimate
func (pe *ProgressEstimator) GetTotalBytes() int64 {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return pe.totalBytes
}

// IsDiscoveryComplete returns whether file discovery is finished
func (pe *ProgressEstimator) IsDiscoveryComplete() bool {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return pe.discoveryComplete
}

// GetStats returns detailed statistics
func (pe *ProgressEstimator) GetStats() map[string]interface{} {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	elapsed := time.Since(pe.startTime)
	rate := pe.calculateSimpleRate()

	stats := map[string]interface{}{
		"processed_files":    pe.processedFiles,
		"total_files":        pe.totalFiles,
		"processed_bytes":    pe.processedBytes,
		"total_bytes":        pe.totalBytes,
		"elapsed_time":       elapsed.String(),
		"files_per_second":   rate,
		"discovery_complete": pe.discoveryComplete,
		"initial_file_count": pe.initialFileCount,
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
