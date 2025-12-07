package session

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/logging"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
)

func createTestProfile() *profile.AdaptiveProfile {
	return &profile.AdaptiveProfile{
		ID:              "720p",
		Width:           1280,
		Height:          720,
		VideoBitrate:    3000000,
		VideoMaxRate:    4500000,
		VideoBufSize:    6000000,
		AudioBitrate:    128000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		SegmentDuration: 6,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{"h264"},
		Preset:          "veryfast",
		GOPSize:         60,
	}
}

func createTestConfig(command string) *Config {
	return &Config{
		FFmpegPaths: &hls.Paths{
			FFmpeg: command,
		},
	}
}

func createTestVideoInfo(codec string, width, height int, isHDR bool) *hls.VideoInfo {
	return &hls.VideoInfo{
		Codec:  codec,
		Width:  width,
		Height: height,
		IsHDR:  isHDR,
	}
}

func createTestLogWriter(sessionID string, logDir string) (*logging.LogWriter, error) {
	return logging.NewLogWriterForTest(sessionID, logDir)
}

func TestStart_BasicFunctionality(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		validate func(*testing.T, *TranscodeSession)
	}{
		{
			name:    "creates output directory",
			command: "echo",
			validate: func(t *testing.T, s *TranscodeSession) {
				if _, err := os.Stat(s.OutputDir); os.IsNotExist(err) {
					t.Errorf("Output directory was not created: %s", s.OutputDir)
				}
			},
		},
		{
			name:    "executes ffmpeg command",
			command: "sleep",
			validate: func(t *testing.T, s *TranscodeSession) {
				if s.FFmpegCmd == nil {
					t.Error("FFmpegCmd should not be nil")
				}
				if s.FFmpegCmd.Process == nil {
					t.Error("FFmpegCmd.Process should not be nil")
				}
				if s.FFmpegStartedAt.IsZero() {
					t.Error("FFmpegStartedAt should be set")
				}
				if s.watchdog == nil {
					t.Error("watchdog should be initialized")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			outputDir := filepath.Join(tempDir, "transcode")
			session := NewTranscodeSession(1, "720p", 0, -1, outputDir, logger, nil)

			inputPath := filepath.Join(tempDir, "test.mp4")
			if err := os.WriteFile(inputPath, []byte("test"), 0o644); err != nil {
				t.Fatalf("failed to create test input file: %v", err)
			}

			err := session.Start(StartParams{
				InputPath:             inputPath,
				Profile:               createTestProfile(),
				Strategy:              "transcode",
				HWAccel:               "none",
				HWDevice:              "",
				VideoInfo:             createTestVideoInfo("h264", 1920, 1080, false),
				Config:                createTestConfig(tt.command),
				ClientSupportedCodecs: []string{"h264"},
			})
			if err != nil {
				t.Fatalf("Start() failed: %v", err)
			}
			defer session.Stop()

			tt.validate(t, session)
		})
	}
}

func TestStart_WithLogWriter(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	outputDir := filepath.Join(tempDir, "transcode")

	logWriter, err := createTestLogWriter("test-session", filepath.Join(tempDir, "logs"))
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer logWriter.Close()

	session := NewTranscodeSession(1, "720p", 0, -1, outputDir, logger, logWriter)

	inputPath := filepath.Join(tempDir, "test.mp4")
	if err := os.WriteFile(inputPath, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test input file: %v", err)
	}

	err = session.Start(StartParams{
		InputPath:             inputPath,
		Profile:               createTestProfile(),
		Strategy:              "transcode",
		HWAccel:               "none",
		HWDevice:              "",
		VideoInfo:             createTestVideoInfo("h264", 1920, 1080, false),
		Config:                createTestConfig("echo"),
		ClientSupportedCodecs: []string{"h264"},
	})
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer session.Stop()

	time.Sleep(100 * time.Millisecond)

	logContent, err := os.ReadFile(logWriter.FilePath())
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logString := string(logContent)
	if !strings.Contains(logString, "Command:") {
		t.Error("Log file should contain FFmpeg command")
	}
	if !strings.Contains(logString, "TIMING: FFmpeg started") {
		t.Error("Log file should contain FFmpeg start timing")
	}
}

func TestStop_Scenarios(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*TranscodeSession) error
	}{
		{
			name: "graceful shutdown",
			setup: func(s *TranscodeSession) error {
				ctx, cancel := context.WithCancel(context.Background())
				s.ctx = ctx
				s.cancel = cancel
				s.FFmpegCmd = exec.CommandContext(ctx, "sleep", "30")
				return s.FFmpegCmd.Start()
			},
		},
		{
			name: "no process",
			setup: func(s *TranscodeSession) error {
				return nil
			},
		},
		{
			name: "already stopped process",
			setup: func(s *TranscodeSession) error {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				s.FFmpegCmd = exec.CommandContext(ctx, "echo", "test")
				if err := s.FFmpegCmd.Start(); err != nil {
					return err
				}
				time.Sleep(100 * time.Millisecond)
				return nil
			},
		},
		{
			name: "with fsWatcher",
			setup: func(s *TranscodeSession) error {
				watcher, err := fsnotify.NewWatcher()
				if err != nil {
					return err
				}
				s.fsWatcher = watcher
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				s.FFmpegCmd = exec.CommandContext(ctx, "echo", "test")
				return s.FFmpegCmd.Start()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			session := NewTranscodeSession(1, "720p", 0, -1, t.TempDir(), logger, nil)

			if err := tt.setup(session); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			if err := session.Stop(); err != nil {
				t.Errorf("Stop() returned error: %v", err)
			}
		})
	}
}

func TestStop_ConcurrentCalls(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	session := NewTranscodeSession(1, "720p", 0, -1, t.TempDir(), logger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session.FFmpegCmd = exec.CommandContext(ctx, "sleep", "10")
	if err := session.FFmpegCmd.Start(); err != nil {
		t.Fatalf("failed to start test command: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session.Stop()
		}()
	}

	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("Concurrent Stop() calls did not complete in time")
	}
}

func TestMonitorStdout(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{
			name: "segment detection",
			output: `Opening 'seg_000000.ts' for writing
Opening 'playlist.m3u8' for writing
Opening 'seg_000001.ts' for writing`,
		},
		{
			name:   "empty output",
			output: "",
		},
		{
			name:   "large output",
			output: strings.Repeat("Opening 'seg_000000.ts' for writing\n", 100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			session := NewTranscodeSession(1, "720p", 0, -1, t.TempDir(), logger, nil)

			r, w := io.Pipe()
			done := make(chan bool)
			go func() {
				session.monitorStdout(r)
				done <- true
			}()

			if tt.output != "" {
				go func() {
					w.Write([]byte(tt.output))
					w.Close()
				}()
			} else {
				w.Close()
			}

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Error("monitorStdout did not complete in time")
			}
		})
	}
}

func TestMonitorStderr_ProgressAndErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	session := NewTranscodeSession(1, "720p", 0, -1, t.TempDir(), logger, nil)
	session.FFmpegStartedAt = time.Now()

	r, w := io.Pipe()
	done := make(chan bool)
	go func() {
		session.monitorStderr(r)
		done <- true
	}()

	output := `ffmpeg version 4.4.2
Input #0, mov,mp4,m4a,3gp,3g2,mj2
frame=    0 fps=0.0 q=0.0 size=0kB time=00:00:00.00 bitrate=N/A
frame=   10 fps=0.0 q=28.0 size=256kB time=00:00:00.40 bitrate=5242.9kbits/s
Invalid Block Addition (0x4e7b)
Error opening file for writing`

	w.Write([]byte(output))
	w.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("monitorStderr did not complete in time")
	}
}

func TestMonitorStderr_WithLogWriter(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	logWriter, err := createTestLogWriter("test-session", filepath.Join(tempDir, "logs"))
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer logWriter.Close()

	session := NewTranscodeSession(1, "720p", 0, -1, tempDir, logger, logWriter)
	session.FFmpegStartedAt = time.Now()

	r, w := io.Pipe()
	done := make(chan bool)
	go func() {
		session.monitorStderr(r)
		done <- true
	}()

	testOutput := "frame=   10 fps=0.0 q=28.0 size=256kB time=00:00:00.40 bitrate=5242.9kbits/s\n"
	w.Write([]byte(testOutput))
	w.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("monitorStderr did not complete in time")
	}

	time.Sleep(100 * time.Millisecond)

	logContent, err := os.ReadFile(logWriter.FilePath())
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(logContent), "frame=") {
		t.Error("Log file should contain FFmpeg output")
	}
}

func TestMonitorStderr_TimingMetrics(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	logWriter, err := createTestLogWriter("test-session", filepath.Join(tempDir, "logs"))
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer logWriter.Close()

	session := NewTranscodeSession(1, "720p", 0, -1, tempDir, logger, logWriter)
	session.FFmpegStartedAt = time.Now()

	r, w := io.Pipe()
	done := make(chan bool)
	go func() {
		session.monitorStderr(r)
		done <- true
	}()

	w.Write([]byte("frame=    1 fps=0.0 q=0.0 size=0kB time=00:00:00.00 bitrate=N/A\n"))
	time.Sleep(50 * time.Millisecond)
	w.Write([]byte("Opening 'seg_000000.ts' for writing\n"))
	time.Sleep(50 * time.Millisecond)
	w.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("monitorStderr did not complete in time")
	}

	if session.FirstFrameAt.IsZero() {
		t.Error("FirstFrameAt should be set")
	}
	if session.FirstSegmentAt.IsZero() {
		t.Error("FirstSegmentAt should be set")
	}
	if !session.firstFrameLogged || !session.firstSegmentLogged {
		t.Error("Timing flags should be set")
	}

	time.Sleep(100 * time.Millisecond)
	logContent, err := os.ReadFile(logWriter.FilePath())
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(logContent)
	if !strings.Contains(content, "TIMING: First frame") {
		t.Error("Log should contain first frame timing")
	}
	if !strings.Contains(content, "TIMING: First segment") {
		t.Error("Log should contain first segment timing")
	}
}

func TestMonitorStderr_WatchdogUpdate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	session := NewTranscodeSession(1, "720p", 0, -1, t.TempDir(), logger, nil)
	session.FFmpegStartedAt = time.Now()
	session.watchdog = NewProgressWatchdog(session, 5*time.Second)

	initialTime := time.Unix(0, session.watchdog.lastProgressTime.Load())

	r, w := io.Pipe()
	done := make(chan bool)
	go func() {
		session.monitorStderr(r)
		done <- true
	}()

	time.Sleep(50 * time.Millisecond)
	w.Write([]byte("frame=   10 fps=0.0 q=28.0 size=256kB time=00:00:00.40 bitrate=5242.9kbits/s\n"))
	time.Sleep(50 * time.Millisecond)
	w.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("monitorStderr did not complete in time")
	}

	updatedTime := time.Unix(0, session.watchdog.lastProgressTime.Load())
	if !updatedTime.After(initialTime) {
		t.Error("Watchdog should have been updated")
	}
}

func TestMonitorStderr_LargeChunks(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	logWriter, err := createTestLogWriter("test-large", filepath.Join(tempDir, "logs"))
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer logWriter.Close()

	session := NewTranscodeSession(1, "720p", 0, -1, tempDir, logger, logWriter)
	session.FFmpegStartedAt = time.Now()

	r, w := io.Pipe()
	done := make(chan bool)
	go func() {
		session.monitorStderr(r)
		done <- true
	}()

	largeChunk := bytes.Repeat([]byte("frame=   10 fps=0.0 q=28.0 size=256kB time=00:00:00.40 bitrate=5242.9kbits/s\n"), 100)
	w.Write(largeChunk)
	w.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("monitorStderr did not complete in time")
	}

	time.Sleep(100 * time.Millisecond)
	logContent, err := os.ReadFile(logWriter.FilePath())
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if len(logContent) < len(largeChunk) {
		t.Errorf("Log file size %d is less than expected %d", len(logContent), len(largeChunk))
	}
}

func TestSession_LogWriterAccessors(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	session := NewTranscodeSession(1, "720p", 0, -1, tempDir, logger, nil)
	if session.GetLogWriter() != nil {
		t.Error("GetLogWriter() should return nil initially")
	}

	logWriter, err := createTestLogWriter("test-session", filepath.Join(tempDir, "logs"))
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer logWriter.Close()

	session.SetLogWriter(logWriter)
	if session.GetLogWriter() != logWriter {
		t.Error("GetLogWriter() should return the set LogWriter")
	}

	session.SetLogWriter(nil)
	if session.GetLogWriter() != nil {
		t.Error("GetLogWriter() should return nil after SetLogWriter(nil)")
	}
}

func TestSessionLifecycle_Integration(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	outputDir := filepath.Join(tempDir, "transcode")

	logWriter, err := createTestLogWriter("test-lifecycle", filepath.Join(tempDir, "logs"))
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer logWriter.Close()

	session := NewTranscodeSession(18, "720p", 0, -1, outputDir, logger, logWriter)

	if session.ID == "" || session.MediaID != 18 || session.Quality != "720p" {
		t.Error("Session initialization incorrect")
	}
	if session.GetLogWriter() != logWriter {
		t.Error("LogWriter should be set")
	}

	inputPath := filepath.Join(tempDir, "test.mp4")
	if err := os.WriteFile(inputPath, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test input file: %v", err)
	}

	err = session.Start(StartParams{
		InputPath:             inputPath,
		Profile:               createTestProfile(),
		Strategy:              "transcode",
		HWAccel:               "none",
		HWDevice:              "",
		VideoInfo:             createTestVideoInfo("h264", 1920, 1080, false),
		Config:                createTestConfig("sleep"),
		ClientSupportedCodecs: []string{"h264"},
	})
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if session.FFmpegCmd == nil || session.FFmpegCmd.Process == nil {
		t.Fatal("FFmpegCmd should be running")
	}

	time.Sleep(100 * time.Millisecond)

	beforeUpdate := session.LastAccessed
	time.Sleep(10 * time.Millisecond)
	session.UpdateLastAccessed()
	if !session.LastAccessed.After(beforeUpdate) {
		t.Error("LastAccessed should be updated")
	}

	if err := session.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	if session.FFmpegCmd.ProcessState == nil {
		t.Error("ProcessState should be set after Stop()")
	}

	if _, err := os.Stat(session.OutputDir); os.IsNotExist(err) {
		t.Error("Output directory should exist")
	}
}
