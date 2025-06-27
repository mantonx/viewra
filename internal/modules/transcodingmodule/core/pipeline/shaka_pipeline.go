// Package pipeline provides the Shaka Packager integration for proper DASH/HLS packaging.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

// ShakaPipeline handles FFmpeg to Shaka Packager pipeline for DASH/HLS packaging
type ShakaPipeline struct {
	ffmpegPath string
	shakaPath  string
	outputDir  string
	
	// Process management
	ffmpegCmd *exec.Cmd
	shakaCmd  *exec.Cmd
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	
	// Callbacks
	onProgress  func(progress float64)
	onComplete  func()
	onError     func(error)
	
	logger hclog.Logger
}

// NewShakaPipeline creates a new Shaka pipeline
func NewShakaPipeline(outputDir string) *ShakaPipeline {
	return &ShakaPipeline{
		ffmpegPath: "ffmpeg",
		shakaPath:  "shaka-packager",
		outputDir:  outputDir,
		logger:     hclog.New(&hclog.LoggerOptions{
			Name:  "shaka-pipeline",
			Level: hclog.Info,
		}),
	}
}

// TranscodeWithShaka performs a complete transcode with Shaka Packager for DASH/HLS
func (p *ShakaPipeline) TranscodeWithShaka(ctx context.Context, input string, container string) error {
	p.ctx, p.cancel = context.WithCancel(ctx)
	
	// Ensure output directory exists
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Create subdirectories for segments
	for _, dir := range []string{"video", "audio"} {
		if err := os.MkdirAll(filepath.Join(p.outputDir, dir), 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", dir, err)
		}
	}
	
	// Build FFmpeg args for encoding
	ffmpegArgs := p.buildFFmpegArgs(input)
	
	// Build Shaka args for packaging
	shakaArgs := p.buildShakaArgs(container)
	
	// Create FFmpeg command
	p.ffmpegCmd = exec.CommandContext(p.ctx, p.ffmpegPath, ffmpegArgs...)
	
	// Create Shaka command
	p.shakaCmd = exec.CommandContext(p.ctx, p.shakaPath, shakaArgs...)
	
	// Create pipe from FFmpeg to Shaka
	ffmpegStdout, err := p.ffmpegCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create FFmpeg stdout pipe: %w", err)
	}
	
	// Connect FFmpeg output to Shaka input
	p.shakaCmd.Stdin = ffmpegStdout
	
	// Capture Shaka output for logging
	p.shakaCmd.Stdout = os.Stdout
	p.shakaCmd.Stderr = os.Stderr
	
	// Start Shaka first (it will wait for input)
	if err := p.shakaCmd.Start(); err != nil {
		return fmt.Errorf("failed to start Shaka Packager: %w", err)
	}
	
	// Start FFmpeg
	if err := p.ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}
	
	p.logger.Info("Pipeline started",
		"ffmpeg_pid", p.ffmpegCmd.Process.Pid,
		"shaka_pid", p.shakaCmd.Process.Pid)
	
	// Monitor both processes
	p.wg.Add(2)
	
	// Monitor FFmpeg
	go func() {
		defer p.wg.Done()
		if err := p.ffmpegCmd.Wait(); err != nil {
			if p.ctx.Err() == nil {
				p.logger.Error("FFmpeg failed", "error", err)
				if p.onError != nil {
					p.onError(fmt.Errorf("FFmpeg failed: %w", err))
				}
			}
		} else {
			p.logger.Info("FFmpeg completed successfully")
		}
	}()
	
	// Monitor Shaka
	go func() {
		defer p.wg.Done()
		if err := p.shakaCmd.Wait(); err != nil {
			if p.ctx.Err() == nil {
				p.logger.Error("Shaka Packager failed", "error", err)
				if p.onError != nil {
					p.onError(fmt.Errorf("Shaka Packager failed: %w", err))
				}
			}
		} else {
			p.logger.Info("Shaka Packager completed successfully")
			if p.onComplete != nil {
				p.onComplete()
			}
		}
	}()
	
	return nil
}

// buildFFmpegArgs creates FFmpeg arguments for encoding to stdout
func (p *ShakaPipeline) buildFFmpegArgs(input string) []string {
	return []string{
		"-i", input,
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-f", "mp4",
		"-", // Output to stdout
	}
}

// buildShakaArgs creates Shaka Packager arguments for DASH/HLS packaging
func (p *ShakaPipeline) buildShakaArgs(container string) []string {
	args := []string{
		"in=-,stream=video,output=" + filepath.Join(p.outputDir, "video/stream.mp4"),
		"in=-,stream=audio,output=" + filepath.Join(p.outputDir, "audio/stream.mp4"),
		"--segment_duration", "4",
	}
	
	// Add container-specific options
	switch container {
	case "dash":
		args = append(args,
			"--mpd_output", filepath.Join(p.outputDir, "manifest.mpd"),
			"--generate_static_live_mpd",
		)
	case "hls":
		args = append(args,
			"--hls_master_playlist_output", filepath.Join(p.outputDir, "playlist.m3u8"),
		)
	default:
		// Default to DASH
		args = append(args,
			"--mpd_output", filepath.Join(p.outputDir, "manifest.mpd"),
		)
	}
	
	return args
}

// Stop stops the pipeline
func (p *ShakaPipeline) Stop() error {
	if p.cancel != nil {
		p.cancel()
	}
	
	// Wait for processes to finish
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-time.After(10 * time.Second):
		// Force kill if needed
		if p.ffmpegCmd != nil && p.ffmpegCmd.Process != nil {
			p.ffmpegCmd.Process.Kill()
		}
		if p.shakaCmd != nil && p.shakaCmd.Process != nil {
			p.shakaCmd.Process.Kill()
		}
		return fmt.Errorf("pipeline stop timeout")
	}
}

// SetCallbacks sets progress and completion callbacks
func (p *ShakaPipeline) SetCallbacks(onProgress func(float64), onComplete func(), onError func(error)) {
	p.onProgress = onProgress
	p.onComplete = onComplete
	p.onError = onError
}

// AlternativeApproach performs two-stage processing (for files that can't be piped)
func (p *ShakaPipeline) TwoStageTranscode(ctx context.Context, input string, container string) error {
	p.ctx, p.cancel = context.WithCancel(ctx)
	
	// Stage 1: Encode with FFmpeg to temporary file
	tempFile := filepath.Join(p.outputDir, "temp_encoded.mp4")
	
	ffmpegArgs := []string{
		"-i", input,
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		tempFile,
	}
	
	p.logger.Info("Stage 1: Encoding with FFmpeg")
	ffmpegCmd := exec.CommandContext(p.ctx, p.ffmpegPath, ffmpegArgs...)
	if output, err := ffmpegCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("FFmpeg encoding failed: %w\nOutput: %s", err, string(output))
	}
	
	// Stage 2: Package with Shaka
	p.logger.Info("Stage 2: Packaging with Shaka")
	
	shakaArgs := []string{
		"in=" + tempFile + ",stream=0,output=" + filepath.Join(p.outputDir, "video/stream.mp4"),
		"in=" + tempFile + ",stream=1,output=" + filepath.Join(p.outputDir, "audio/stream.mp4"),
		"--segment_duration", "4",
	}
	
	// Add manifest output
	if container == "dash" {
		shakaArgs = append(shakaArgs,
			"--mpd_output", filepath.Join(p.outputDir, "manifest.mpd"),
		)
	} else {
		shakaArgs = append(shakaArgs,
			"--hls_master_playlist_output", filepath.Join(p.outputDir, "playlist.m3u8"),
		)
	}
	
	shakaCmd := exec.CommandContext(p.ctx, p.shakaPath, shakaArgs...)
	if output, err := shakaCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Shaka packaging failed: %w\nOutput: %s", err, string(output))
	}
	
	// Clean up temp file
	os.Remove(tempFile)
	
	p.logger.Info("Two-stage transcoding completed successfully")
	if p.onComplete != nil {
		p.onComplete()
	}
	
	return nil
}