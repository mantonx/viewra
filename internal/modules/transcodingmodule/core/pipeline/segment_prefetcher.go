// Package pipeline provides segment prefetching and intelligent buffering for snappy startup
package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

// SegmentPrefetcher handles intelligent segment buffering for instant playback startup
type SegmentPrefetcher struct {
	logger           hclog.Logger
	contentStorePath string

	// Prefetch configuration
	initialSegments  int     // Number of segments to prefetch for startup
	bufferSize       int     // Maximum segments to keep in buffer
	prefetchDistance int     // How far ahead to prefetch during playback
	bufferThreshold  float64 // Buffer level to trigger prefetching (0.0-1.0)

	// Buffer management
	segmentBuffer map[string]*SegmentBuffer // contentHash -> buffer
	bufferMutex   sync.RWMutex

	// Prefetch workers
	workers     int
	workQueue   chan PrefetchTask
	workerGroup sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc

	// Metrics
	cacheHits       int64
	cacheMisses     int64
	prefetchedBytes int64
	metricsMutex    sync.RWMutex
}

// SegmentBuffer manages buffered segments for a specific content
type SegmentBuffer struct {
	contentHash string
	segments    map[int]*BufferedSegment // segmentIndex -> segment
	accessOrder []int                    // LRU tracking
	totalSize   int64
	lastAccess  time.Time
	mutex       sync.RWMutex

	// Playback state
	currentPosition int
	isPlaying       bool
	playbackSpeed   float64
}

// BufferedSegment represents a segment in the buffer
type BufferedSegment struct {
	index         int
	data          []byte
	size          int64
	loadTime      time.Time
	accessCount   int64
	lastAccess    time.Time
	prefetchScore float64 // Priority score for prefetching
}

// PrefetchTask represents a segment prefetching task
type PrefetchTask struct {
	contentHash  string
	segmentIndex int
	priority     int // Higher = more urgent
	sessionID    string
	requestTime  time.Time
}

// PrefetchStrategy defines different prefetching strategies
type PrefetchStrategy int

const (
	StrategyLinear     PrefetchStrategy = iota // Simple sequential prefetch
	StrategyAdaptive                           // Adapt based on playback patterns
	StrategyPredictive                         // ML-based prediction (future)
)

// NewSegmentPrefetcher creates a new segment prefetcher
func NewSegmentPrefetcher(contentStorePath string, logger hclog.Logger) *SegmentPrefetcher {
	ctx, cancel := context.WithCancel(context.Background())

	prefetcher := &SegmentPrefetcher{
		logger:           logger,
		contentStorePath: contentStorePath,

		// Default configuration optimized for web streaming
		initialSegments:  3,   // 3 segments for instant startup (12 seconds at 4s/segment)
		bufferSize:       10,  // Keep 10 segments buffered (40 seconds)
		prefetchDistance: 5,   // Prefetch 5 segments ahead (20 seconds)
		bufferThreshold:  0.3, // Prefetch when buffer drops below 30%

		segmentBuffer: make(map[string]*SegmentBuffer),
		workers:       3, // 3 concurrent prefetch workers
		workQueue:     make(chan PrefetchTask, 100),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Start prefetch workers
	for i := 0; i < prefetcher.workers; i++ {
		prefetcher.workerGroup.Add(1)
		go prefetcher.prefetchWorker()
	}

	logger.Info("Segment prefetcher started",
		"workers", prefetcher.workers,
		"initial_segments", prefetcher.initialSegments,
		"buffer_size", prefetcher.bufferSize,
	)

	return prefetcher
}

// PrefetchForStartup prefetches initial segments for instant playback
func (sp *SegmentPrefetcher) PrefetchForStartup(contentHash string, sessionID string) error {
	sp.logger.Info("Prefetching segments for startup",
		"content_hash", contentHash[:8],
		"session", sessionID,
		"segments", sp.initialSegments,
	)

	// Get or create buffer for this content
	buffer := sp.getOrCreateBuffer(contentHash)

	// Prefetch initial segments with high priority
	for i := 0; i < sp.initialSegments; i++ {
		task := PrefetchTask{
			contentHash:  contentHash,
			segmentIndex: i,
			priority:     100 - i, // Decreasing priority
			sessionID:    sessionID,
			requestTime:  time.Now(),
		}

		select {
		case sp.workQueue <- task:
			sp.logger.Debug("Queued startup prefetch", "segment", i)
		case <-sp.ctx.Done():
			return fmt.Errorf("prefetcher stopped")
		default:
			sp.logger.Warn("Prefetch queue full, skipping segment", "segment", i)
		}
	}

	// Wait for initial segments to be buffered (with timeout)
	startTime := time.Now()
	timeout := 10 * time.Second

	for time.Since(startTime) < timeout {
		buffer.mutex.RLock()
		bufferedCount := len(buffer.segments)
		buffer.mutex.RUnlock()

		if bufferedCount >= sp.initialSegments {
			sp.logger.Info("Startup prefetch completed",
				"duration", time.Since(startTime),
				"segments", bufferedCount,
			)
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Log partial success
	buffer.mutex.RLock()
	bufferedCount := len(buffer.segments)
	buffer.mutex.RUnlock()

	sp.logger.Warn("Startup prefetch timeout",
		"buffered", bufferedCount,
		"requested", sp.initialSegments,
		"duration", time.Since(startTime),
	)

	return nil
}

// UpdatePlaybackPosition updates the current playback position for intelligent prefetching
func (sp *SegmentPrefetcher) UpdatePlaybackPosition(contentHash string, segmentIndex int, isPlaying bool, playbackSpeed float64) {
	buffer := sp.getOrCreateBuffer(contentHash)

	buffer.mutex.Lock()
	oldPosition := buffer.currentPosition
	buffer.currentPosition = segmentIndex
	buffer.isPlaying = isPlaying
	buffer.playbackSpeed = playbackSpeed
	buffer.lastAccess = time.Now()
	buffer.mutex.Unlock()

	// Mark current segment as accessed
	if segment := sp.getBufferedSegment(contentHash, segmentIndex); segment != nil {
		segment.lastAccess = time.Now()
		segment.accessCount++
	}

	// Trigger adaptive prefetching if playback position changed significantly
	if isPlaying && (segmentIndex > oldPosition || segmentIndex < oldPosition-2) {
		go sp.triggerAdaptivePrefetch(contentHash, segmentIndex, playbackSpeed)
	}

	sp.logger.Debug("Updated playback position",
		"content_hash", contentHash[:8],
		"segment", segmentIndex,
		"playing", isPlaying,
		"speed", playbackSpeed,
	)
}

// GetSegment retrieves a segment from buffer or loads it
func (sp *SegmentPrefetcher) GetSegment(contentHash string, segmentIndex int) ([]byte, error) {
	// Try buffer first (cache hit)
	if segment := sp.getBufferedSegment(contentHash, segmentIndex); segment != nil {
		sp.incrementCacheHits()
		segment.lastAccess = time.Now()
		segment.accessCount++

		sp.logger.Debug("Segment cache hit",
			"content_hash", contentHash[:8],
			"segment", segmentIndex,
			"size", len(segment.data),
		)

		return segment.data, nil
	}

	// Cache miss - load segment
	sp.incrementCacheMisses()

	sp.logger.Debug("Segment cache miss, loading",
		"content_hash", contentHash[:8],
		"segment", segmentIndex,
	)

	// Load segment from storage
	data, err := sp.loadSegmentFromStorage(contentHash, segmentIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to load segment %d: %w", segmentIndex, err)
	}

	// Add to buffer
	sp.addToBuffer(contentHash, segmentIndex, data)

	return data, nil
}

// triggerAdaptivePrefetch starts intelligent prefetching based on playback patterns
func (sp *SegmentPrefetcher) triggerAdaptivePrefetch(contentHash string, currentSegment int, playbackSpeed float64) {
	buffer := sp.getOrCreateBuffer(contentHash)

	// Calculate how many segments to prefetch based on playback speed
	prefetchCount := sp.prefetchDistance
	if playbackSpeed > 1.0 {
		prefetchCount = int(float64(prefetchCount) * playbackSpeed)
	}

	// Check current buffer level
	buffer.mutex.RLock()
	bufferedAhead := 0
	for i := currentSegment + 1; i <= currentSegment+sp.prefetchDistance; i++ {
		if _, exists := buffer.segments[i]; exists {
			bufferedAhead++
		}
	}
	bufferLevel := float64(bufferedAhead) / float64(sp.prefetchDistance)
	buffer.mutex.RUnlock()

	// Only prefetch if buffer level is below threshold
	if bufferLevel > sp.bufferThreshold {
		return
	}

	sp.logger.Debug("Triggering adaptive prefetch",
		"content_hash", contentHash[:8],
		"current_segment", currentSegment,
		"buffer_level", bufferLevel,
		"prefetch_count", prefetchCount,
	)

	// Queue prefetch tasks with calculated priorities
	for i := 1; i <= prefetchCount; i++ {
		segmentIndex := currentSegment + i

		// Skip if already buffered
		if sp.getBufferedSegment(contentHash, segmentIndex) != nil {
			continue
		}

		// Calculate priority (closer segments = higher priority)
		priority := 50 - i

		task := PrefetchTask{
			contentHash:  contentHash,
			segmentIndex: segmentIndex,
			priority:     priority,
			requestTime:  time.Now(),
		}

		select {
		case sp.workQueue <- task:
			sp.logger.Debug("Queued adaptive prefetch",
				"segment", segmentIndex,
				"priority", priority,
			)
		case <-sp.ctx.Done():
			return
		default:
			sp.logger.Debug("Prefetch queue full, skipping", "segment", segmentIndex)
		}
	}
}

// prefetchWorker processes prefetch tasks
func (sp *SegmentPrefetcher) prefetchWorker() {
	defer sp.workerGroup.Done()

	for {
		select {
		case <-sp.ctx.Done():
			return
		case task := <-sp.workQueue:
			sp.processPrefetchTask(task)
		}
	}
}

// processPrefetchTask loads and buffers a segment
func (sp *SegmentPrefetcher) processPrefetchTask(task PrefetchTask) {
	startTime := time.Now()

	// Skip if already buffered
	if sp.getBufferedSegment(task.contentHash, task.segmentIndex) != nil {
		sp.logger.Debug("Segment already buffered, skipping",
			"content_hash", task.contentHash[:8],
			"segment", task.segmentIndex,
		)
		return
	}

	// Load segment data
	data, err := sp.loadSegmentFromStorage(task.contentHash, task.segmentIndex)
	if err != nil {
		sp.logger.Error("Failed to prefetch segment",
			"content_hash", task.contentHash[:8],
			"segment", task.segmentIndex,
			"error", err,
		)
		return
	}

	// Add to buffer
	sp.addToBuffer(task.contentHash, task.segmentIndex, data)

	duration := time.Since(startTime)
	sp.logger.Debug("Prefetched segment",
		"content_hash", task.contentHash[:8],
		"segment", task.segmentIndex,
		"size", len(data),
		"duration", duration,
		"priority", task.priority,
	)

	// Update metrics
	sp.metricsMutex.Lock()
	sp.prefetchedBytes += int64(len(data))
	sp.metricsMutex.Unlock()
}

// loadSegmentFromStorage loads a segment from the content store
func (sp *SegmentPrefetcher) loadSegmentFromStorage(contentHash string, segmentIndex int) ([]byte, error) {
	// Build segment path using content store structure
	segmentPath := filepath.Join(
		sp.contentStorePath,
		"content",
		contentHash[:2],
		contentHash,
		"segments",
		fmt.Sprintf("segment_%03d.mp4", segmentIndex),
	)

	// Read segment file
	data, err := os.ReadFile(segmentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read segment file %s: %w", segmentPath, err)
	}

	return data, nil
}

// addToBuffer adds a segment to the buffer with LRU management
func (sp *SegmentPrefetcher) addToBuffer(contentHash string, segmentIndex int, data []byte) {
	buffer := sp.getOrCreateBuffer(contentHash)

	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()

	// Create buffered segment
	segment := &BufferedSegment{
		index:         segmentIndex,
		data:          data,
		size:          int64(len(data)),
		loadTime:      time.Now(),
		accessCount:   0,
		lastAccess:    time.Now(),
		prefetchScore: sp.calculatePrefetchScore(segmentIndex, buffer),
	}

	// Add to buffer
	buffer.segments[segmentIndex] = segment
	buffer.accessOrder = append(buffer.accessOrder, segmentIndex)
	buffer.totalSize += segment.size

	// Enforce buffer size limit
	sp.evictIfNeeded(buffer)

	sp.logger.Debug("Added segment to buffer",
		"content_hash", contentHash[:8],
		"segment", segmentIndex,
		"buffer_size", len(buffer.segments),
		"total_size", buffer.totalSize,
	)
}

// evictIfNeeded removes old segments if buffer is full
func (sp *SegmentPrefetcher) evictIfNeeded(buffer *SegmentBuffer) {
	// Evict segments if buffer is too large
	for len(buffer.segments) > sp.bufferSize {
		// Sort by LRU (least recently used first)
		sort.Slice(buffer.accessOrder, func(i, j int) bool {
			seg1 := buffer.segments[buffer.accessOrder[i]]
			seg2 := buffer.segments[buffer.accessOrder[j]]
			return seg1.lastAccess.Before(seg2.lastAccess)
		})

		// Remove oldest segment
		oldestIndex := buffer.accessOrder[0]
		oldestSegment := buffer.segments[oldestIndex]

		delete(buffer.segments, oldestIndex)
		buffer.accessOrder = buffer.accessOrder[1:]
		buffer.totalSize -= oldestSegment.size

		sp.logger.Debug("Evicted segment from buffer",
			"segment", oldestIndex,
			"last_access", oldestSegment.lastAccess,
			"buffer_size", len(buffer.segments),
		)
	}
}

// calculatePrefetchScore calculates priority score for a segment
func (sp *SegmentPrefetcher) calculatePrefetchScore(segmentIndex int, buffer *SegmentBuffer) float64 {
	// Base score decreases with distance from current position
	distance := abs(segmentIndex - buffer.currentPosition)
	baseScore := 100.0 / (1.0 + float64(distance))

	// Boost score for forward direction during playback
	if buffer.isPlaying && segmentIndex > buffer.currentPosition {
		baseScore *= 1.5
	}

	// Adjust for playback speed
	if buffer.playbackSpeed > 1.0 {
		baseScore *= buffer.playbackSpeed
	}

	return baseScore
}

// getOrCreateBuffer gets or creates a segment buffer for content
func (sp *SegmentPrefetcher) getOrCreateBuffer(contentHash string) *SegmentBuffer {
	sp.bufferMutex.Lock()
	defer sp.bufferMutex.Unlock()

	if buffer, exists := sp.segmentBuffer[contentHash]; exists {
		return buffer
	}

	buffer := &SegmentBuffer{
		contentHash:     contentHash,
		segments:        make(map[int]*BufferedSegment),
		accessOrder:     make([]int, 0),
		totalSize:       0,
		lastAccess:      time.Now(),
		currentPosition: 0,
		isPlaying:       false,
		playbackSpeed:   1.0,
	}

	sp.segmentBuffer[contentHash] = buffer

	sp.logger.Debug("Created new segment buffer", "content_hash", contentHash[:8])
	return buffer
}

// getBufferedSegment retrieves a segment from buffer if available
func (sp *SegmentPrefetcher) getBufferedSegment(contentHash string, segmentIndex int) *BufferedSegment {
	sp.bufferMutex.RLock()
	buffer, exists := sp.segmentBuffer[contentHash]
	sp.bufferMutex.RUnlock()

	if !exists {
		return nil
	}

	buffer.mutex.RLock()
	segment, exists := buffer.segments[segmentIndex]
	buffer.mutex.RUnlock()

	if !exists {
		return nil
	}

	return segment
}

// GetMetrics returns prefetching metrics
func (sp *SegmentPrefetcher) GetMetrics() map[string]interface{} {
	sp.metricsMutex.RLock()
	sp.bufferMutex.RLock()
	defer sp.metricsMutex.RUnlock()
	defer sp.bufferMutex.RUnlock()

	totalBufferedSegments := 0
	totalBufferedSize := int64(0)

	for _, buffer := range sp.segmentBuffer {
		buffer.mutex.RLock()
		totalBufferedSegments += len(buffer.segments)
		totalBufferedSize += buffer.totalSize
		buffer.mutex.RUnlock()
	}

	hitRate := 0.0
	totalRequests := sp.cacheHits + sp.cacheMisses
	if totalRequests > 0 {
		hitRate = float64(sp.cacheHits) / float64(totalRequests)
	}

	return map[string]interface{}{
		"cache_hits":        sp.cacheHits,
		"cache_misses":      sp.cacheMisses,
		"hit_rate":          hitRate,
		"total_requests":    totalRequests,
		"prefetched_bytes":  sp.prefetchedBytes,
		"buffered_segments": totalBufferedSegments,
		"buffered_size":     totalBufferedSize,
		"active_buffers":    len(sp.segmentBuffer),
		"queue_length":      len(sp.workQueue),
		"workers":           sp.workers,
	}
}

// CleanupStaleBuffers removes unused buffers to free memory
func (sp *SegmentPrefetcher) CleanupStaleBuffers(maxAge time.Duration) {
	sp.bufferMutex.Lock()
	defer sp.bufferMutex.Unlock()

	now := time.Now()
	removed := 0

	for contentHash, buffer := range sp.segmentBuffer {
		buffer.mutex.RLock()
		lastAccess := buffer.lastAccess
		bufferSize := len(buffer.segments)
		buffer.mutex.RUnlock()

		if now.Sub(lastAccess) > maxAge {
			delete(sp.segmentBuffer, contentHash)
			removed++

			sp.logger.Debug("Removed stale buffer",
				"content_hash", contentHash[:8],
				"last_access", lastAccess,
				"segments", bufferSize,
			)
		}
	}

	if removed > 0 {
		sp.logger.Info("Cleaned up stale buffers",
			"removed", removed,
			"remaining", len(sp.segmentBuffer),
		)
	}
}

// Stop shuts down the prefetcher
func (sp *SegmentPrefetcher) Stop() {
	sp.logger.Info("Stopping segment prefetcher")

	sp.cancel()
	sp.workerGroup.Wait()

	// Clear all buffers
	sp.bufferMutex.Lock()
	sp.segmentBuffer = make(map[string]*SegmentBuffer)
	sp.bufferMutex.Unlock()

	sp.logger.Info("Segment prefetcher stopped")
}

// incrementCacheHits atomically increments cache hits
func (sp *SegmentPrefetcher) incrementCacheHits() {
	sp.metricsMutex.Lock()
	sp.cacheHits++
	sp.metricsMutex.Unlock()
}

// incrementCacheMisses atomically increments cache misses
func (sp *SegmentPrefetcher) incrementCacheMisses() {
	sp.metricsMutex.Lock()
	sp.cacheMisses++
	sp.metricsMutex.Unlock()
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// GetBufferStatus returns buffer status for a specific content
func (sp *SegmentPrefetcher) GetBufferStatus(contentHash string) map[string]interface{} {
	buffer := sp.getOrCreateBuffer(contentHash)

	buffer.mutex.RLock()
	defer buffer.mutex.RUnlock()

	// Calculate buffer coverage around current position
	currentPos := buffer.currentPosition
	bufferAhead := 0
	bufferBehind := 0

	for segmentIndex := range buffer.segments {
		if segmentIndex > currentPos {
			bufferAhead++
		} else if segmentIndex < currentPos {
			bufferBehind++
		}
	}

	return map[string]interface{}{
		"content_hash":     contentHash[:8],
		"current_position": buffer.currentPosition,
		"is_playing":       buffer.isPlaying,
		"playback_speed":   buffer.playbackSpeed,
		"total_segments":   len(buffer.segments),
		"buffer_ahead":     bufferAhead,
		"buffer_behind":    bufferBehind,
		"total_size":       buffer.totalSize,
		"last_access":      buffer.lastAccess,
	}
}
