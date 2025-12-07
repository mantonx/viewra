package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	infraffmpeg "github.com/mantonx/viewra/internal/infrastructure/ffmpeg"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/process"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/config"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/videoinfo"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"

	"github.com/mantonx/viewra/internal/domain/transcode"
)

// errWithLongMessage is a helper type for testing long error messages
type errWithLongMessage struct {
	msg string
}

func (e errWithLongMessage) Error() string {
	return e.msg
}

// mockTranscodeRepository implements transcode.Repository for testing
type mockTranscodeRepository struct {
	updateFunc      func(ctx context.Context, job *transcode.TranscodeJob) error
	updateCallCount int
	lastUpdatedJob  *transcode.TranscodeJob
}

func (m *mockTranscodeRepository) Create(ctx context.Context, job *transcode.TranscodeJob) error {
	return nil
}

func (m *mockTranscodeRepository) Update(ctx context.Context, job *transcode.TranscodeJob) error {
	m.updateCallCount++
	m.lastUpdatedJob = job
	if m.updateFunc != nil {
		return m.updateFunc(ctx, job)
	}
	return nil
}

func (m *mockTranscodeRepository) GetByID(ctx context.Context, id int64) (*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockTranscodeRepository) GetByMediaIDAndQuality(ctx context.Context, mediaID int64, quality string) (*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockTranscodeRepository) ListByStatus(ctx context.Context, status string) ([]*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockTranscodeRepository) ListQueuedJobs(ctx context.Context, limit int) ([]*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockTranscodeRepository) ListProcessingJobs(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockTranscodeRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	return 0, nil
}

func (m *mockTranscodeRepository) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockTranscodeRepository) DeleteByMediaID(ctx context.Context, mediaID int64) error {
	return nil
}

func (m *mockTranscodeRepository) ListAll(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	return nil, nil
}

func (m *mockTranscodeRepository) UpdateAccess(ctx context.Context, mediaID int64, quality string) error {
	return nil
}

func (m *mockTranscodeRepository) GetTotalSize(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockTranscodeRepository) ListByLRU(ctx context.Context, limit int) ([]*transcode.TranscodeJob, error) {
	return nil, nil
}

// Test helper to create a test executor
func createTestExecutor(t *testing.T) *FFmpegExecutor {
	t.Helper()
	cfg := &config.TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		HardwareAccel:    config.AccelNone,
		ProcessGroupKill: true,
		MaxMemoryMB:      2048,
	}
	return &FFmpegExecutor{
		Cfg:            cfg,
		processManager: process.NewManager(convertToProcessConfig(cfg)),
	}
}

// Test helper to create a test logger
func createTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewJobExecutor(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	if jobExecutor == nil {
		t.Fatal("NewJobExecutor returned nil")
	}
	if jobExecutor.repo != repo {
		t.Error("repo not set correctly")
	}
	if jobExecutor.executor != executor {
		t.Error("executor not set correctly")
	}
	if jobExecutor.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestJobExecutor_FFmpegExecutorRef(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	ref := jobExecutor.FFmpegExecutorRef()
	if ref != executor {
		t.Error("FFmpegExecutorRef should return the executor")
	}
}

func TestJobExecutor_Execute_NilJob(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	err := jobExecutor.Execute(
		context.Background(),
		nil,
		"/input.mp4",
		"/output",
		"transcode",
		func(ctx context.Context, opts TranscodeOptions) error { return nil },
	)

	if err == nil {
		t.Error("expected error for nil job")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("expected nil job error, got: %v", err)
	}
}

func TestJobExecutor_Execute_InvalidJobStatus(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	// Create a job that's already completed
	job := &transcode.TranscodeJob{
		ID:      1,
		MediaID: 100,
		Quality: "720p",
		Status:  transcode.StatusCompleted,
	}

	err := jobExecutor.Execute(
		context.Background(),
		job,
		"/input.mp4",
		"/output",
		"transcode",
		func(ctx context.Context, opts TranscodeOptions) error { return nil },
	)

	if err == nil {
		t.Error("expected error for completed job")
	}
	if !strings.Contains(err.Error(), "queued or processing") {
		t.Errorf("expected status error, got: %v", err)
	}
}

func TestJobExecutor_Execute_InvalidQuality(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	job := &transcode.TranscodeJob{
		ID:      1,
		MediaID: 100,
		Quality: "invalid-quality",
		Status:  transcode.StatusQueued,
	}

	err := jobExecutor.Execute(
		context.Background(),
		job,
		"/input.mp4",
		"/output",
		"transcode",
		func(ctx context.Context, opts TranscodeOptions) error { return nil },
	)

	if err == nil {
		t.Error("expected error for invalid quality")
	}
	// Job should be marked as failed
	if job.Status != transcode.StatusFailed {
		t.Errorf("job should be marked as failed, got: %s", job.Status)
	}
}

func TestConvertToFFmpegProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile *profile.AdaptiveProfile
		wantNil bool
	}{
		{
			name:    "nil profile returns nil",
			profile: nil,
			wantNil: true,
		},
		{
			name: "full profile converts correctly",
			profile: &profile.AdaptiveProfile{
				Width:           1920,
				Height:          1080,
				VideoBitrate:    8_000_000,
				VideoMaxRate:    8_800_000,
				VideoBufSize:    16_000_000,
				AudioBitrate:    192_000,
				AudioChannels:   2,
				AudioSampleRate: 48000,
				SegmentDuration: 4,
				GOPSize:         60,
				PreferredCodec:  "h264",
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToFFmpegProfile(tt.profile)
			if tt.wantNil {
				if result != nil {
					t.Error("expected nil result")
				}
			} else {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if result.Width != tt.profile.Width {
					t.Errorf("Width = %v, want %v", result.Width, tt.profile.Width)
				}
				if result.Height != tt.profile.Height {
					t.Errorf("Height = %v, want %v", result.Height, tt.profile.Height)
				}
				if result.VideoBitrate != tt.profile.VideoBitrate {
					t.Errorf("VideoBitrate = %v, want %v", result.VideoBitrate, tt.profile.VideoBitrate)
				}
				if result.AudioBitrate != tt.profile.AudioBitrate {
					t.Errorf("AudioBitrate = %v, want %v", result.AudioBitrate, tt.profile.AudioBitrate)
				}
				if result.AudioChannels != tt.profile.AudioChannels {
					t.Errorf("AudioChannels = %v, want %v", result.AudioChannels, tt.profile.AudioChannels)
				}
				if result.SegmentDuration != tt.profile.SegmentDuration {
					t.Errorf("SegmentDuration = %v, want %v", result.SegmentDuration, tt.profile.SegmentDuration)
				}
				if result.GOPSize != tt.profile.GOPSize {
					t.Errorf("GOPSize = %v, want %v", result.GOPSize, tt.profile.GOPSize)
				}
				if result.PreferredCodec != tt.profile.PreferredCodec {
					t.Errorf("PreferredCodec = %v, want %v", result.PreferredCodec, tt.profile.PreferredCodec)
				}
			}
		})
	}
}

func TestConvertToFFmpegVideoInfo(t *testing.T) {
	tests := []struct {
		name      string
		videoInfo *videoinfo.VideoInfo
		wantNil   bool
	}{
		{
			name:      "nil video info returns nil",
			videoInfo: nil,
			wantNil:   true,
		},
		{
			name: "HDR video info converts correctly",
			videoInfo: &videoinfo.VideoInfo{
				Width:         3840,
				Height:        2160,
				AudioChannels: 6,
				IsHDR:         true,
			},
			wantNil: false,
		},
		{
			name: "SDR video info converts correctly",
			videoInfo: &videoinfo.VideoInfo{
				Width:         1920,
				Height:        1080,
				AudioChannels: 2,
				IsHDR:         false,
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToFFmpegVideoInfo(tt.videoInfo)
			if tt.wantNil {
				if result != nil {
					t.Error("expected nil result")
				}
			} else {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if result.Width != tt.videoInfo.Width {
					t.Errorf("Width = %v, want %v", result.Width, tt.videoInfo.Width)
				}
				if result.Height != tt.videoInfo.Height {
					t.Errorf("Height = %v, want %v", result.Height, tt.videoInfo.Height)
				}
				if result.AudioChannels != tt.videoInfo.AudioChannels {
					t.Errorf("AudioChannels = %v, want %v", result.AudioChannels, tt.videoInfo.AudioChannels)
				}
				if result.IsHDR != tt.videoInfo.IsHDR {
					t.Errorf("IsHDR = %v, want %v", result.IsHDR, tt.videoInfo.IsHDR)
				}
			}
		})
	}
}

func TestConvertToProcessConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.TranscodeConfig
		wantNil bool
	}{
		{
			name:    "nil config returns nil",
			config:  nil,
			wantNil: true,
		},
		{
			name: "full config with paths",
			config: &config.TranscodeConfig{
				FFmpegPaths: &infraffmpeg.Paths{
					FFmpeg:  "/usr/bin/ffmpeg",
					FFprobe: "/usr/bin/ffprobe",
					LibPath: "/usr/lib/ffmpeg-custom",
				},
				HardwareAccel:    config.AccelNVENC,
				HardwareDevice:   "/dev/nvidia0",
				ProcessGroupKill: true,
				MaxMemoryMB:      4096,
			},
			wantNil: false,
		},
		{
			name: "software config",
			config: &config.TranscodeConfig{
				FFmpegPaths: &infraffmpeg.Paths{
					FFmpeg:  "/usr/bin/ffmpeg",
					FFprobe: "/usr/bin/ffprobe",
				},
				HardwareAccel:    config.AccelNone,
				ProcessGroupKill: false,
				MaxMemoryMB:      2048,
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToProcessConfig(tt.config)
			if tt.wantNil {
				if result != nil {
					t.Error("expected nil result")
				}
			} else {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if tt.config.FFmpegPaths != nil {
					if result.FFmpegPath != tt.config.FFmpegPaths.FFmpeg {
						t.Errorf("FFmpegPath = %v, want %v", result.FFmpegPath, tt.config.FFmpegPaths.FFmpeg)
					}
					if result.FFmpegLibPath != tt.config.FFmpegPaths.LibPath {
						t.Errorf("FFmpegLibPath = %v, want %v", result.FFmpegLibPath, tt.config.FFmpegPaths.LibPath)
					}
				}
				if result.ProcessGroupKill != tt.config.ProcessGroupKill {
					t.Errorf("ProcessGroupKill = %v, want %v", result.ProcessGroupKill, tt.config.ProcessGroupKill)
				}
				if result.MaxMemoryMB != tt.config.MaxMemoryMB {
					t.Errorf("MaxMemoryMB = %v, want %v", result.MaxMemoryMB, tt.config.MaxMemoryMB)
				}
			}
		})
	}
}

func TestMonitorProgress(t *testing.T) {
	tests := []struct {
		name           string
		stderrContent  string
		totalDuration  time.Duration
		expectedCalls  int
		expectFinalPct int
	}{
		{
			name:           "normal progress with duration",
			stderrContent:  "time=00:00:30.00 bitrate=5000kbps\ntime=00:01:00.00 bitrate=5000kbps\n",
			totalDuration:  2 * time.Minute,
			expectedCalls:  3, // 25%, 50%, and 100% at end
			expectFinalPct: 100,
		},
		{
			name:           "progress exceeds duration caps at 100",
			stderrContent:  "time=00:03:00.00 bitrate=5000kbps\n",
			totalDuration:  2 * time.Minute,
			expectedCalls:  1, // caps at 100
			expectFinalPct: 100,
		},
		{
			name:           "no duration reports 50%",
			stderrContent:  "time=00:01:00.00 bitrate=5000kbps\n",
			totalDuration:  0,
			expectedCalls:  2, // 50%, then 100
			expectFinalPct: 100,
		},
		{
			name:           "lines without time are ignored",
			stderrContent:  "frame=1000 fps=30 q=28.0\nspeed=1.5x\ntime=00:00:30.00\n",
			totalDuration:  2 * time.Minute,
			expectedCalls:  2, // 25% from time line, then 100% at end
			expectFinalPct: 100,
		},
		{
			name:           "progress with hours format",
			stderrContent:  "time=01:30:00.00 bitrate=5000kbps\n",
			totalDuration:  3 * time.Hour,
			expectedCalls:  2, // 50%, then 100
			expectFinalPct: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.TranscodeConfig{
				FFmpegPaths: &infraffmpeg.Paths{
					FFmpeg:  "/usr/bin/ffmpeg",
					FFprobe: "/usr/bin/ffprobe",
				},
			}
			executor := &FFmpegExecutor{
				Cfg:            cfg,
				processManager: process.NewManager(convertToProcessConfig(cfg)),
			}

			var progressValues []int
			handler := func(progress int) {
				progressValues = append(progressValues, progress)
			}

			done := make(chan struct{})
			reader := bytes.NewReader([]byte(tt.stderrContent))

			executor.monitorProgress(reader, tt.totalDuration, handler, done)

			<-done // Wait for monitoring to complete

			if len(progressValues) < 1 {
				t.Errorf("expected at least 1 progress call, got %d", len(progressValues))
			}

			// Last value should be 100
			if len(progressValues) > 0 && progressValues[len(progressValues)-1] != tt.expectFinalPct {
				t.Errorf("final progress = %d, want %d", progressValues[len(progressValues)-1], tt.expectFinalPct)
			}
		})
	}
}

func TestMonitorProgress_NilHandler(t *testing.T) {
	cfg := &config.TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
	}
	executor := &FFmpegExecutor{
		Cfg:            cfg,
		processManager: process.NewManager(convertToProcessConfig(cfg)),
	}

	done := make(chan struct{})
	reader := bytes.NewReader([]byte("time=00:01:00.00\n"))

	// Should not panic with nil handler
	executor.monitorProgress(reader, 2*time.Minute, nil, done)

	<-done
}

func TestJobExecutor_prepareOptions(t *testing.T) {
	repo := &mockTranscodeRepository{}
	cfg := &config.TranscodeConfig{
		FFmpegPaths: &infraffmpeg.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		ToneMappingEnabled:         true,
		ToneMappingAlgorithm:       "hable",
		ToneMappingBackend:         "libplacebo",
		LibPlaceboPeakDetect:       true,
		LibPlaceboContrastRecovery: 0.3,
	}
	executor := &FFmpegExecutor{
		Cfg:            cfg,
		processManager: process.NewManager(convertToProcessConfig(cfg)),
	}
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	t.Run("basic options", func(t *testing.T) {
		job := &transcode.TranscodeJob{
			ID:      1,
			MediaID: 100,
			Quality: "1080p",
		}
		adaptiveProfile := &profile.AdaptiveProfile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 8_000_000,
		}

		opts := jobExecutor.prepareOptions(job, "/input.mp4", "/output", adaptiveProfile, nil)

		if opts.InputPath != "/input.mp4" {
			t.Errorf("InputPath = %v, want /input.mp4", opts.InputPath)
		}
		if opts.OutputDir != "/output" {
			t.Errorf("OutputDir = %v, want /output", opts.OutputDir)
		}
		if opts.ToneMappingEnabled != true {
			t.Error("ToneMappingEnabled should be true")
		}
		if opts.ToneMappingAlgorithm != "hable" {
			t.Errorf("ToneMappingAlgorithm = %v, want hable", opts.ToneMappingAlgorithm)
		}
	})

	t.Run("with specific audio track", func(t *testing.T) {
		job := &transcode.TranscodeJob{
			ID:      1,
			MediaID: 100,
			Quality: "1080p",
		}
		adaptiveProfile := &profile.AdaptiveProfile{}
		videoInfo := &videoinfo.VideoInfo{
			SelectedAudioTrackIndex: 2,
			AudioCodec:              "ac3",
			AudioChannels:           6,
		}

		opts := jobExecutor.prepareOptions(job, "/input.mp4", "/output", adaptiveProfile, videoInfo)

		if !opts.UseSpecificAudioTrack {
			t.Error("UseSpecificAudioTrack should be true")
		}
		if opts.AudioTrackIndex != 2 {
			t.Errorf("AudioTrackIndex = %v, want 2", opts.AudioTrackIndex)
		}
	})

	t.Run("with start position", func(t *testing.T) {
		job := &transcode.TranscodeJob{
			ID:            1,
			MediaID:       100,
			Quality:       "1080p",
			StartPosition: 120,
		}
		adaptiveProfile := &profile.AdaptiveProfile{}

		opts := jobExecutor.prepareOptions(job, "/input.mp4", "/output", adaptiveProfile, nil)

		if !opts.UseStartPosition {
			t.Error("UseStartPosition should be true")
		}
		if opts.StartPosition != 120 {
			t.Errorf("StartPosition = %v, want 120", opts.StartPosition)
		}
	})
}

func TestJobExecutor_createProgressHandler(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	job := &transcode.TranscodeJob{
		ID:       1,
		MediaID:  100,
		Quality:  "1080p",
		Status:   transcode.StatusProcessing,
		Progress: 0,
	}

	handler := jobExecutor.createProgressHandler(job)

	// Test progress updates
	// lastReported starts at 0, so 5-0=5 which is >= 5 threshold
	handler(5) // Should trigger update (5% threshold met: 5-0=5)
	if repo.updateCallCount != 1 {
		t.Errorf("expected 1 update at 5%%, got %d", repo.updateCallCount)
	}

	handler(8) // Should be ignored (8-5=3 < 5% from last update)
	if repo.updateCallCount != 1 {
		t.Errorf("expected still 1 update after 8%%, got %d", repo.updateCallCount)
	}

	handler(10) // Should trigger update (10-5=5 >= 5% threshold)
	if repo.updateCallCount != 2 {
		t.Errorf("expected 2 updates at 10%%, got %d", repo.updateCallCount)
	}

	handler(12) // Should be ignored (12-10=2 < 5)
	if repo.updateCallCount != 2 {
		t.Errorf("expected still 2 updates after 12%%, got %d", repo.updateCallCount)
	}

	handler(100) // 100% always triggers update (special case)
	if repo.updateCallCount != 3 {
		t.Errorf("expected 3 updates at 100%%, got %d", repo.updateCallCount)
	}
}

func TestJobExecutor_createProgressHandler_InvalidProgress(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	job := &transcode.TranscodeJob{
		ID:       1,
		MediaID:  100,
		Quality:  "1080p",
		Status:   transcode.StatusProcessing,
		Progress: 0,
	}

	handler := jobExecutor.createProgressHandler(job)

	// Invalid progress (>100) should not update
	handler(150)
	if repo.updateCallCount != 0 {
		t.Errorf("invalid progress should not trigger update, got %d", repo.updateCallCount)
	}

	// Negative progress should not update
	handler(-10)
	if repo.updateCallCount != 0 {
		t.Errorf("negative progress should not trigger update, got %d", repo.updateCallCount)
	}
}

func TestJobExecutor_failJob(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	t.Run("normal error", func(t *testing.T) {
		job := &transcode.TranscodeJob{
			ID:      1,
			MediaID: 100,
			Quality: "1080p",
			Status:  transcode.StatusProcessing,
		}

		err := jobExecutor.failJob(context.Background(), job, io.EOF)

		if err != io.EOF {
			t.Errorf("should return original error, got: %v", err)
		}
		if job.Status != transcode.StatusFailed {
			t.Errorf("job status = %v, want failed", job.Status)
		}
		if repo.updateCallCount != 1 {
			t.Errorf("expected 1 update, got %d", repo.updateCallCount)
		}
	})

	t.Run("long error message truncated", func(t *testing.T) {
		repo := &mockTranscodeRepository{}
		jobExecutor := NewJobExecutor(repo, executor, logger)

		job := &transcode.TranscodeJob{
			ID:      2,
			MediaID: 100,
			Quality: "1080p",
			Status:  transcode.StatusProcessing,
		}

		// Create error message longer than 1000 chars
		longError := strings.Repeat("x", 1500)
		jobExecutor.failJob(context.Background(), job, errWithLongMessage{longError})

		if len(job.Error) > 1003 { // 1000 + "..."
			t.Errorf("error message not truncated, length = %d", len(job.Error))
		}
	})
}

func TestJobExecutor_completeJob(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	// Create temp directory with a file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "playlist.m3u8")
	os.WriteFile(testFile, []byte("test content"), 0o644)

	job := &transcode.TranscodeJob{
		ID:      1,
		MediaID: 100,
		Quality: "1080p",
		Status:  transcode.StatusProcessing,
	}

	err := jobExecutor.completeJob(context.Background(), job, tempDir, "transcode")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if job.Status != transcode.StatusCompleted {
		t.Errorf("job status = %v, want completed", job.Status)
	}
	if job.Progress != 100 {
		t.Errorf("job progress = %d, want 100", job.Progress)
	}
	if job.FilePath != tempDir {
		t.Errorf("job FilePath = %v, want %v", job.FilePath, tempDir)
	}
	if job.FileSizeBytes == 0 {
		t.Error("job FileSizeBytes should be > 0")
	}
}

func TestConvertToFFmpegTranscodeOptions(t *testing.T) {
	opts := TranscodeOptions{
		InputPath:                  "/input.mp4",
		OutputDir:                  "/output",
		AudioTrackIndex:            2,
		UseSpecificAudioTrack:      true,
		StartPosition:              120,
		UseStartPosition:           true,
		ToneMappingEnabled:         true,
		ToneMappingAlgorithm:       "hable",
		ToneMappingBackend:         "libplacebo",
		LibPlaceboPeakDetect:       true,
		LibPlaceboContrastRecovery: 0.5,
		VideoCodec:                 hls.CodecH265,
		Profile: &profile.AdaptiveProfile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 8_000_000,
		},
		VideoInfo: &videoinfo.VideoInfo{
			Width:  3840,
			Height: 2160,
			IsHDR:  true,
		},
	}

	result := convertToFFmpegTranscodeOptions(opts)

	if result.InputPath != opts.InputPath {
		t.Errorf("InputPath = %v, want %v", result.InputPath, opts.InputPath)
	}
	if result.OutputDir != opts.OutputDir {
		t.Errorf("OutputDir = %v, want %v", result.OutputDir, opts.OutputDir)
	}
	if result.AudioTrackIndex != opts.AudioTrackIndex {
		t.Errorf("AudioTrackIndex = %v, want %v", result.AudioTrackIndex, opts.AudioTrackIndex)
	}
	if result.UseSpecificAudioTrack != opts.UseSpecificAudioTrack {
		t.Errorf("UseSpecificAudioTrack = %v, want %v", result.UseSpecificAudioTrack, opts.UseSpecificAudioTrack)
	}
	if result.StartPosition != opts.StartPosition {
		t.Errorf("StartPosition = %v, want %v", result.StartPosition, opts.StartPosition)
	}
	if result.UseStartPosition != opts.UseStartPosition {
		t.Errorf("UseStartPosition = %v, want %v", result.UseStartPosition, opts.UseStartPosition)
	}
	if result.ToneMappingEnabled != opts.ToneMappingEnabled {
		t.Errorf("ToneMappingEnabled = %v, want %v", result.ToneMappingEnabled, opts.ToneMappingEnabled)
	}
	if result.ToneMappingAlgorithm != opts.ToneMappingAlgorithm {
		t.Errorf("ToneMappingAlgorithm = %v, want %v", result.ToneMappingAlgorithm, opts.ToneMappingAlgorithm)
	}
	if result.VideoCodec != hls.CodecH265 {
		t.Errorf("VideoCodec = %v, want h265", result.VideoCodec)
	}
	if result.Profile == nil {
		t.Fatal("Profile should not be nil")
	}
	if result.VideoInfo == nil {
		t.Fatal("VideoInfo should not be nil")
	}
}

func TestJobExecutor_getVideoInfo(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	job := &transcode.TranscodeJob{
		ID:      1,
		MediaID: 100,
		Quality: "1080p",
	}

	// Test with non-existent file
	_, err := jobExecutor.getVideoInfo(context.Background(), job, "/nonexistent/file.mp4")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestJobExecutor_checkDiskSpace(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	job := &transcode.TranscodeJob{
		ID:      1,
		MediaID: 100,
		Quality: "1080p",
	}

	adaptiveProfile := &profile.AdaptiveProfile{
		VideoBitrate: 8_000_000,
		AudioBitrate: 192_000,
	}

	t.Run("nil video info returns no error", func(t *testing.T) {
		err := jobExecutor.checkDiskSpace(context.Background(), job, "/tmp", adaptiveProfile, nil)
		if err != nil {
			t.Errorf("expected no error with nil video info, got: %v", err)
		}
	})

	t.Run("video info without duration returns no error", func(t *testing.T) {
		videoInfo := &videoinfo.VideoInfo{
			Width:    1920,
			Height:   1080,
			Duration: 0, // No duration
		}
		err := jobExecutor.checkDiskSpace(context.Background(), job, "/tmp", adaptiveProfile, videoInfo)
		if err != nil {
			t.Errorf("expected no error with zero duration, got: %v", err)
		}
	})

	t.Run("video info with duration calculates size", func(t *testing.T) {
		videoInfo := &videoinfo.VideoInfo{
			Width:    1920,
			Height:   1080,
			Duration: 3600.0, // 1 hour
		}
		tempDir := t.TempDir()
		err := jobExecutor.checkDiskSpace(context.Background(), job, tempDir, adaptiveProfile, videoInfo)
		// This might pass or fail depending on disk space, but should not panic
		// We just verify the function executes
		_ = err
	})
}

func TestJobExecutor_completeJob_ErrorPaths(t *testing.T) {
	t.Run("completeJob with update error", func(t *testing.T) {
		repo := &mockTranscodeRepository{
			updateFunc: func(ctx context.Context, job *transcode.TranscodeJob) error {
				return io.EOF // Simulate update error
			},
		}
		executor := createTestExecutor(t)
		logger := createTestLogger()

		jobExecutor := NewJobExecutor(repo, executor, logger)

		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "playlist.m3u8")
		os.WriteFile(testFile, []byte("test"), 0o644)

		job := &transcode.TranscodeJob{
			ID:      1,
			MediaID: 100,
			Quality: "1080p",
			Status:  transcode.StatusProcessing,
		}

		err := jobExecutor.completeJob(context.Background(), job, tempDir, "transcode")
		if err == nil {
			t.Error("expected error when update fails")
		}
		if !strings.Contains(err.Error(), "failed to update job status") {
			t.Errorf("expected update error, got: %v", err)
		}
	})

	t.Run("completeJob with invalid directory for size calculation", func(t *testing.T) {
		repo := &mockTranscodeRepository{}
		executor := createTestExecutor(t)
		logger := createTestLogger()

		jobExecutor := NewJobExecutor(repo, executor, logger)

		job := &transcode.TranscodeJob{
			ID:      1,
			MediaID: 100,
			Quality: "1080p",
			Status:  transcode.StatusProcessing,
		}

		// Use non-existent directory - should not fail but log warning
		err := jobExecutor.completeJob(context.Background(), job, "/nonexistent/dir", "transcode")
		if err != nil {
			// Update should succeed even if size calculation fails
			if !strings.Contains(err.Error(), "failed to update job status") {
				t.Errorf("unexpected error: %v", err)
			}
		}
	})
}

func TestJobExecutor_failJob_UpdateError(t *testing.T) {
	updateErr := fmt.Errorf("database connection lost")
	repo := &mockTranscodeRepository{
		updateFunc: func(ctx context.Context, job *transcode.TranscodeJob) error {
			return updateErr
		},
	}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	job := &transcode.TranscodeJob{
		ID:      1,
		MediaID: 100,
		Quality: "1080p",
		Status:  transcode.StatusProcessing,
	}

	originalErr := fmt.Errorf("transcode failed: out of memory")
	err := jobExecutor.failJob(context.Background(), job, originalErr)

	// Should return error mentioning both the original error and update failure
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "out of memory") {
		t.Errorf("error should contain original error message, got: %v", err)
	}
	if !strings.Contains(errStr, "failed to update job") {
		t.Errorf("error should mention update failure, got: %v", err)
	}

	// Job should still be marked as failed
	if job.Status != transcode.StatusFailed {
		t.Errorf("job status = %v, want failed", job.Status)
	}
}

func TestJobExecutor_Execute_ValidationError(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	job := &transcode.TranscodeJob{
		ID:      1,
		MediaID: 100,
		Quality: "1080p",
		Status:  transcode.StatusQueued,
	}

	// Test with non-existent input path (should fail validation)
	err := jobExecutor.Execute(
		context.Background(),
		job,
		"/nonexistent/input.mp4",
		t.TempDir(),
		"transcode",
		func(ctx context.Context, opts TranscodeOptions) error { return nil },
	)

	if err == nil {
		t.Error("expected validation error for non-existent input")
	}
	if job.Status != transcode.StatusFailed {
		t.Errorf("job status should be failed, got: %s", job.Status)
	}
}

func TestJobExecutor_Execute_DirectoryCreationFailure(t *testing.T) {
	repo := &mockTranscodeRepository{}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	job := &transcode.TranscodeJob{
		ID:      1,
		MediaID: 100,
		Quality: "1080p",
		Status:  transcode.StatusQueued,
	}

	// Create a file instead of directory to cause mkdir failure
	tempDir := t.TempDir()
	blockingFile := filepath.Join(tempDir, "hls")
	os.WriteFile(blockingFile, []byte("blocking"), 0o644)

	err := jobExecutor.Execute(
		context.Background(),
		job,
		"/some/input.mp4",
		tempDir,
		"transcode",
		func(ctx context.Context, opts TranscodeOptions) error { return nil },
	)

	if err == nil {
		t.Error("expected error when directory creation fails")
	}
	if job.Status != transcode.StatusFailed {
		t.Errorf("job status should be failed, got: %s", job.Status)
	}
}

func TestJobExecutor_Execute_MarkProcessingError(t *testing.T) {
	updateCallCount := 0
	repo := &mockTranscodeRepository{
		updateFunc: func(ctx context.Context, job *transcode.TranscodeJob) error {
			updateCallCount++
			if updateCallCount == 1 {
				// First update (mark as processing) fails
				return fmt.Errorf("database error")
			}
			// Subsequent updates succeed
			return nil
		},
	}
	executor := createTestExecutor(t)
	logger := createTestLogger()

	jobExecutor := NewJobExecutor(repo, executor, logger)

	// Create a valid test file
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "input.mp4")
	os.WriteFile(inputFile, []byte("fake video"), 0o644)

	job := &transcode.TranscodeJob{
		ID:      1,
		MediaID: 100,
		Quality: "1080p",
		Status:  transcode.StatusQueued,
	}

	// The operation should continue even if marking as processing fails
	// Use "remux" operation to skip validation errors on fake file
	executorCalled := false
	err := jobExecutor.Execute(
		context.Background(),
		job,
		inputFile,
		tempDir,
		"remux",
		func(ctx context.Context, opts TranscodeOptions) error {
			executorCalled = true
			return fmt.Errorf("executor error") // Force failure to trigger failJob path
		},
	)

	// Should still call executor and fail with executor error
	if !executorCalled {
		t.Error("executor function should be called even if mark processing fails")
	}
	if err == nil {
		t.Error("expected error from executor")
	}
}
