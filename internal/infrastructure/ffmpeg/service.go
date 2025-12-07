// Package ffmpeg provides video processing functionality using FFmpeg.
//
// This package exposes functionality from sub-packages:
//   - [paths]: FFmpeg/FFprobe binary resolution and command preparation
//   - [probe]: Video metadata extraction using ffprobe
//   - [thumbnail]: Thumbnail generation from video files
//   - [hls]: HLS transcoding command building
//   - [process]: FFmpeg process lifecycle management
//
// The main entry point is [Service], which provides a unified interface
// for common FFmpeg operations.
//
// Example:
//
//	svc, err := ffmpeg.NewService(logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Extract metadata
//	metadata, err := svc.ExtractMetadata(ctx, "/path/to/video.mkv")
//
//	// Generate thumbnail
//	err = svc.GenerateThumbnail(ctx, "/path/to/video.mkv", "/path/to/thumb.jpg", ffmpeg.ThumbnailOptions{
//	    Timestamp: 30 * time.Second,
//	    Width:     320,
//	})
package ffmpeg

import (
	"context"
	"log/slog"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/paths"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/probe"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/thumbnail"
	pkgLogger "github.com/mantonx/viewra/internal/pkg/logger"
)

// Re-export types from sub-packages for convenience.
// Consumers can import just the ffmpeg package for common operations.
type (
	// Paths holds FFmpeg and FFprobe binary paths with optional library path.
	Paths = paths.Paths

	// VideoMetadata contains extracted metadata from a video file.
	VideoMetadata = probe.VideoMetadata

	// AudioTrackInfo contains metadata for an audio stream.
	AudioTrackInfo = probe.AudioTrackInfo

	// SubtitleTrackInfo contains metadata for a subtitle stream.
	SubtitleTrackInfo = probe.SubtitleTrackInfo

	// MediaTracksInfo contains all audio and subtitle tracks from a media file.
	MediaTracksInfo = probe.MediaTracksInfo

	// ThumbnailOptions configures thumbnail generation.
	ThumbnailOptions = thumbnail.Options
)

// Re-export errors from sub-packages.
var (
	ErrInvalidFile         = probe.ErrInvalidFile
	ErrMetadataExtraction  = probe.ErrMetadataExtraction
	ErrThumbnailGeneration = thumbnail.ErrThumbnailGeneration
)

// Re-export path functions.
var (
	// NewPaths resolves FFmpeg/FFprobe paths from environment or system PATH.
	NewPaths = paths.New
)

// Service provides a unified interface for FFmpeg operations.
type Service struct {
	paths     *paths.Paths
	probe     *probe.Client
	thumbnail *thumbnail.Client
	logger    *slog.Logger
}

// NewService creates a new FFmpeg service.
// It verifies that FFmpeg is available and logs configuration.
func NewService(logger *slog.Logger) (*Service, error) {
	logger = pkgLogger.DefaultIfNil(logger)

	p, err := paths.New()
	if err != nil {
		return nil, err
	}

	probeClient, err := probe.NewClient(p)
	if err != nil {
		return nil, err
	}

	thumbnailClient, err := thumbnail.NewClient(p)
	if err != nil {
		return nil, err
	}

	// Log configuration once
	version := p.Version()
	if p.IsCustomBuild() {
		logger.Info("using ffmpeg-viewra",
			"path", p.FFmpeg,
			"version", version,
			"lib_path", p.LibPath,
		)
	} else {
		logger.Info("using system ffmpeg",
			"path", p.FFmpeg,
			"version", version,
		)
	}

	return &Service{
		paths:     p,
		probe:     probeClient,
		thumbnail: thumbnailClient,
		logger:    logger,
	}, nil
}

// NewServiceWithPaths creates a new FFmpeg service with explicit paths.
// Use this when you already have a Paths instance to avoid duplicate resolution.
func NewServiceWithPaths(p *paths.Paths, logger *slog.Logger) (*Service, error) {
	logger = pkgLogger.DefaultIfNil(logger)

	if p == nil {
		return NewService(logger)
	}

	probeClient, err := probe.NewClient(p)
	if err != nil {
		return nil, err
	}

	thumbnailClient, err := thumbnail.NewClient(p)
	if err != nil {
		return nil, err
	}

	return &Service{
		paths:     p,
		probe:     probeClient,
		thumbnail: thumbnailClient,
		logger:    logger,
	}, nil
}

// Paths returns the underlying Paths configuration.
func (s *Service) Paths() *paths.Paths {
	return s.paths
}

// ExtractMetadata extracts metadata from a video file using ffprobe.
func (s *Service) ExtractMetadata(ctx context.Context, filePath string) (*probe.VideoMetadata, error) {
	return s.probe.ExtractMetadata(ctx, filePath)
}

// ExtractTracks extracts audio and subtitle track information from a media file.
func (s *Service) ExtractTracks(ctx context.Context, filePath string) (*probe.MediaTracksInfo, error) {
	return s.probe.ExtractTracks(ctx, filePath)
}

// GenerateThumbnail generates a thumbnail image from a video file.
// The output path should include the desired image extension (e.g., .jpg, .png).
func (s *Service) GenerateThumbnail(ctx context.Context, videoPath, outputPath string, opts thumbnail.Options) error {
	return s.thumbnail.Generate(ctx, videoPath, outputPath, opts)
}

// ProbeClient returns the underlying probe client for advanced usage.
func (s *Service) ProbeClient() *probe.Client {
	return s.probe
}

// ThumbnailClient returns the underlying thumbnail client for advanced usage.
func (s *Service) ThumbnailClient() *thumbnail.Client {
	return s.thumbnail
}
