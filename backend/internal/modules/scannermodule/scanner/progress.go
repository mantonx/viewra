package scanner

import (
	"sync"
	"time"
)

// ScanProgressTracker extends the basic ProgressEstimator with scan-specific functionality
type ScanProgressTracker struct {
	*ProgressEstimator
	mu sync.RWMutex
	
	// Scan-specific metrics
	scanStartTime    time.Time
	lastProgressTime time.Time
	filesSkipped     int64
	errorsCount      int64
	
	// Performance tracking
	peakRate         float64
	avgRate          float64
	performanceSamples []performanceSample
	maxPerfSamples     int
	
	// Progress callbacks
	callbacks []ProgressCallback
	
	// Thresholds for notifications
	progressThresholds []float64
	lastNotifiedProgress float64
}

// performanceSample tracks performance metrics over time
type performanceSample struct {
	timestamp    time.Time
	filesPerSec  float64
	bytesPerSec  float64
	cpuPercent   float64
	memoryMB     float64
}

// ProgressCallback defines the interface for progress notifications
type ProgressCallback interface {
	OnProgressUpdate(progress float64, eta time.Time, rate float64)
	OnMilestoneReached(milestone float64)
	OnRateChange(newRate float64, trend string)
}

// NewScanProgressTracker creates a new scan progress tracker
func NewScanProgressTracker() *ScanProgressTracker {
	return &ScanProgressTracker{
		ProgressEstimator:  NewProgressEstimator(),
		scanStartTime:      time.Now(),
		lastProgressTime:   time.Now(),
		maxPerfSamples:     100,
		performanceSamples: make([]performanceSample, 0, 100),
		callbacks:          make([]ProgressCallback, 0),
		progressThresholds: []float64{10, 25, 50, 75, 90, 95, 99}, // Default milestones
	}
}

// SetProgressThresholds sets the milestones for progress notifications
func (spt *ScanProgressTracker) SetProgressThresholds(thresholds []float64) {
	spt.mu.Lock()
	defer spt.mu.Unlock()
	spt.progressThresholds = thresholds
}

// AddProgressCallback registers a callback for progress updates
func (spt *ScanProgressTracker) AddProgressCallback(callback ProgressCallback) {
	spt.mu.Lock()
	defer spt.mu.Unlock()
	spt.callbacks = append(spt.callbacks, callback)
}

// RemoveProgressCallback unregisters a progress callback
func (spt *ScanProgressTracker) RemoveProgressCallback(callback ProgressCallback) {
	spt.mu.Lock()
	defer spt.mu.Unlock()
	
	for i, cb := range spt.callbacks {
		if cb == callback {
			spt.callbacks = append(spt.callbacks[:i], spt.callbacks[i+1:]...)
			break
		}
	}
}

// UpdateProgress updates progress with additional scan-specific metrics
func (spt *ScanProgressTracker) UpdateProgress(processedFiles, processedBytes, skippedFiles, errorCount int64) {
	spt.mu.Lock()
	defer spt.mu.Unlock()
	
	// Update base progress estimator
	spt.ProgressEstimator.Update(processedFiles, processedBytes)
	
	// Update scan-specific metrics
	spt.filesSkipped = skippedFiles
	spt.errorsCount = errorCount
	spt.lastProgressTime = time.Now()
	
	// Get current estimates
	progress, eta, rate := spt.ProgressEstimator.GetEstimate()
	
	// Update performance tracking
	spt.updatePerformanceMetrics(rate)
	
	// Check for milestones
	spt.checkMilestones(progress)
	
	// Notify callbacks
	spt.notifyCallbacks(progress, eta, rate)
}

// updatePerformanceMetrics tracks performance over time
func (spt *ScanProgressTracker) updatePerformanceMetrics(currentRate float64) {
	now := time.Now()
	
	// Update peak rate
	if currentRate > spt.peakRate {
		spt.peakRate = currentRate
	}
	
	// Calculate average rate
	elapsed := now.Sub(spt.scanStartTime).Seconds()
	if elapsed > 0 {
		spt.avgRate = float64(spt.ProgressEstimator.processedFiles) / elapsed
	}
	
	// Add performance sample (with mock system metrics for now)
	sample := performanceSample{
		timestamp:   now,
		filesPerSec: currentRate,
		bytesPerSec: 0, // Would be calculated from bytes processed
		cpuPercent:  50.0, // Mock value
		memoryMB:    1024.0, // Mock value
	}
	
	spt.performanceSamples = append(spt.performanceSamples, sample)
	
	// Keep only recent samples
	if len(spt.performanceSamples) > spt.maxPerfSamples {
		spt.performanceSamples = spt.performanceSamples[len(spt.performanceSamples)-spt.maxPerfSamples:]
	}
}

// checkMilestones checks if we've reached any progress milestones
func (spt *ScanProgressTracker) checkMilestones(currentProgress float64) {
	for _, threshold := range spt.progressThresholds {
		if currentProgress >= threshold && spt.lastNotifiedProgress < threshold {
			// Milestone reached
			for _, callback := range spt.callbacks {
				go callback.OnMilestoneReached(threshold)
			}
		}
	}
	spt.lastNotifiedProgress = currentProgress
}

// notifyCallbacks notifies all registered callbacks of progress updates
func (spt *ScanProgressTracker) notifyCallbacks(progress float64, eta time.Time, rate float64) {
	for _, callback := range spt.callbacks {
		go callback.OnProgressUpdate(progress, eta, rate)
	}
}

// GetDetailedStats returns comprehensive progress statistics
func (spt *ScanProgressTracker) GetDetailedStats() map[string]interface{} {
	spt.mu.RLock()
	defer spt.mu.RUnlock()
	
	// Get base stats
	stats := spt.ProgressEstimator.GetStats()
	
	// Add scan-specific stats
	elapsed := time.Since(spt.scanStartTime)
	stats["scan_elapsed_time"] = elapsed.String()
	stats["files_skipped"] = spt.filesSkipped
	stats["errors_count"] = spt.errorsCount
	stats["peak_rate"] = spt.peakRate
	stats["average_rate"] = spt.avgRate
	
	// Calculate performance metrics
	if len(spt.performanceSamples) > 0 {
		latest := spt.performanceSamples[len(spt.performanceSamples)-1]
		stats["current_cpu_percent"] = latest.cpuPercent
		stats["current_memory_mb"] = latest.memoryMB
	}
	
	// Calculate efficiency metrics
	totalProcessed := spt.ProgressEstimator.processedFiles + spt.filesSkipped
	if totalProcessed > 0 {
		stats["success_rate"] = float64(spt.ProgressEstimator.processedFiles) / float64(totalProcessed) * 100
		stats["skip_rate"] = float64(spt.filesSkipped) / float64(totalProcessed) * 100
		stats["error_rate"] = float64(spt.errorsCount) / float64(totalProcessed) * 100
	}
	
	return stats
}

// GetPerformanceHistory returns recent performance samples
func (spt *ScanProgressTracker) GetPerformanceHistory() []performanceSample {
	spt.mu.RLock()
	defer spt.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	result := make([]performanceSample, len(spt.performanceSamples))
	copy(result, spt.performanceSamples)
	return result
}

// GetCurrentTrend analyzes the performance trend
func (spt *ScanProgressTracker) GetCurrentTrend() (trend string, confidence float64) {
	spt.mu.RLock()
	defer spt.mu.RUnlock()
	
	if len(spt.performanceSamples) < 3 {
		return "unknown", 0.0
	}
	
	// Analyze recent samples to determine trend
	recent := spt.performanceSamples[len(spt.performanceSamples)-3:]
	
	// Calculate rate changes
	rate1 := recent[0].filesPerSec
	rate2 := recent[1].filesPerSec
	rate3 := recent[2].filesPerSec
	
	change1 := rate2 - rate1
	change2 := rate3 - rate2
	
	// Determine trend
	if change1 > 0 && change2 > 0 {
		trend = "improving"
		confidence = 0.8
	} else if change1 < 0 && change2 < 0 {
		trend = "declining"
		confidence = 0.8
	} else if change1 == 0 && change2 == 0 {
		trend = "stable"
		confidence = 0.9
	} else {
		trend = "fluctuating"
		confidence = 0.6
	}
	
	return trend, confidence
}

// EstimateCompletion provides multiple completion estimates
func (spt *ScanProgressTracker) EstimateCompletion() map[string]interface{} {
	spt.mu.RLock()
	defer spt.mu.RUnlock()
	
	progress, eta, currentRate := spt.ProgressEstimator.GetEstimate()
	
	estimates := map[string]interface{}{
		"current_progress": progress,
		"eta_current_rate": eta,
		"current_rate":     currentRate,
	}
	
	// Estimate based on average rate
	if spt.avgRate > 0 && spt.ProgressEstimator.totalFiles > 0 {
		remaining := spt.ProgressEstimator.totalFiles - spt.ProgressEstimator.processedFiles
		if remaining > 0 {
			etaAvg := time.Now().Add(time.Duration(float64(remaining)/spt.avgRate) * time.Second)
			estimates["eta_average_rate"] = etaAvg
		}
	}
	
	// Estimate based on peak rate (optimistic)
	if spt.peakRate > 0 && spt.ProgressEstimator.totalFiles > 0 {
		remaining := spt.ProgressEstimator.totalFiles - spt.ProgressEstimator.processedFiles
		if remaining > 0 {
			etaPeak := time.Now().Add(time.Duration(float64(remaining)/spt.peakRate) * time.Second)
			estimates["eta_peak_rate"] = etaPeak
		}
	}
	
	// Add confidence level
	if len(spt.performanceSamples) >= 5 {
		estimates["confidence"] = "high"
	} else if len(spt.performanceSamples) >= 2 {
		estimates["confidence"] = "medium"
	} else {
		estimates["confidence"] = "low"
	}
	
	return estimates
}

// Reset resets the progress tracker for a new scan
func (spt *ScanProgressTracker) Reset() {
	spt.mu.Lock()
	defer spt.mu.Unlock()
	
	spt.ProgressEstimator = NewProgressEstimator()
	spt.scanStartTime = time.Now()
	spt.lastProgressTime = time.Now()
	spt.filesSkipped = 0
	spt.errorsCount = 0
	spt.peakRate = 0
	spt.avgRate = 0
	spt.performanceSamples = spt.performanceSamples[:0]
	spt.lastNotifiedProgress = 0
} 