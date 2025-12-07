package ffmpeg

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/paths"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/probe"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/thumbnail"
)

// TestNewService tests the NewService constructor.
func TestNewService(t *testing.T) {
	tests := []struct {
		name       string
		logger     *slog.Logger
		wantLogger bool
		wantErr    bool
		setupEnv   func()
		cleanupEnv func()
	}{
		{
			name:       "with custom logger",
			logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
			wantLogger: true,
			wantErr:    false,
		},
		{
			name:       "with nil logger (should use default)",
			logger:     nil,
			wantLogger: true,
			wantErr:    false,
		},
		{
			name:       "with custom ffmpeg path",
			logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
			wantLogger: true,
			wantErr:    false,
			setupEnv: func() {
				// This will use the system PATH ffmpeg
			},
		},
		{
			name:    "fails when ffmpeg not in PATH",
			logger:  nil,
			wantErr: true,
			setupEnv: func() {
				// Clear PATH and ffmpeg env vars to force failure
				os.Setenv("VIEWRA_FFMPEG_PATH", "")
				os.Setenv("VIEWRA_FFPROBE_PATH", "")
				os.Setenv("PATH_BACKUP", os.Getenv("PATH"))
				os.Setenv("PATH", "")
			},
			cleanupEnv: func() {
				// Restore PATH
				os.Setenv("PATH", os.Getenv("PATH_BACKUP"))
				os.Unsetenv("PATH_BACKUP")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			if tt.cleanupEnv != nil {
				defer tt.cleanupEnv()
			}

			svc, err := NewService(tt.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if svc == nil {
				t.Fatal("NewService() returned nil service")
			}

			if svc.logger == nil && tt.wantLogger {
				t.Error("Service logger is nil, expected non-nil")
			}

			if svc.paths == nil {
				t.Error("Service paths is nil")
			}

			if svc.probe == nil {
				t.Error("Service probe client is nil")
			}

			if svc.thumbnail == nil {
				t.Error("Service thumbnail client is nil")
			}
		})
	}
}

// TestNewServiceWithPaths tests the NewServiceWithPaths constructor.
func TestNewServiceWithPaths(t *testing.T) {
	tests := []struct {
		name       string
		paths      *paths.Paths
		logger     *slog.Logger
		wantLogger bool
		wantErr    bool
	}{
		{
			name: "with valid paths and logger",
			paths: &paths.Paths{
				FFmpeg:  "/usr/bin/ffmpeg",
				FFprobe: "/usr/bin/ffprobe",
			},
			logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
			wantLogger: true,
			wantErr:    false,
		},
		{
			name: "with valid paths and nil logger",
			paths: &paths.Paths{
				FFmpeg:  "/usr/bin/ffmpeg",
				FFprobe: "/usr/bin/ffprobe",
			},
			logger:     nil,
			wantLogger: true,
			wantErr:    false,
		},
		{
			name:       "with nil paths (should call NewService)",
			paths:      nil,
			logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
			wantLogger: true,
			wantErr:    false,
		},
		{
			name: "with custom lib path",
			paths: &paths.Paths{
				FFmpeg:  "/custom/bin/ffmpeg",
				FFprobe: "/custom/bin/ffprobe",
				LibPath: "/custom/lib",
			},
			logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
			wantLogger: true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewServiceWithPaths(tt.paths, tt.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewServiceWithPaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if svc == nil {
				t.Fatal("NewServiceWithPaths() returned nil service")
			}

			if svc.logger == nil && tt.wantLogger {
				t.Error("Service logger is nil, expected non-nil")
			}

			if svc.paths == nil {
				t.Error("Service paths is nil")
			}

			if svc.probe == nil {
				t.Error("Service probe client is nil")
			}

			if svc.thumbnail == nil {
				t.Error("Service thumbnail client is nil")
			}

			// If we passed in non-nil paths, verify they are used
			if tt.paths != nil && svc.paths != tt.paths {
				t.Error("Service did not use the provided paths")
			}
		})
	}
}

// TestServicePaths tests the Paths() method.
func TestServicePaths(t *testing.T) {
	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
		LibPath: "/custom/lib",
	}

	svc, err := NewServiceWithPaths(p, nil)
	if err != nil {
		t.Fatalf("NewServiceWithPaths() error = %v", err)
	}

	got := svc.Paths()
	if got != p {
		t.Errorf("Paths() returned different pointer: got %p, want %p", got, p)
	}

	if got.FFmpeg != p.FFmpeg {
		t.Errorf("Paths().FFmpeg = %v, want %v", got.FFmpeg, p.FFmpeg)
	}

	if got.FFprobe != p.FFprobe {
		t.Errorf("Paths().FFprobe = %v, want %v", got.FFprobe, p.FFprobe)
	}

	if got.LibPath != p.LibPath {
		t.Errorf("Paths().LibPath = %v, want %v", got.LibPath, p.LibPath)
	}
}

// TestServiceProbeClient tests the ProbeClient() method.
func TestServiceProbeClient(t *testing.T) {
	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}

	svc, err := NewServiceWithPaths(p, nil)
	if err != nil {
		t.Fatalf("NewServiceWithPaths() error = %v", err)
	}

	client := svc.ProbeClient()
	if client == nil {
		t.Error("ProbeClient() returned nil")
	}

	// Verify it's the same client stored in the service
	if client != svc.probe {
		t.Error("ProbeClient() returned different client than stored")
	}
}

// TestServiceThumbnailClient tests the ThumbnailClient() method.
func TestServiceThumbnailClient(t *testing.T) {
	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}

	svc, err := NewServiceWithPaths(p, nil)
	if err != nil {
		t.Fatalf("NewServiceWithPaths() error = %v", err)
	}

	client := svc.ThumbnailClient()
	if client == nil {
		t.Error("ThumbnailClient() returned nil")
	}

	// Verify it's the same client stored in the service
	if client != svc.thumbnail {
		t.Error("ThumbnailClient() returned different client than stored")
	}
}

// TestServiceExtractMetadata tests the ExtractMetadata delegation.
func TestServiceExtractMetadata(t *testing.T) {
	// Skip if ffmpeg is not available
	if _, err := os.Stat("/usr/bin/ffprobe"); os.IsNotExist(err) {
		t.Skip("ffprobe not found at /usr/bin/ffprobe, skipping integration test")
	}

	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}

	svc, err := NewServiceWithPaths(p, nil)
	if err != nil {
		t.Fatalf("NewServiceWithPaths() error = %v", err)
	}

	tests := []struct {
		name     string
		filePath string
		wantErr  bool
		errType  error
	}{
		{
			name:     "invalid file path",
			filePath: "/nonexistent/file.mkv",
			wantErr:  true,
			errType:  probe.ErrInvalidFile,
		},
		{
			name:     "empty file path",
			filePath: "",
			wantErr:  true,
			errType:  probe.ErrInvalidFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			metadata, err := svc.ExtractMetadata(ctx, tt.filePath)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errType != nil {
				if !errors.Is(err, tt.errType) {
					t.Errorf("ExtractMetadata() error type = %T, want %T", err, tt.errType)
				}
			}

			if !tt.wantErr && metadata == nil {
				t.Error("ExtractMetadata() returned nil metadata for valid file")
			}
		})
	}
}

// TestServiceExtractTracks tests the ExtractTracks delegation.
func TestServiceExtractTracks(t *testing.T) {
	// Skip if ffmpeg is not available
	if _, err := os.Stat("/usr/bin/ffprobe"); os.IsNotExist(err) {
		t.Skip("ffprobe not found at /usr/bin/ffprobe, skipping integration test")
	}

	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}

	svc, err := NewServiceWithPaths(p, nil)
	if err != nil {
		t.Fatalf("NewServiceWithPaths() error = %v", err)
	}

	tests := []struct {
		name     string
		filePath string
		wantErr  bool
		errType  error
	}{
		{
			name:     "invalid file path",
			filePath: "/nonexistent/file.mkv",
			wantErr:  true,
			errType:  probe.ErrInvalidFile,
		},
		{
			name:     "empty file path",
			filePath: "",
			wantErr:  true,
			errType:  probe.ErrInvalidFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tracks, err := svc.ExtractTracks(ctx, tt.filePath)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractTracks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errType != nil {
				if !errors.Is(err, tt.errType) {
					t.Errorf("ExtractTracks() error type = %T, want %T", err, tt.errType)
				}
			}

			if !tt.wantErr && tracks == nil {
				t.Error("ExtractTracks() returned nil tracks for valid file")
			}
		})
	}
}

// TestServiceGenerateThumbnail tests the GenerateThumbnail delegation.
func TestServiceGenerateThumbnail(t *testing.T) {
	// Skip if ffmpeg is not available
	if _, err := os.Stat("/usr/bin/ffmpeg"); os.IsNotExist(err) {
		t.Skip("ffmpeg not found at /usr/bin/ffmpeg, skipping integration test")
	}

	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}

	svc, err := NewServiceWithPaths(p, nil)
	if err != nil {
		t.Fatalf("NewServiceWithPaths() error = %v", err)
	}

	tests := []struct {
		name       string
		videoPath  string
		outputPath string
		opts       ThumbnailOptions
		wantErr    bool
		errType    error
	}{
		{
			name:       "invalid video path",
			videoPath:  "/nonexistent/video.mkv",
			outputPath: "/tmp/thumb.jpg",
			opts: ThumbnailOptions{
				Timestamp: 30 * time.Second,
				Width:     320,
			},
			wantErr: true,
			errType: thumbnail.ErrInvalidFile,
		},
		{
			name:       "empty video path",
			videoPath:  "",
			outputPath: "/tmp/thumb.jpg",
			opts: ThumbnailOptions{
				Timestamp: 30 * time.Second,
			},
			wantErr: true,
			errType: thumbnail.ErrInvalidFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := svc.GenerateThumbnail(ctx, tt.videoPath, tt.outputPath, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateThumbnail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errType != nil {
				if !errors.Is(err, tt.errType) {
					t.Errorf("GenerateThumbnail() error type = %T, want %T", err, tt.errType)
				}
			}
		})
	}
}

// TestReExportedTypes verifies that re-exported types are accessible.
func TestReExportedTypes(t *testing.T) {
	// Test that types can be instantiated
	var _ Paths
	var _ VideoMetadata
	var _ AudioTrackInfo
	var _ SubtitleTrackInfo
	var _ MediaTracksInfo
	var _ ThumbnailOptions

	// Test concrete values
	p := Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}
	if p.FFmpeg == "" {
		t.Error("Paths type not working correctly")
	}

	vm := VideoMetadata{
		Width:  1920,
		Height: 1080,
	}
	if vm.Width != 1920 {
		t.Error("VideoMetadata type not working correctly")
	}

	at := AudioTrackInfo{
		StreamIndex: 0,
		Codec:       "aac",
	}
	if at.Codec != "aac" {
		t.Error("AudioTrackInfo type not working correctly")
	}

	st := SubtitleTrackInfo{
		StreamIndex: 0,
		Codec:       "subrip",
	}
	if st.Codec != "subrip" {
		t.Error("SubtitleTrackInfo type not working correctly")
	}

	mti := MediaTracksInfo{
		AudioTracks:    []AudioTrackInfo{at},
		SubtitleTracks: []SubtitleTrackInfo{st},
	}
	if len(mti.AudioTracks) != 1 {
		t.Error("MediaTracksInfo type not working correctly")
	}

	opts := ThumbnailOptions{
		Timestamp: 30 * time.Second,
		Width:     320,
		Quality:   2,
	}
	if opts.Width != 320 {
		t.Error("ThumbnailOptions type not working correctly")
	}
}

// TestReExportedErrors verifies that re-exported errors are accessible.
func TestReExportedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "ErrInvalidFile",
			err:  ErrInvalidFile,
			want: "invalid or inaccessible file path",
		},
		{
			name: "ErrMetadataExtraction",
			err:  ErrMetadataExtraction,
			want: "failed to extract metadata from file",
		},
		{
			name: "ErrThumbnailGeneration",
			err:  ErrThumbnailGeneration,
			want: "failed to generate thumbnail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Error("error is nil")
			}

			if tt.err.Error() != tt.want {
				t.Errorf("error message = %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}

// TestReExportedFunctions verifies that re-exported functions are accessible.
func TestReExportedFunctions(t *testing.T) {
	// Test NewPaths function is accessible
	if NewPaths == nil {
		t.Error("NewPaths function is nil")
	}

	// Verify it's the same as paths.New
	_, err1 := NewPaths()
	_, err2 := paths.New()

	// Both should succeed or both should fail
	if (err1 == nil) != (err2 == nil) {
		t.Error("NewPaths and paths.New behave differently")
	}
}

// TestServiceLoggerFallback tests that the service handles nil logger properly.
func TestServiceLoggerFallback(t *testing.T) {
	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}

	// Test with nil logger
	svc, err := NewServiceWithPaths(p, nil)
	if err != nil {
		t.Fatalf("NewServiceWithPaths() error = %v", err)
	}

	if svc.logger == nil {
		t.Error("Service logger is nil after creation with nil logger")
	}

	// Verify logger works (doesn't panic)
	svc.logger.Info("test message")
}

// TestServiceLoggingBehavior tests the logging behavior for custom vs system builds.
func TestServiceLoggingBehavior(t *testing.T) {
	tests := []struct {
		name       string
		paths      *paths.Paths
		wantCustom bool
	}{
		{
			name: "custom build with lib path",
			paths: &paths.Paths{
				FFmpeg:  "/custom/bin/ffmpeg",
				FFprobe: "/custom/bin/ffprobe",
				LibPath: "/custom/lib",
			},
			wantCustom: true,
		},
		{
			name: "system build without lib path",
			paths: &paths.Paths{
				FFmpeg:  "/usr/bin/ffmpeg",
				FFprobe: "/usr/bin/ffprobe",
			},
			wantCustom: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture log output
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}))

			svc, err := NewServiceWithPaths(tt.paths, logger)
			if err != nil {
				t.Fatalf("NewServiceWithPaths() error = %v", err)
			}

			if svc == nil {
				t.Fatal("NewServiceWithPaths() returned nil service")
			}

			// Verify paths are used
			if svc.paths.IsCustomBuild() != tt.wantCustom {
				t.Errorf("IsCustomBuild() = %v, want %v", svc.paths.IsCustomBuild(), tt.wantCustom)
			}
		})
	}
}

// TestNewServiceLogging tests that NewService logs correctly for custom and system builds.
func TestNewServiceLogging(t *testing.T) {
	tests := []struct {
		name           string
		setupEnv       func()
		cleanupEnv     func()
		wantLogContain string
	}{
		{
			name: "logs ffmpeg-viewra when VIEWRA_FFMPEG_PATH set",
			setupEnv: func() {
				os.Setenv("VIEWRA_FFMPEG_PATH", "/custom/bin/ffmpeg")
				os.Setenv("VIEWRA_FFPROBE_PATH", "/custom/bin/ffprobe")
				os.Setenv("VIEWRA_FFMPEG_LIB_PATH", "/custom/lib")
			},
			cleanupEnv: func() {
				os.Unsetenv("VIEWRA_FFMPEG_PATH")
				os.Unsetenv("VIEWRA_FFPROBE_PATH")
				os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")
			},
			wantLogContain: "ffmpeg-viewra",
		},
		{
			name: "logs system ffmpeg when env vars not set",
			setupEnv: func() {
				// Ensure env vars are not set
				os.Unsetenv("VIEWRA_FFMPEG_PATH")
				os.Unsetenv("VIEWRA_FFPROBE_PATH")
				os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")
			},
			wantLogContain: "system ffmpeg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			if tt.cleanupEnv != nil {
				defer tt.cleanupEnv()
			}

			// Capture log output
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}))

			_, err := NewService(logger)
			if err != nil {
				t.Skipf("NewService() error = %v, skipping (likely ffmpeg not available)", err)
				return
			}

			logOutput := buf.String()
			if !bytes.Contains([]byte(logOutput), []byte(tt.wantLogContain)) {
				t.Errorf("Log output does not contain %q\nGot: %s", tt.wantLogContain, logOutput)
			}
		})
	}
}

// TestServiceInitialization tests that all components are properly initialized.
func TestServiceInitialization(t *testing.T) {
	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}

	svc, err := NewServiceWithPaths(p, nil)
	if err != nil {
		t.Fatalf("NewServiceWithPaths() error = %v", err)
	}

	// Verify all fields are initialized
	if svc.paths == nil {
		t.Error("Service.paths is nil")
	}

	if svc.probe == nil {
		t.Error("Service.probe is nil")
	}

	if svc.thumbnail == nil {
		t.Error("Service.thumbnail is nil")
	}

	if svc.logger == nil {
		t.Error("Service.logger is nil")
	}

	// Verify paths are correctly set
	if svc.paths != p {
		t.Error("Service.paths does not match provided paths")
	}

	// Verify clients are usable
	probeClient := svc.ProbeClient()
	if probeClient == nil {
		t.Error("ProbeClient() returned nil")
	}

	thumbnailClient := svc.ThumbnailClient()
	if thumbnailClient == nil {
		t.Error("ThumbnailClient() returned nil")
	}

	paths := svc.Paths()
	if paths == nil {
		t.Error("Paths() returned nil")
	}
}

// TestServiceContextHandling tests that context is properly passed to underlying clients.
func TestServiceContextHandling(t *testing.T) {
	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}

	svc, err := NewServiceWithPaths(p, nil)
	if err != nil {
		t.Fatalf("NewServiceWithPaths() error = %v", err)
	}

	tests := []struct {
		name string
		ctx  context.Context
	}{
		{
			name: "with background context",
			ctx:  context.Background(),
		},
		{
			name: "with timeout context",
			ctx: func() context.Context {
				ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
				return ctx
			}(),
		},
		{
			name: "with canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These should accept the context without panic
			_, _ = svc.ExtractMetadata(tt.ctx, "/nonexistent/file.mkv")
			_, _ = svc.ExtractTracks(tt.ctx, "/nonexistent/file.mkv")
			_ = svc.GenerateThumbnail(tt.ctx, "/nonexistent/video.mkv", "/tmp/thumb.jpg", ThumbnailOptions{})
		})
	}
}

// TestNewServiceErrorHandling tests error handling in NewService.
func TestNewServiceErrorHandling(t *testing.T) {
	// Save original env
	origFFmpeg := os.Getenv("VIEWRA_FFMPEG_PATH")
	origFFprobe := os.Getenv("VIEWRA_FFPROBE_PATH")
	origLibPath := os.Getenv("VIEWRA_FFMPEG_LIB_PATH")
	origPath := os.Getenv("PATH")

	defer func() {
		// Restore original env
		if origFFmpeg != "" {
			os.Setenv("VIEWRA_FFMPEG_PATH", origFFmpeg)
		} else {
			os.Unsetenv("VIEWRA_FFMPEG_PATH")
		}
		if origFFprobe != "" {
			os.Setenv("VIEWRA_FFPROBE_PATH", origFFprobe)
		} else {
			os.Unsetenv("VIEWRA_FFPROBE_PATH")
		}
		if origLibPath != "" {
			os.Setenv("VIEWRA_FFMPEG_LIB_PATH", origLibPath)
		} else {
			os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")
		}
		os.Setenv("PATH", origPath)
	}()

	// Clear all ffmpeg env vars and PATH to force paths.New() to fail
	os.Unsetenv("VIEWRA_FFMPEG_PATH")
	os.Unsetenv("VIEWRA_FFPROBE_PATH")
	os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")
	os.Setenv("PATH", "")

	_, err := NewService(nil)
	if err == nil {
		t.Error("NewService() expected error when ffmpeg not found, got nil")
	}
}

// TestNewServiceWithPathsErrorHandling tests error handling when nil paths triggers NewService.
func TestNewServiceWithPathsErrorHandling(t *testing.T) {
	// Save original env
	origPath := os.Getenv("PATH")

	defer func() {
		os.Setenv("PATH", origPath)
	}()

	// Clear PATH to force NewService to fail when called from NewServiceWithPaths
	os.Unsetenv("VIEWRA_FFMPEG_PATH")
	os.Unsetenv("VIEWRA_FFPROBE_PATH")
	os.Setenv("PATH", "")

	_, err := NewServiceWithPaths(nil, nil)
	if err == nil {
		t.Error("NewServiceWithPaths(nil, nil) expected error when ffmpeg not found, got nil")
	}
}

// TestServiceMethodsReturnCorrectTypes tests that all service methods return the correct types.
func TestServiceMethodsReturnCorrectTypes(t *testing.T) {
	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}

	svc, err := NewServiceWithPaths(p, nil)
	if err != nil {
		t.Fatalf("NewServiceWithPaths() error = %v", err)
	}

	// Test Paths() returns *paths.Paths
	var _ *paths.Paths = svc.Paths()

	// Test ProbeClient() returns *probe.Client
	var _ *probe.Client = svc.ProbeClient()

	// Test ThumbnailClient() returns *thumbnail.Client
	var _ *thumbnail.Client = svc.ThumbnailClient()

	// Test ExtractMetadata returns correct types
	ctx := context.Background()
	_, err = svc.ExtractMetadata(ctx, "/nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}

	// Test ExtractTracks returns correct types
	_, err = svc.ExtractTracks(ctx, "/nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}

	// Test GenerateThumbnail accepts correct types
	err = svc.GenerateThumbnail(ctx, "/nonexistent", "/tmp/out.jpg", ThumbnailOptions{
		Timestamp: 30 * time.Second,
		Width:     320,
		Height:    180,
		Quality:   2,
	})
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

// BenchmarkNewService benchmarks service creation.
func BenchmarkNewService(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewService(nil)
	}
}

// BenchmarkNewServiceWithPaths benchmarks service creation with paths.
func BenchmarkNewServiceWithPaths(b *testing.B) {
	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewServiceWithPaths(p, nil)
	}
}

// BenchmarkServiceOperations benchmarks service operations.
func BenchmarkServiceOperations(b *testing.B) {
	p := &paths.Paths{
		FFmpeg:  "/usr/bin/ffmpeg",
		FFprobe: "/usr/bin/ffprobe",
	}

	svc, err := NewServiceWithPaths(p, nil)
	if err != nil {
		b.Fatalf("NewServiceWithPaths() error = %v", err)
	}

	b.Run("Paths", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = svc.Paths()
		}
	})

	b.Run("ProbeClient", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = svc.ProbeClient()
		}
	})

	b.Run("ThumbnailClient", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = svc.ThumbnailClient()
		}
	})
}
