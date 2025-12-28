package pipeline

import (
	"sync"
	"time"
)

// ThroughputTracker tracks the rate of completions per stage using a sliding window.
type ThroughputTracker struct {
	mu       sync.RWMutex
	stages   map[string]*stageMetrics
	window   time.Duration // Time window for calculating rate (e.g., 60 seconds)
	interval time.Duration // Bucket interval (e.g., 5 seconds)
}

// stageMetrics tracks completion counts in time buckets for a single stage.
type stageMetrics struct {
	buckets    []bucket
	totalCount int64 // Running total for calculating deltas
}

// bucket represents a time bucket with completion count.
type bucket struct {
	timestamp time.Time
	count     int64
}

// StageThroughput represents throughput metrics for a single stage.
type StageThroughput struct {
	Stage          string  `json:"stage"`
	ItemsPerSecond float64 `json:"items_per_second"`
	ItemsPerMinute float64 `json:"items_per_minute"`
	RemainingItems int64   `json:"remaining_items"`
	ETASeconds     int64   `json:"eta_seconds"` // 0 if cannot be calculated
}

// NewThroughputTracker creates a new throughput tracker.
// window: time period over which to calculate average rate (e.g., 60s)
// interval: bucket size for aggregating counts (e.g., 5s)
func NewThroughputTracker(window, interval time.Duration) *ThroughputTracker {
	return &ThroughputTracker{
		stages:   make(map[string]*stageMetrics),
		window:   window,
		interval: interval,
	}
}

// RecordCompletion records a completion for a stage.
// This should be called each time an item completes a stage.
func (t *ThroughputTracker) RecordCompletion(stage string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	metrics := t.getOrCreateStageMetrics(stage)
	now := time.Now()

	// Find or create current bucket
	if len(metrics.buckets) == 0 || now.Sub(metrics.buckets[len(metrics.buckets)-1].timestamp) >= t.interval {
		// Create new bucket
		metrics.buckets = append(metrics.buckets, bucket{
			timestamp: now.Truncate(t.interval),
			count:     1,
		})
	} else {
		// Increment current bucket
		metrics.buckets[len(metrics.buckets)-1].count++
	}

	metrics.totalCount++

	// Prune old buckets
	t.pruneOldBuckets(metrics, now)
}

// UpdateFromStats updates throughput based on current queue stats.
// This is an alternative to RecordCompletion - call periodically with current stats.
func (t *ThroughputTracker) UpdateFromStats(stage string, completedCount int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	metrics := t.getOrCreateStageMetrics(stage)
	now := time.Now()

	// Calculate delta since last update
	delta := completedCount - metrics.totalCount
	if delta <= 0 {
		// No new completions or count went backwards (reset)
		if completedCount < metrics.totalCount {
			// Count reset, reinitialize
			metrics.totalCount = completedCount
			metrics.buckets = nil
		}
		return
	}

	// Record the delta in current bucket
	if len(metrics.buckets) == 0 || now.Sub(metrics.buckets[len(metrics.buckets)-1].timestamp) >= t.interval {
		// Create new bucket
		metrics.buckets = append(metrics.buckets, bucket{
			timestamp: now.Truncate(t.interval),
			count:     delta,
		})
	} else {
		// Add to current bucket
		metrics.buckets[len(metrics.buckets)-1].count += delta
	}

	metrics.totalCount = completedCount

	// Prune old buckets
	t.pruneOldBuckets(metrics, now)
}

// GetThroughput returns the current throughput for a stage.
func (t *ThroughputTracker) GetThroughput(stage string, remainingItems int64) StageThroughput {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := StageThroughput{
		Stage:          stage,
		RemainingItems: remainingItems,
	}

	metrics, exists := t.stages[stage]
	if !exists || len(metrics.buckets) == 0 {
		return result
	}

	// Calculate total completions in window
	now := time.Now()
	cutoff := now.Add(-t.window)
	var totalInWindow int64
	var oldestBucket time.Time

	for _, b := range metrics.buckets {
		if b.timestamp.After(cutoff) || b.timestamp.Equal(cutoff) {
			totalInWindow += b.count
			if oldestBucket.IsZero() || b.timestamp.Before(oldestBucket) {
				oldestBucket = b.timestamp
			}
		}
	}

	if totalInWindow == 0 {
		return result
	}

	// Calculate actual time span (from oldest bucket to now)
	elapsed := now.Sub(oldestBucket)
	if elapsed < time.Second {
		elapsed = time.Second // Minimum 1 second to avoid division issues
	}

	result.ItemsPerSecond = float64(totalInWindow) / elapsed.Seconds()
	result.ItemsPerMinute = result.ItemsPerSecond * 60

	// Calculate ETA
	if result.ItemsPerSecond > 0 && remainingItems > 0 {
		result.ETASeconds = int64(float64(remainingItems) / result.ItemsPerSecond)
	}

	return result
}

// GetAllThroughput returns throughput for all tracked stages.
func (t *ThroughputTracker) GetAllThroughput(remainingByStage map[string]int64) map[string]StageThroughput {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]StageThroughput)
	for stage := range t.stages {
		remaining := remainingByStage[stage]
		result[stage] = t.GetThroughput(stage, remaining)
	}
	return result
}

// CalculateOverallETA calculates the overall ETA based on the slowest stage throughput
// and the total remaining items. Each item must pass through all stages, so the
// bottleneck stage determines overall throughput.
// totalRemainingItems should come from the overall progress (unique items not yet fully enriched).
// Returns 0 if ETA cannot be calculated.
func (t *ThroughputTracker) CalculateOverallETA(totalRemainingItems int64) int64 {
	if totalRemainingItems <= 0 {
		return 0
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Find the slowest stage throughput (the bottleneck)
	var slowestThroughput float64 = -1
	now := time.Now()
	cutoff := now.Add(-t.window)

	for _, metrics := range t.stages {
		if len(metrics.buckets) == 0 {
			continue
		}

		// Calculate throughput for this stage
		var totalInWindow int64
		var oldestBucket time.Time

		for _, b := range metrics.buckets {
			if b.timestamp.After(cutoff) || b.timestamp.Equal(cutoff) {
				totalInWindow += b.count
				if oldestBucket.IsZero() || b.timestamp.Before(oldestBucket) {
					oldestBucket = b.timestamp
				}
			}
		}

		if totalInWindow == 0 {
			continue
		}

		elapsed := now.Sub(oldestBucket)
		if elapsed < time.Second {
			elapsed = time.Second
		}

		itemsPerSecond := float64(totalInWindow) / elapsed.Seconds()

		// Track the slowest (minimum) throughput - this is the bottleneck
		if slowestThroughput < 0 || itemsPerSecond < slowestThroughput {
			slowestThroughput = itemsPerSecond
		}
	}

	if slowestThroughput <= 0 {
		return 0
	}

	// ETA = remaining items / bottleneck throughput
	return int64(float64(totalRemainingItems) / slowestThroughput)
}

// getOrCreateStageMetrics returns or creates metrics for a stage.
// Must be called with lock held.
func (t *ThroughputTracker) getOrCreateStageMetrics(stage string) *stageMetrics {
	if metrics, exists := t.stages[stage]; exists {
		return metrics
	}
	metrics := &stageMetrics{
		buckets: make([]bucket, 0, int(t.window/t.interval)+1),
	}
	t.stages[stage] = metrics
	return metrics
}

// pruneOldBuckets removes buckets older than the window.
// Must be called with lock held.
func (t *ThroughputTracker) pruneOldBuckets(metrics *stageMetrics, now time.Time) {
	cutoff := now.Add(-t.window)
	firstValid := 0
	for i, b := range metrics.buckets {
		if b.timestamp.After(cutoff) || b.timestamp.Equal(cutoff) {
			firstValid = i
			break
		}
		firstValid = i + 1
	}
	if firstValid > 0 && firstValid < len(metrics.buckets) {
		metrics.buckets = metrics.buckets[firstValid:]
	} else if firstValid >= len(metrics.buckets) {
		metrics.buckets = metrics.buckets[:0]
	}
}

// Reset clears all tracked metrics.
func (t *ThroughputTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stages = make(map[string]*stageMetrics)
}

// ResetStage clears metrics for a specific stage.
func (t *ThroughputTracker) ResetStage(stage string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.stages, stage)
}
