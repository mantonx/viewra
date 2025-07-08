// Package core provides transcode deduplication functionality.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/models"
	"github.com/mantonx/viewra/internal/utils"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// TranscodeRequest represents a transcode request with deduplication info
type TranscodeRequest struct {
	MediaFileID   string
	InputPath     string
	Container     string
	VideoCodec    string
	AudioCodec    string
	Resolution    *plugins.Resolution
	VideoBitrate  int
	AudioBitrate  int
	UserID        string
	DeviceProfile string
	RequestTime   time.Time
}

// TranscodeResult represents the result of a transcode operation
type TranscodeResult struct {
	TranscodeID       string
	OutputPath        string
	FileSize          int64
	Duration          int64
	WasDeduped        bool
	OriginalRequestID string
	CreatedAt         time.Time
}

// TranscodeDeduplicator handles deduplication of transcode requests
type TranscodeDeduplicator struct {
	logger hclog.Logger
	db     *gorm.DB

	// In-memory cache for fast lookups
	mu              sync.RWMutex
	pendingRequests map[string]*PendingRequest
	completedCache  map[string]*TranscodeResult
	cacheMaxSize    int
	cacheMaxAge     time.Duration
}

// PendingRequest tracks ongoing transcode requests
type PendingRequest struct {
	RequestID     string
	TranscodeHash string
	RequestTime   time.Time
	Waiters       []chan *TranscodeResult
	InProgress    bool
}

// NewTranscodeDeduplicator creates a new transcode deduplicator
func NewTranscodeDeduplicator(logger hclog.Logger, db *gorm.DB) *TranscodeDeduplicator {
	return &TranscodeDeduplicator{
		logger:          logger,
		db:              db,
		pendingRequests: make(map[string]*PendingRequest),
		completedCache:  make(map[string]*TranscodeResult),
		cacheMaxSize:    1000,           // Keep last 1000 transcodes in memory
		cacheMaxAge:     24 * time.Hour, // Cache for 24 hours
	}
}

// GenerateTranscodeHash creates a unique hash for transcode parameters
func (td *TranscodeDeduplicator) GenerateTranscodeHash(req *TranscodeRequest) string {
	// Create a canonical representation of transcode parameters
	params := []string{
		req.MediaFileID,
		req.Container,
		req.VideoCodec,
		req.AudioCodec,
		fmt.Sprintf("%d", req.VideoBitrate),
		fmt.Sprintf("%d", req.AudioBitrate),
	}

	if req.Resolution != nil {
		params = append(params, fmt.Sprintf("%dx%d", req.Resolution.Width, req.Resolution.Height))
	}

	// Sort to ensure consistent hashing regardless of parameter order
	sort.Strings(params)

	// Create hash
	hasher := sha256.New()
	hasher.Write([]byte(strings.Join(params, "|")))
	return hex.EncodeToString(hasher.Sum(nil))[:16] // Use first 16 chars
}

// RequestTranscode requests a transcode with deduplication
func (td *TranscodeDeduplicator) RequestTranscode(req *TranscodeRequest) (*TranscodeResult, error) {
	transcodeHash := td.GenerateTranscodeHash(req)
	requestID := utils.GenerateUUID()

	td.logger.Debug("Requesting transcode",
		"requestID", requestID,
		"transcodeHash", transcodeHash,
		"mediaFileID", req.MediaFileID)

	// Check if transcode already exists in database
	if result, found := td.findExistingTranscode(transcodeHash); found {
		td.logger.Info("Found existing transcode",
			"transcodeHash", transcodeHash,
			"transcodeID", result.TranscodeID)

		// Update usage tracking
		td.updateTranscodeUsage(result.TranscodeID)

		return result, nil
	}

	// Check if transcode is currently being processed
	td.mu.Lock()
	if pending, exists := td.pendingRequests[transcodeHash]; exists {
		// Add to waiters list
		waiter := make(chan *TranscodeResult, 1)
		pending.Waiters = append(pending.Waiters, waiter)
		td.mu.Unlock()

		td.logger.Info("Transcode already in progress, waiting",
			"transcodeHash", transcodeHash,
			"waiters", len(pending.Waiters))

		// Wait for completion
		select {
		case result := <-waiter:
			return result, nil
		case <-time.After(10 * time.Minute): // Timeout after 10 minutes
			return nil, fmt.Errorf("transcode request timed out")
		}
	}

	// Create new pending request
	pending := &PendingRequest{
		RequestID:     requestID,
		TranscodeHash: transcodeHash,
		RequestTime:   time.Now(),
		Waiters:       make([]chan *TranscodeResult, 0),
		InProgress:    true,
	}
	td.pendingRequests[transcodeHash] = pending
	td.mu.Unlock()

	// Start transcode in background
	go td.performTranscode(req, pending)

	// For the original requester, wait for completion
	waiter := make(chan *TranscodeResult, 1)
	td.mu.Lock()
	pending.Waiters = append(pending.Waiters, waiter)
	td.mu.Unlock()

	select {
	case result := <-waiter:
		return result, nil
	case <-time.After(10 * time.Minute):
		return nil, fmt.Errorf("transcode request timed out")
	}
}

// findExistingTranscode looks for an existing transcode with the same hash
func (td *TranscodeDeduplicator) findExistingTranscode(transcodeHash string) (*TranscodeResult, bool) {
	// Check memory cache first
	td.mu.RLock()
	if cached, exists := td.completedCache[transcodeHash]; exists {
		// Check if cache entry is still valid
		if time.Since(cached.CreatedAt) < td.cacheMaxAge {
			td.mu.RUnlock()
			return cached, true
		}
		// Remove expired entry
		delete(td.completedCache, transcodeHash)
	}
	td.mu.RUnlock()

	// Check database
	var dbRecord models.TranscodeCache
	err := td.db.Where("transcode_hash = ? AND status = ?", transcodeHash, "completed").
		First(&dbRecord).Error

	if err == nil {
		// Found in database, add to cache
		result := &TranscodeResult{
			TranscodeID: dbRecord.TranscodeID,
			OutputPath:  dbRecord.OutputPath,
			FileSize:    dbRecord.FileSize,
			Duration:    dbRecord.Duration,
			WasDeduped:  true,
			CreatedAt:   dbRecord.CreatedAt,
		}

		td.addToCache(transcodeHash, result)
		return result, true
	}

	return nil, false
}

// performTranscode executes the actual transcoding operation
func (td *TranscodeDeduplicator) performTranscode(req *TranscodeRequest, pending *PendingRequest) {
	defer func() {
		// Clean up pending request
		td.mu.Lock()
		delete(td.pendingRequests, pending.TranscodeHash)
		td.mu.Unlock()
	}()

	td.logger.Info("Starting transcode",
		"requestID", pending.RequestID,
		"transcodeHash", pending.TranscodeHash,
		"mediaFileID", req.MediaFileID)

	// Create database record for tracking
	transcodeID := utils.GenerateUUID()
	dbRecord := models.TranscodeCache{
		ID:            utils.GenerateUUID(),
		TranscodeID:   transcodeID,
		TranscodeHash: pending.TranscodeHash,
		MediaFileID:   req.MediaFileID,
		InputPath:     req.InputPath,
		OutputPath:    "", // Will be set after transcode
		Container:     req.Container,
		VideoCodec:    req.VideoCodec,
		AudioCodec:    req.AudioCodec,
		VideoBitrate:  req.VideoBitrate,
		AudioBitrate:  req.AudioBitrate,
		Status:        "pending",
		RequestTime:   req.RequestTime,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if req.Resolution != nil {
		dbRecord.ResolutionWidth = req.Resolution.Width
		dbRecord.ResolutionHeight = req.Resolution.Height
	}

	if err := td.db.Create(&dbRecord).Error; err != nil {
		td.logger.Error("Failed to create transcode record", "error", err)
		td.notifyWaiters(pending, nil, fmt.Errorf("failed to create transcode record: %w", err))
		return
	}

	// Simulate transcode operation
	// In real implementation, this would call the transcoding service
	outputPath := fmt.Sprintf("/app/viewra-data/transcodes/%s.%s", transcodeID, req.Container)

	// Update record with output path
	dbRecord.OutputPath = outputPath
	dbRecord.Status = "in_progress"
	td.db.Save(&dbRecord)

	// Simulate transcoding time (remove in real implementation)
	time.Sleep(100 * time.Millisecond)

	// Simulate successful completion
	dbRecord.Status = "completed"
	dbRecord.FileSize = 50 * 1024 * 1024 // 50MB example
	dbRecord.Duration = 3600             // 1 hour example
	now := time.Now()
	dbRecord.CompletedAt = &now

	if err := td.db.Save(&dbRecord).Error; err != nil {
		td.logger.Error("Failed to update transcode record", "error", err)
		td.notifyWaiters(pending, nil, fmt.Errorf("failed to update transcode record: %w", err))
		return
	}

	// Create result
	result := &TranscodeResult{
		TranscodeID:       transcodeID,
		OutputPath:        outputPath,
		FileSize:          dbRecord.FileSize,
		Duration:          dbRecord.Duration,
		WasDeduped:        false,
		OriginalRequestID: pending.RequestID,
		CreatedAt:         dbRecord.CreatedAt,
	}

	// Add to cache
	td.addToCache(pending.TranscodeHash, result)

	td.logger.Info("Transcode completed",
		"transcodeID", transcodeID,
		"transcodeHash", pending.TranscodeHash,
		"outputPath", outputPath)

	// Notify all waiters
	td.notifyWaiters(pending, result, nil)
}

// notifyWaiters notifies all waiting requests about completion
func (td *TranscodeDeduplicator) notifyWaiters(pending *PendingRequest, result *TranscodeResult, err error) {
	td.mu.Lock()
	waiters := pending.Waiters
	td.mu.Unlock()

	for _, waiter := range waiters {
		if err != nil {
			close(waiter) // Signal error by closing channel
		} else {
			select {
			case waiter <- result:
			default:
				// Waiter may have timed out, don't block
			}
			close(waiter)
		}
	}
}

// addToCache adds a result to the in-memory cache
func (td *TranscodeDeduplicator) addToCache(transcodeHash string, result *TranscodeResult) {
	td.mu.Lock()
	defer td.mu.Unlock()

	// Enforce cache size limit
	if len(td.completedCache) >= td.cacheMaxSize {
		// Remove oldest entries (simple LRU approximation)
		oldestTime := time.Now()
		var oldestHash string

		for hash, cached := range td.completedCache {
			if cached.CreatedAt.Before(oldestTime) {
				oldestTime = cached.CreatedAt
				oldestHash = hash
			}
		}

		if oldestHash != "" {
			delete(td.completedCache, oldestHash)
		}
	}

	td.completedCache[transcodeHash] = result
}

// updateTranscodeUsage updates the last used time for cache management
func (td *TranscodeDeduplicator) updateTranscodeUsage(transcodeID string) {
	td.db.Model(&models.TranscodeCache{}).
		Where("transcode_id = ?", transcodeID).
		Update("last_used", time.Now())
}

// GetDeduplicationStats returns statistics about deduplication
func (td *TranscodeDeduplicator) GetDeduplicationStats() map[string]interface{} {
	td.mu.RLock()
	defer td.mu.RUnlock()

	stats := map[string]interface{}{
		"pending_requests": len(td.pendingRequests),
		"cached_results":   len(td.completedCache),
		"cache_max_size":   td.cacheMaxSize,
		"cache_max_age":    td.cacheMaxAge,
	}

	// Get database stats
	var totalTranscodes int64
	td.db.Model(&models.TranscodeCache{}).Count(&totalTranscodes)
	stats["total_transcodes"] = totalTranscodes

	var completedTranscodes int64
	td.db.Model(&models.TranscodeCache{}).
		Where("status = ?", "completed").
		Count(&completedTranscodes)
	stats["completed_transcodes"] = completedTranscodes

	var pendingTranscodes int64
	td.db.Model(&models.TranscodeCache{}).
		Where("status IN ?", []string{"pending", "in_progress"}).
		Count(&pendingTranscodes)
	stats["pending_transcodes"] = pendingTranscodes

	// Calculate deduplication ratio
	if totalTranscodes > 0 {
		var uniqueHashes int64
		td.db.Model(&models.TranscodeCache{}).
			Select("COUNT(DISTINCT transcode_hash)").
			Scan(&uniqueHashes)

		if uniqueHashes > 0 {
			dedupeRatio := float64(totalTranscodes-uniqueHashes) / float64(totalTranscodes) * 100
			stats["deduplication_ratio"] = fmt.Sprintf("%.1f%%", dedupeRatio)
		}
	}

	return stats
}

// CleanupFailedTranscodes removes failed/stale transcode records
func (td *TranscodeDeduplicator) CleanupFailedTranscodes() error {
	// Remove records that have been pending for too long (1 hour)
	staleThreshold := time.Now().Add(-1 * time.Hour)

	result := td.db.Where("status IN ? AND created_at < ?",
		[]string{"pending", "in_progress"}, staleThreshold).
		Delete(&models.TranscodeCache{})

	if result.Error != nil {
		return fmt.Errorf("failed to cleanup failed transcodes: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		td.logger.Info("Cleaned up stale transcode records",
			"count", result.RowsAffected)
	}

	return nil
}
