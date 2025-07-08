// Package pipeline provides transcoding pipeline implementations.
// The file pipeline handles complete file transcoding operations.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/session"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/storage"
	"github.com/mantonx/viewra/internal/utils"
	tErrors "github.com/mantonx/viewra/internal/modules/transcodingmodule/errors"
	plugins "github.com/mantonx/viewra/sdk"
)

// FilePipeline implements file-based transcoding for complete media files.
// It provides a straightforward approach to transcoding where the entire
// file is processed and stored using content-addressable storage.
type FilePipeline struct {
	logger       hclog.Logger
	sessionStore *session.SessionStore
	contentStore *storage.ContentStore
	baseDir      string
}

// NewFilePipeline creates a new file-based transcoding pipeline.
//
// Parameters:
//   - logger: Logger instance for operational logging
//   - sessionStore: Store for managing transcoding sessions
//   - contentStore: Content-addressable storage for output files
//   - baseDir: Base directory for temporary transcoding files
func NewFilePipeline(logger hclog.Logger, sessionStore *session.SessionStore, contentStore *storage.ContentStore, baseDir string) *FilePipeline {
	return &FilePipeline{
		logger:       logger,
		sessionStore: sessionStore,
		contentStore: contentStore,
		baseDir:      baseDir,
	}
}

// Transcode initiates a file-based transcoding operation.
// It creates a new session, checks for existing transcoded content,
// and starts the transcoding process if needed.
//
// The method implements content deduplication by checking if the same
// content has already been transcoded with the requested parameters.
func (p *FilePipeline) Transcode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	// Generate session ID
	sessionID := generateSessionID()
	outputDir := filepath.Join(p.baseDir, "sessions", sessionID)
	
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, tErrors.StorageError("create_output_dir", err).
			WithDetail("dir", outputDir)
	}
	
	// Generate content hash for deduplication
	contentHash := generateContentHash(req.InputPath, req.MediaID, req.Container)
	
	// Check if content already exists
	if p.contentStore.Exists(contentHash) {
		p.logger.Info("Content already transcoded, reusing", "hash", contentHash)
		
		// Create session pointing to existing content
		sess, err := p.sessionStore.CreateSession("file-pipeline", &req)
		if err != nil {
			return nil, tErrors.SessionError("create_session", err).
				WithDetail("content_hash", contentHash)
		}
		
		// Mark as completed since content exists
		if err := p.sessionStore.UpdateSessionStatus(sess.ID, "completed", ""); err != nil {
			return nil, tErrors.SessionError("update_session_status", err).
				WithSession(sess.ID)
		}
		
		return &plugins.TranscodeHandle{
		SessionID: sess.ID,
		Provider:  "file-pipeline",
		Status:    "completed",
		}, nil
	}
	
	// Create new session
	sess, err := p.sessionStore.CreateSession("file-pipeline", &req)
	if err != nil {
		return nil, tErrors.SessionError("create_session", err).
			WithDetail("content_hash", contentHash)
	}
	
	// Store session ID for background processing
	sessionID = sess.ID
	
	// Start transcoding in background
	go p.runTranscode(ctx, sess.ID, req, outputDir, contentHash)
	
	return &plugins.TranscodeHandle{
		SessionID: sess.ID,
		Provider:  "file-pipeline",
		Status:    "running",
	}, nil
}

// runTranscode performs the actual transcoding operation in a goroutine.
// It executes FFmpeg with the appropriate parameters and moves the
// completed file to content-addressable storage.
//
// This method handles:
//   - FFmpeg execution with proper context cancellation
//   - Error recovery with panic handling
//   - Moving output to content-addressable storage
//   - Session status updates throughout the process
func (p *FilePipeline) runTranscode(ctx context.Context, sessionID string, req plugins.TranscodeRequest, outputDir string, contentHash string) {
	// Comprehensive panic recovery
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("Transcode panic", "error", r, "session_id", sessionID)
			
			// Convert panic to error
			var panicErr error
			if err, ok := r.(error); ok {
				panicErr = err
			} else {
				panicErr = fmt.Errorf("panic: %v", r)
			}
			
			// Update session with structured error
			tErr := tErrors.TranscodeError("transcode_panic", panicErr).
				WithSession(sessionID)
			p.sessionStore.UpdateSessionStatus(sessionID, "failed", tErr.Error())
		}
	}()
	
	// Determine output file
	outputFile := filepath.Join(outputDir, "output.mp4")
	if req.Container == "mkv" {
		outputFile = filepath.Join(outputDir, "output.mkv")
	}
	
	// Build FFmpeg command
	args := p.buildFFmpegArgs(req, outputFile)
	
	p.logger.Info("Starting transcode", "input", req.InputPath, "output", outputFile)
	
	// Run FFmpeg
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Dir = outputDir
	
	// Capture output for debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		tErr := tErrors.TranscodeError("ffmpeg_execution", err).
			WithSession(sessionID).
			WithDetail("input", req.InputPath).
			WithDetail("output", outputFile).
			WithDetail("ffmpeg_output", string(output))
		p.logger.Error("Transcode failed", 
			"error", tErr,
			"session_id", sessionID)
		p.sessionStore.UpdateSessionStatus(sessionID, "failed", tErr.Error())
		return
	}
	
	// Store content in content store
	metadata := storage.ContentMetadata{
		Hash:      contentHash,
		MediaID:   req.MediaID,
		Format:    req.Container,
		CreatedAt: time.Now(),
	}
	
	if err := p.contentStore.Store(contentHash, outputDir, metadata); err != nil {
		tErr := tErrors.StorageError("store_content", err).
			WithSession(sessionID).
			WithDetail("content_hash", contentHash)
		p.logger.Error("Failed to store content", 
			"error", tErr,
			"session_id", sessionID)
		p.sessionStore.UpdateSessionStatus(sessionID, "failed", tErr.Error())
		return
	}
	
	// Content has been stored, clean up temp directory
	if err := os.RemoveAll(outputDir); err != nil {
		// Log but don't fail - content is already stored
		p.logger.Warn("Failed to clean up temp directory",
			"dir", outputDir,
			"error", err,
			"session_id", sessionID)
	}
	
	// Update session
	p.sessionStore.UpdateSessionStatus(sessionID, "completed", "")
	p.logger.Info("Transcode completed", "session", sessionID, "content", contentHash)
}

// buildFFmpegArgs constructs the FFmpeg command line arguments based on the transcoding request.
//
// The method provides sensible defaults:
//   - Video codec: H.264 (libx264) if not specified
//   - Audio codec: AAC if not specified
//   - Quality preset: "fast" for balanced speed/quality
//   - Resolution scaling maintains aspect ratio (-2 for even dimensions)
func (p *FilePipeline) buildFFmpegArgs(req plugins.TranscodeRequest, outputFile string) []string {
	args := []string{
		"-i", req.InputPath,
		"-y", // Overwrite output
	}
	
	// Video codec
	if req.VideoCodec != "" {
		args = append(args, "-c:v", req.VideoCodec)
	} else {
		args = append(args, "-c:v", "libx264")
	}
	
	// Audio codec
	if req.AudioCodec != "" {
		args = append(args, "-c:a", req.AudioCodec)
	} else {
		args = append(args, "-c:a", "aac")
	}
	
	// Resolution
	if req.Resolution != nil {
		if req.Resolution.Height == 1080 {
			args = append(args, "-vf", "scale=-2:1080")
		} else if req.Resolution.Height == 720 {
			args = append(args, "-vf", "scale=-2:720")
		} else if req.Resolution.Height == 480 {
			args = append(args, "-vf", "scale=-2:480")
		} else {
			args = append(args, "-vf", fmt.Sprintf("scale=-2:%d", req.Resolution.Height))
		}
	}
	
	// Bitrate
	if req.VideoBitrate > 0 {
		args = append(args, "-b:v", fmt.Sprintf("%dk", req.VideoBitrate/1000))
	}
	
	if req.AudioBitrate > 0 {
		args = append(args, "-b:a", fmt.Sprintf("%dk", req.AudioBitrate/1000))
	}
	
	// Quality preset
	args = append(args, "-preset", "fast")
	
	// Output file
	args = append(args, outputFile)
	
	return args
}

// GetProgress retrieves the current progress of a transcoding session.
//
// For file-based transcoding, progress is estimated based on status:
//   - completed: 100% (1.0)
//   - running: 50% (0.5) - estimated as we don't parse FFmpeg output yet
//   - other states: 0%
//
// TODO: Implement real-time progress by parsing FFmpeg stderr output
func (p *FilePipeline) GetProgress(sessionID string) (*plugins.TranscodingProgress, error) {
	sess, err := p.sessionStore.GetSession(sessionID)
	if err != nil {
		return nil, tErrors.SessionError("get_progress", err).
			WithSession(sessionID)
	}
	
	// For file pipeline, we return status-based progress
	progress := &plugins.TranscodingProgress{
		PercentComplete: 0.0,
		TimeElapsed:     time.Since(sess.CreatedAt),
		CurrentSpeed:    0.0,
		AverageSpeed:    0.0,
	}
	
	if sess.Status == "completed" {
		progress.PercentComplete = 1.0
	} else if sess.Status == "running" {
		progress.PercentComplete = 0.5 // Estimate
	}
	
	return progress, nil
}

// Stop cancels an active transcoding session.
//
// Currently this only updates the database status. Future improvements:
//   - Send SIGTERM to FFmpeg process for graceful shutdown
//   - Clean up temporary files
//   - Wait for process to actually terminate
func (p *FilePipeline) Stop(sessionID string) error {
	// TODO: Implement process termination
	if err := p.sessionStore.UpdateSessionStatus(sessionID, "cancelled", "User cancelled"); err != nil {
		return tErrors.SessionError("stop_transcode", err).
			WithSession(sessionID)
	}
	return nil
}

// generateSessionID creates a unique session identifier
func generateSessionID() string {
	return uuid.New().String()
}

// generateContentHash creates a deterministic hash for content deduplication
func generateContentHash(inputPath, mediaID, container string) string {
	// Use input path as fallback if mediaID is empty
	mediaIdentifier := mediaID
	if mediaIdentifier == "" {
		mediaIdentifier = inputPath
	}
	
	// Use the centralized hash utility for consistency
	// TODO: Add quality and resolution parameters when available in the request
	return utils.GenerateContentHash(mediaIdentifier, container, 0, nil)
}