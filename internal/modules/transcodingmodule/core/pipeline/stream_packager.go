// Package pipeline provides streaming pipeline functionality for real-time transcoding.
// This file implements real-time Shaka packaging for progressive DASH/HLS delivery.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/events"
)

// StreamPackager handles real-time packaging of segments into DASH/HLS
type StreamPackager struct {
	shakaPath string
	outputDir string
	baseURL   string

	// Segment queue for processing
	segmentQueue chan SegmentInfo
	queueMutex   sync.Mutex

	// Manifest management
	manifestPath string
	manifestLock sync.RWMutex

	// Process management
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	// Callbacks
	onManifestUpdate  func(manifestPath string)
	onSegmentPackaged func(segmentPath string)
	onError           func(error)

	// Event system integration
	eventBus    *events.EventBus
	sessionID   string
	contentHash string
}

// SegmentInfo contains information about a segment to package
type SegmentInfo struct {
	Path      string
	Index     int
	Profile   string
	Type      string // "video" or "audio"
	Timestamp time.Time
}

// NewStreamPackager creates a new real-time packager
func NewStreamPackager(outputDir string, baseURL string) *StreamPackager {
	return &StreamPackager{
		shakaPath:    "shaka-packager", // TODO: Make configurable
		outputDir:    outputDir,
		baseURL:      baseURL,
		segmentQueue: make(chan SegmentInfo, 100), // Buffer for smooth processing
	}
}

// SetCallbacks sets event callbacks
func (p *StreamPackager) SetCallbacks(onManifest func(string), onSegment func(string), onError func(error)) {
	p.onManifestUpdate = onManifest
	p.onSegmentPackaged = onSegment
	p.onError = onError
}

// SetEventBus integrates with the segment event system
func (p *StreamPackager) SetEventBus(eventBus *events.EventBus, sessionID, contentHash string) {
	p.eventBus = eventBus
	p.sessionID = sessionID
	p.contentHash = contentHash
}

// Start begins the real-time packaging process
func (p *StreamPackager) Start(ctx context.Context) error {
	p.ctx, p.cancelFunc = context.WithCancel(ctx)

	// Ensure output directory exists
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create subdirectories
	for _, subdir := range []string{"init", "segments"} {
		if err := os.MkdirAll(filepath.Join(p.outputDir, subdir), 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", subdir, err)
		}
	}

	// Initialize manifest
	p.manifestPath = filepath.Join(p.outputDir, "stream.mpd")
	if err := p.initializeManifest(); err != nil {
		return fmt.Errorf("failed to initialize manifest: %w", err)
	}

	// Start segment processing workers
	workerCount := 4 // Process multiple segments in parallel
	for i := 0; i < workerCount; i++ {
		p.wg.Add(1)
		go p.segmentProcessor(i)
	}

	logger.Info("Stream packager started", "output", p.outputDir, "workers", workerCount)
	return nil
}

// Stop gracefully stops the packager
func (p *StreamPackager) Stop() error {
	if p.cancelFunc != nil {
		p.cancelFunc()
	}

	// Close segment queue
	close(p.segmentQueue)

	// Wait for workers to finish
	p.wg.Wait()

	// Finalize manifest
	if err := p.finalizeManifest(); err != nil {
		logger.Warn("Failed to finalize manifest", "error", err)
	}

	return nil
}

// QueueSegment adds a segment to the packaging queue
func (p *StreamPackager) QueueSegment(segment SegmentInfo) error {
	select {
	case p.segmentQueue <- segment:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("packager is stopping")
	default:
		return fmt.Errorf("segment queue is full")
	}
}

// segmentProcessor processes segments from the queue
func (p *StreamPackager) segmentProcessor(workerID int) {
	defer p.wg.Done()

	for {
		select {
		case segment, ok := <-p.segmentQueue:
			if !ok {
				return // Queue closed
			}

			if err := p.processSegment(segment); err != nil {
				logger.Error("Failed to process segment",
					"worker", workerID,
					"segment", segment.Index,
					"error", err)
				if p.onError != nil {
					p.onError(err)
				}
			}

		case <-p.ctx.Done():
			return
		}
	}
}

// processSegment packages a single segment
func (p *StreamPackager) processSegment(segment SegmentInfo) error {
	startTime := time.Now()

	// For now, skip Shaka Packager and just copy the segment to the output directory
	// The segments from FFmpeg are already in fragmented MP4 format suitable for DASH
	outputPath := filepath.Join(p.outputDir, fmt.Sprintf("segment_%03d.mp4", segment.Index))
	
	// Copy the segment file
	input, err := os.Open(segment.Path)
	if err != nil {
		return fmt.Errorf("failed to open segment: %w", err)
	}
	defer input.Close()
	
	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer output.Close()
	
	if _, err := output.ReadFrom(input); err != nil {
		return fmt.Errorf("failed to copy segment: %w", err)
	}

	// Create or update a simple DASH manifest
	if segment.Index == 0 {
		// Initialize manifest on first segment
		p.manifestPath = filepath.Join(p.outputDir, "manifest.mpd")
		if err := p.initializeManifest(); err != nil {
			return fmt.Errorf("failed to create manifest: %w", err)
		}
	}

	// Update manifest with new segment
	if err := p.updateManifest(segment, outputPath); err != nil {
		return fmt.Errorf("failed to update manifest: %w", err)
	}

	// Notify callbacks
	if p.onSegmentPackaged != nil {
		p.onSegmentPackaged(outputPath)
	}

	logger.Debug("Segment processed",
		"index", segment.Index,
		"type", segment.Type,
		"profile", segment.Profile,
		"duration", time.Since(startTime))

	return nil
}

// buildSegmentArgs constructs Shaka arguments for real-time segment packaging
func (p *StreamPackager) buildSegmentArgs(segment SegmentInfo, outputPath string) []string {
	// For streaming pipeline, we need to specify the output properly
	// Shaka Packager requires either 'output' or 'segment_template' to be specified
	args := []string{
		"in=" + segment.Path + ",stream=0,output=" + outputPath,
		"--fragment_duration", "4",
		"--segment_duration", "4",
	}

	// Only generate manifests on the first segment
	if segment.Index == 0 {
		args = append(args, 
			"--mpd_output", filepath.Join(p.outputDir, "manifest.mpd"),
			"--hls_master_playlist_output", filepath.Join(p.outputDir, "playlist.m3u8"),
		)
	}

	return args
}

// initializeManifest creates the initial DASH manifest
func (p *StreamPackager) initializeManifest() error {
	// Create minimal valid DASH manifest that can be updated
	initialManifest := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" 
     type="dynamic" 
     minimumUpdatePeriod="PT2S"
     availabilityStartTime="` + time.Now().UTC().Format(time.RFC3339) + `"
     minBufferTime="PT4S"
     profiles="urn:mpeg:dash:profile:isoff-live:2011">
  <Period id="0" start="PT0S">
    <!-- Representations will be added as segments arrive -->
  </Period>
</MPD>`

	return os.WriteFile(p.manifestPath, []byte(initialManifest), 0644)
}

// updateManifest dynamically updates the manifest with new segment information
func (p *StreamPackager) updateManifest(segment SegmentInfo, segmentPath string) error {
	p.manifestLock.Lock()
	defer p.manifestLock.Unlock()

	// Read current manifest
	manifestData, err := os.ReadFile(p.manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	// Parse and update manifest based on type
	var updatedManifest string
	if strings.Contains(string(manifestData), "xmlns=\"urn:mpeg:dash:schema:mpd:2011\"") {
		// DASH manifest
		updatedManifest, err = p.updateDashManifest(string(manifestData), segment, segmentPath)
	} else {
		// HLS manifest
		updatedManifest, err = p.updateHLSManifest(string(manifestData), segment, segmentPath)
	}

	if err != nil {
		return fmt.Errorf("failed to update manifest: %w", err)
	}

	// Write updated manifest
	if err := os.WriteFile(p.manifestPath, []byte(updatedManifest), 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Publish manifest updated event
	if p.eventBus != nil {
		p.eventBus.PublishManifestUpdated(p.sessionID, p.contentHash, p.manifestPath)
	}

	// Legacy callback support
	if p.onManifestUpdate != nil {
		p.onManifestUpdate(p.manifestPath)
	}

	return nil
}

// updateDashManifest updates a DASH manifest with new segment
func (p *StreamPackager) updateDashManifest(manifest string, segment SegmentInfo, segmentPath string) (string, error) {
	// For now, add the segment to the segment list
	// In a full implementation, this would properly parse XML and update the SegmentList
	relativePath := filepath.Base(segmentPath)
	segmentEntry := fmt.Sprintf(`<SegmentURL media="%s" />`, relativePath)

	// Find the right adaptation set and add the segment
	// This is a simplified implementation - production code would use proper XML parsing
	if strings.Contains(manifest, "</SegmentList>") {
		// Add before closing SegmentList tag
		manifest = strings.Replace(manifest, "</SegmentList>", segmentEntry+"\n    </SegmentList>", 1)
	} else {
		// Add segment list if it doesn't exist
		segmentList := fmt.Sprintf(`\n      <SegmentList>\n        %s\n      </SegmentList>`, segmentEntry)
		manifest = strings.Replace(manifest, "</Representation>", segmentList+"\n    </Representation>", 1)
	}

	// Update media presentation duration if this is a new segment
	estimatedDuration := time.Duration(segment.Index*4) * time.Second // 4 seconds per segment
	durationStr := fmt.Sprintf("PT%dS", int(estimatedDuration.Seconds()))
	manifest = strings.Replace(manifest, `mediaPresentationDuration="PT0S"`, fmt.Sprintf(`mediaPresentationDuration="%s"`, durationStr), 1)

	return manifest, nil
}

// updateHLSManifest updates an HLS manifest with new segment
func (p *StreamPackager) updateHLSManifest(manifest string, segment SegmentInfo, segmentPath string) (string, error) {
	// Add new segment to HLS playlist
	relativePath := filepath.Base(segmentPath)
	segmentEntry := fmt.Sprintf("#EXTINF:4.0,\n%s", relativePath)

	// Add segment before #EXT-X-ENDLIST (if it exists) or at the end
	if strings.Contains(manifest, "#EXT-X-ENDLIST") {
		manifest = strings.Replace(manifest, "#EXT-X-ENDLIST", segmentEntry+"\n#EXT-X-ENDLIST", 1)
	} else {
		manifest += "\n" + segmentEntry
	}

	// Update sequence number
	sequenceNum := segment.Index
	if strings.Contains(manifest, "#EXT-X-MEDIA-SEQUENCE:") {
		// Update existing sequence
		manifest = strings.Replace(manifest, "#EXT-X-MEDIA-SEQUENCE:0", fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d", sequenceNum), 1)
	} else {
		// Add sequence number
		manifest = strings.Replace(manifest, "#EXTM3U", fmt.Sprintf("#EXTM3U\n#EXT-X-MEDIA-SEQUENCE:%d", sequenceNum), 1)
	}

	return manifest, nil
}

// finalizeManifest converts the dynamic manifest to static when streaming is complete
func (p *StreamPackager) finalizeManifest() error {
	p.manifestLock.Lock()
	defer p.manifestLock.Unlock()

	// TODO: Convert type="dynamic" to type="static"
	// Update mediaPresentationDuration
	// Remove minimumUpdatePeriod

	logger.Info("Manifest finalized", "path", p.manifestPath)
	return nil
}

// GetManifestPath returns the current manifest path
func (p *StreamPackager) GetManifestPath() string {
	p.manifestLock.RLock()
	defer p.manifestLock.RUnlock()
	return p.manifestPath
}
