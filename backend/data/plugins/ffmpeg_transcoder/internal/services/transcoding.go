package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/config"
)

// DefaultTranscodingService implements the TranscodingService interface
type DefaultTranscodingService struct {
	config   *config.Config
	executor FFmpegExecutor
	jobs     map[string]*TranscodingJob
	mutex    sync.RWMutex
	stats    SystemStats
}

// NewTranscodingService creates a new transcoding service
func NewTranscodingService(cfg *config.Config) *DefaultTranscodingService {
	return &DefaultTranscodingService{
		config:   cfg,
		executor: NewFFmpegExecutor(cfg),
		jobs:     make(map[string]*TranscodingJob),
		stats:    SystemStats{},
	}
}

// StartJob starts a new transcoding job
func (s *DefaultTranscodingService) StartJob(ctx context.Context, request *TranscodingRequest) (*TranscodingResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check concurrent job limit
	activeCount := s.getActiveJobsCount()
	if activeCount >= s.config.GetMaxConcurrentSessions() {
		return &TranscodingResponse{
			Status:    StatusQueued,
			Message:   fmt.Sprintf("Maximum concurrent jobs (%d) reached, job queued", s.config.GetMaxConcurrentSessions()),
			QueueSize: activeCount + 1,
		}, nil
	}

	// Generate job ID
	jobID := generateJobID()

	// Create job
	job := &TranscodingJob{
		ID:         jobID,
		Status:     StatusPending,
		InputFile:  request.InputFile,
		OutputFile: request.OutputFile,
		Settings:   request.Settings,
		StartTime:  time.Now(),
		Progress: Progress{
			LastUpdate: time.Now(),
		},
	}

	// Create a cancelable context for the job
	jobCtx, cancelFunc := context.WithCancel(context.Background())
	job.CancelFunc = cancelFunc

	// Store job
	s.jobs[jobID] = job

	// Start transcoding in background with the cancelable context
	go s.executeTranscoding(jobCtx, job)

	return &TranscodingResponse{
		JobID:   jobID,
		Status:  StatusProcessing,
		Message: "Transcoding job started",
	}, nil
}

// StopJob stops a running transcoding job
func (s *DefaultTranscodingService) StopJob(ctx context.Context, jobID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	if job.Status != StatusProcessing {
		return fmt.Errorf("job %s is not running (status: %s)", jobID, job.Status)
	}

	// Debug log
	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "🛑 [%s] Stopping job %s\n", time.Now().Format("15:04:05"), jobID)
		debugFile.Close()
	}

	// Cancel the job context if available
	if job.CancelFunc != nil {
		job.CancelFunc()
	}

	// Update status
	job.Status = StatusCancelled
	now := time.Now()
	job.EndTime = &now

	return nil
}

// GetJobStatus returns the status of a job
func (s *DefaultTranscodingService) GetJobStatus(ctx context.Context, jobID string) (*TranscodingJob, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	// Return a copy to avoid concurrent modification
	jobCopy := *job
	return &jobCopy, nil
}

// GetSystemStats returns current system statistics
func (s *DefaultTranscodingService) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	stats := s.stats
	stats.ActiveJobs = s.getActiveJobsCount()
	stats.QueuedJobs = s.getQueuedJobsCount()
	stats.TotalJobs = len(s.jobs)

	// Count completed and failed jobs
	for _, job := range s.jobs {
		switch job.Status {
		case StatusCompleted:
			stats.CompletedJobs++
		case StatusFailed:
			stats.FailedJobs++
		}
	}

	return &stats, nil
}

// CleanupJobs removes old completed jobs
func (s *DefaultTranscodingService) CleanupJobs(ctx context.Context, olderThan int) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	cutoff := time.Now().Add(-time.Duration(olderThan) * time.Hour)
	removedCount := 0

	for jobID, job := range s.jobs {
		if job.EndTime != nil && job.EndTime.Before(cutoff) &&
			(job.Status == StatusCompleted || job.Status == StatusFailed || job.Status == StatusCancelled) {

			// Clean up output file if it exists
			if job.OutputFile != "" {
				os.Remove(job.OutputFile)
			}

			delete(s.jobs, jobID)
			removedCount++
		}
	}

	return removedCount, nil
}

// executeTranscoding runs the actual transcoding process
func (s *DefaultTranscodingService) executeTranscoding(ctx context.Context, job *TranscodingJob) {
	// Write debug info to log file
	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "🚀 [%s] Starting executeTranscoding for job %s\n", time.Now().Format("15:04:05"), job.ID)
		debugFile.Close()
	}

	// Update status to processing
	s.mutex.Lock()
	job.Status = StatusProcessing
	s.mutex.Unlock()

	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "📝 [%s] Updated job %s status to processing\n", time.Now().Format("15:04:05"), job.ID)
		debugFile.Close()
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(job.OutputFile)
	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "📁 [%s] Creating output directory: %s\n", time.Now().Format("15:04:05"), outputDir)
		debugFile.Close()
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
			fmt.Fprintf(debugFile, "❌ [%s] Failed to create output directory: %v\n", time.Now().Format("15:04:05"), err)
			debugFile.Close()
		}
		s.completeJobWithError(job, fmt.Errorf("failed to create output directory: %w", err))
		return
	}

	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "✅ [%s] Output directory created successfully\n", time.Now().Format("15:04:05"))
		debugFile.Close()
	}

	// Build FFmpeg arguments
	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "🔧 [%s] Building FFmpeg arguments for job %s\n", time.Now().Format("15:04:05"), job.ID)
		debugFile.Close()
	}

	args, err := s.buildFFmpegArgs(job)
	if err != nil {
		if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
			fmt.Fprintf(debugFile, "❌ [%s] Failed to build FFmpeg args: %v\n", time.Now().Format("15:04:05"), err)
			debugFile.Close()
		}
		s.completeJobWithError(job, fmt.Errorf("failed to build FFmpeg args: %w", err))
		return
	}

	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "✅ [%s] FFmpeg arguments built successfully: %v\n", time.Now().Format("15:04:05"), args)
		debugFile.Close()
	}

	// Progress callback
	progressCallback := func(jobID string, progress *Progress) {
		s.mutex.Lock()
		if currentJob, exists := s.jobs[jobID]; exists {
			currentJob.Progress = *progress
		}
		s.mutex.Unlock()
	}

	// Execute FFmpeg
	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "🎬 [%s] About to execute FFmpeg for job %s\n", time.Now().Format("15:04:05"), job.ID)
		debugFile.Close()
	}

	err = s.executor.Execute(ctx, args, progressCallback)

	// Check if useful output was produced, even if FFmpeg had an error
	manifestExists := false
	if _, statErr := os.Stat(job.OutputFile); statErr == nil {
		manifestExists = true
		if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
			fmt.Fprintf(debugFile, "✅ [%s] Manifest file exists for job %s: %s\n", time.Now().Format("15:04:05"), job.ID, job.OutputFile)
			debugFile.Close()
		}
	}

	if err != nil {
		if manifestExists {
			// FFmpeg had an error but produced usable output - treat as success with warning
			if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
				fmt.Fprintf(debugFile, "⚠️ [%s] FFmpeg had error but produced manifest for job %s: %v\n", time.Now().Format("15:04:05"), job.ID, err)
				debugFile.Close()
			}

			// Mark as completed with warning
			s.mutex.Lock()
			job.Status = StatusCompleted
			now := time.Now()
			job.EndTime = &now
			job.Progress.Percentage = 100.0
			job.Error = fmt.Sprintf("Completed with warning: %v", err)
			s.mutex.Unlock()

			if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
				fmt.Fprintf(debugFile, "🎉 [%s] Transcoding completed with warning for job %s\n", time.Now().Format("15:04:05"), job.ID)
				debugFile.Close()
			}
			return
		} else {
			// FFmpeg failed and no useful output - mark as failed
			if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
				fmt.Fprintf(debugFile, "❌ [%s] FFmpeg execution failed for job %s (no manifest): %v\n", time.Now().Format("15:04:05"), job.ID, err)
				debugFile.Close()
			}
			s.completeJobWithError(job, fmt.Errorf("transcoding failed: %w", err))
			return
		}
	}

	// Complete successfully (no error at all)
	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "🎉 [%s] Transcoding completed successfully for job %s\n", time.Now().Format("15:04:05"), job.ID)
		debugFile.Close()
	}

	s.mutex.Lock()
	job.Status = StatusCompleted
	now := time.Now()
	job.EndTime = &now
	job.Progress.Percentage = 100.0
	s.mutex.Unlock()
}

// buildFFmpegArgs builds the FFmpeg command line arguments
func (s *DefaultTranscodingService) buildFFmpegArgs(job *TranscodingJob) ([]string, error) {
	var args []string

	// Always overwrite output files (must come first)
	args = append(args, "-y")

	// Always use automatic hardware acceleration (let FFmpeg choose)
	// This automatically detects and uses the best available hardware acceleration
	// (NVENC, VAAPI, QSV, VideoToolbox) without manual configuration
	args = append(args, "-hwaccel", "auto")

	// Input file
	args = append(args, "-i", job.InputFile)

	// Use the video codec as requested (no hardware-specific codec overrides)
	videoCodec := job.Settings.VideoCodec

	// Common video encoding settings
	args = append(args,
		"-c:v", videoCodec,
		"-preset", job.Settings.Preset,
		"-crf", strconv.Itoa(job.Settings.Quality),
	)

	// Audio encoding settings
	args = append(args,
		"-c:a", job.Settings.AudioCodec,
		"-b:a", fmt.Sprintf("%dk", job.Settings.AudioBitrate),
	)

	// Container-specific settings
	switch job.Settings.Container {
	case "dash":
		// DASH-specific arguments
		args = append(args,
			"-f", "dash",
			"-seg_duration", "4",
			"-use_template", "1",
			"-use_timeline", "1",
			"-init_seg_name", "init-$RepresentationID$.m4s",
			"-media_seg_name", "chunk-$RepresentationID$-$Number$.m4s",
			"-adaptation_sets", "id=0,streams=v id=1,streams=a",
			"-map", "0:v:0",
			"-map", "0:a:0",
			job.OutputFile, // This should be the manifest.mpd path
		)

	case "hls":
		// HLS-specific arguments
		outputDir := filepath.Dir(job.OutputFile)
		args = append(args,
			"-f", "hls",
			"-hls_time", "4",
			"-hls_playlist_type", "vod",
			"-hls_segment_filename", filepath.Join(outputDir, "segment_%03d.ts"),
			"-map", "0:v:0",
			"-map", "0:a:0",
			job.OutputFile, // This should be the playlist.m3u8 path
		)

	default:
		// Progressive MP4 (default)
		args = append(args,
			"-f", "mp4",
			"-movflags", "+faststart",
			"-map", "0:v:0?",
			"-map", "0:a:0?",
			job.OutputFile,
		)
	}

	return args, nil
}

// completeJobWithError marks a job as failed with an error
func (s *DefaultTranscodingService) completeJobWithError(job *TranscodingJob, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	job.Status = StatusFailed
	job.Error = err.Error()
	now := time.Now()
	job.EndTime = &now
}

// getActiveJobsCount returns the number of active jobs (must be called with lock held)
func (s *DefaultTranscodingService) getActiveJobsCount() int {
	count := 0
	for _, job := range s.jobs {
		if job.Status == StatusProcessing {
			count++
		}
	}
	return count
}

// getQueuedJobsCount returns the number of queued jobs (must be called with lock held)
func (s *DefaultTranscodingService) getQueuedJobsCount() int {
	count := 0
	for _, job := range s.jobs {
		if job.Status == StatusQueued {
			count++
		}
	}
	return count
}

// generateJobID generates a unique job ID
func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

// FFmpegExecutorImpl implements the FFmpegExecutor interface
type FFmpegExecutorImpl struct {
	config *config.Config
}

// NewFFmpegExecutor creates a new FFmpeg executor
func NewFFmpegExecutor(cfg *config.Config) FFmpegExecutor {
	return &FFmpegExecutorImpl{
		config: cfg,
	}
}

// Execute runs an FFmpeg command with progress monitoring
func (e *FFmpegExecutorImpl) Execute(ctx context.Context, args []string, progressCallback ProgressCallback) error {
	cmd := exec.CommandContext(ctx, e.config.GetFFmpegPath(), args...)

	// Write to debug log
	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "🎬 [%s] Executing FFmpeg (PID will be %d): %s %s\n", time.Now().Format("15:04:05"), 0, e.config.GetFFmpegPath(), strings.Join(args, " "))
		debugFile.Close()
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
			fmt.Fprintf(debugFile, "❌ [%s] Failed to start FFmpeg: %v\n", time.Now().Format("15:04:05"), err)
			debugFile.Close()
		}
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Log the actual PID
	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "🎯 [%s] FFmpeg started with PID: %d\n", time.Now().Format("15:04:05"), cmd.Process.Pid)
		debugFile.Close()
	}

	// Set up cleanup for when context is canceled
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
				fmt.Fprintf(debugFile, "🔪 [%s] Context canceled, killing FFmpeg PID: %d\n", time.Now().Format("15:04:05"), cmd.Process.Pid)
				debugFile.Close()
			}

			// First try graceful termination
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				// If graceful fails, force kill
				cmd.Process.Kill()
			} else {
				// Give it 2 seconds to terminate gracefully, then force kill
				go func() {
					time.Sleep(2 * time.Second)
					if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
						cmd.Process.Kill()
						if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
							fmt.Fprintf(debugFile, "🔥 [%s] Force killed FFmpeg PID: %d after graceful timeout\n", time.Now().Format("15:04:05"), cmd.Process.Pid)
							debugFile.Close()
						}
					}
				}()
			}
		}
	}()

	// Wait for completion
	err := cmd.Wait()
	if err != nil {
		// Check if it was killed due to context cancellation
		if ctx.Err() != nil {
			if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
				fmt.Fprintf(debugFile, "⚡ [%s] FFmpeg process canceled/killed (PID: %d)\n", time.Now().Format("15:04:05"), cmd.Process.Pid)
				debugFile.Close()
			}
			return fmt.Errorf("FFmpeg process canceled: %w", ctx.Err())
		}

		if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
			fmt.Fprintf(debugFile, "⚠️ [%s] FFmpeg execution had exit code (PID: %d): %v\n", time.Now().Format("15:04:05"), cmd.Process.Pid, err)
			debugFile.Close()
		}

		// Don't immediately fail - FFmpeg might have produced useful output despite the error
		// We'll check for actual output in the calling function
		return fmt.Errorf("FFmpeg execution completed with error: %w", err)
	}

	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "✅ [%s] FFmpeg execution completed successfully (PID: %d)\n", time.Now().Format("15:04:05"), cmd.Process.Pid)
		debugFile.Close()
	}
	return nil
}

// GetVersion returns the FFmpeg version
func (e *FFmpegExecutorImpl) GetVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, e.config.GetFFmpegPath(), "-version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get FFmpeg version: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		return lines[0], nil
	}

	return "unknown", nil
}

// ProbeFile probes a media file for format information
func (e *FFmpegExecutorImpl) ProbeFile(ctx context.Context, filename string) (*FormatInfo, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filename,
	)

	_, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to probe file: %w", err)
	}

	// For now, return basic info
	// In a real implementation, you'd parse the JSON output
	info := &FormatInfo{
		Container: filepath.Ext(filename),
		FileSize:  0, // Would get from file stat or probe output
		Metadata:  make(map[string]string),
	}

	return info, nil
}

// ValidateInstallation validates that FFmpeg is properly installed
func (e *FFmpegExecutorImpl) ValidateInstallation(ctx context.Context) error {
	_, err := e.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("FFmpeg not found or not working: %w", err)
	}
	return nil
}
