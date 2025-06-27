package playbackmodule

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/internal/plugins/ffmpeg"
)

// MediaAnalyzer interface for media file analysis
type MediaAnalyzer interface {
	AnalyzeMedia(mediaPath string) (*MediaInfo, error)
}

// SimpleMediaAnalyzer provides basic analysis for development and fallback
type SimpleMediaAnalyzer struct{}

// NewSimpleMediaAnalyzer creates a simple media analyzer
func NewSimpleMediaAnalyzer() MediaAnalyzer {
	return &SimpleMediaAnalyzer{}
}

// FFProbeMediaAnalyzer uses the FFmpeg probe plugin for real media analysis
type FFProbeMediaAnalyzer struct {
	ffmpegPlugin *ffmpeg.FFmpegCorePlugin
	fallback     MediaAnalyzer
}

// NewFFProbeMediaAnalyzer creates an FFprobe-based media analyzer with fallback
func NewFFProbeMediaAnalyzer() MediaAnalyzer {
	plugin := ffmpeg.NewFFmpegCorePlugin().(*ffmpeg.FFmpegCorePlugin)
	plugin.Initialize() // Initialize the FFmpeg plugin

	return &FFProbeMediaAnalyzer{
		ffmpegPlugin: plugin,
		fallback:     NewSimpleMediaAnalyzer(),
	}
}

// AnalyzeMedia uses FFprobe to extract real media information
func (a *FFProbeMediaAnalyzer) AnalyzeMedia(mediaPath string) (*MediaInfo, error) {
	// Try FFprobe-based analysis first
	if a.ffmpegPlugin != nil {
		info, err := a.extractWithFFProbe(mediaPath)
		if err == nil {
			return info, nil
		}

		// Log the error but continue with fallback
		fmt.Printf("FFprobe analysis failed for %s, using fallback: %v\n", mediaPath, err)
	}

	// Fall back to simple analysis
	return a.fallback.AnalyzeMedia(mediaPath)
}

// extractWithFFProbe uses the FFmpeg plugin to extract detailed media information
func (a *FFProbeMediaAnalyzer) extractWithFFProbe(mediaPath string) (*MediaInfo, error) {
	ext := strings.ToLower(filepath.Ext(mediaPath))

	// Check if this is an audio file
	if a.isAudioFile(ext) {
		return a.extractAudioInfo(mediaPath)
	}

	// For video files, extract comprehensive information
	return a.extractVideoInfo(mediaPath)
}

// extractAudioInfo extracts information for audio files
func (a *FFProbeMediaAnalyzer) extractAudioInfo(mediaPath string) (*MediaInfo, error) {
	// Use the FFmpeg plugin's audio extraction method
	audioInfo, err := a.ffmpegPlugin.ExtractAudioTechnicalInfo(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract audio info: %w", err)
	}

	info := &MediaInfo{
		Container:    audioInfo.Format,
		VideoCodec:   "", // No video codec for audio files
		AudioCodec:   audioInfo.Codec,
		Resolution:   "", // No resolution for audio files
		Bitrate:      int64(audioInfo.Bitrate),
		Duration:     int64(audioInfo.Duration),
		HasHDR:       false, // Audio files don't have HDR
		HasSubtitles: false, // Audio files don't have subtitles typically
	}

	return info, nil
}

// extractVideoInfo extracts information for video files
func (a *FFProbeMediaAnalyzer) extractVideoInfo(mediaPath string) (*MediaInfo, error) {
	// Use the FFmpeg plugin's comprehensive extraction method
	videoInfo, err := a.ffmpegPlugin.ExtractComprehensiveTechnicalInfo(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract video info: %w", err)
	}

	info := &MediaInfo{
		Container:    videoInfo.Container,
		VideoCodec:   videoInfo.VideoCodec,
		AudioCodec:   videoInfo.AudioCodec,
		Resolution:   videoInfo.Resolution,
		Bitrate:      int64(videoInfo.Bitrate),
		Duration:     int64(videoInfo.Duration),
		HasHDR:       a.detectHDR(videoInfo),
		HasSubtitles: videoInfo.HasSubtitles,
	}

	return info, nil
}

// detectHDR determines if the video has HDR content
func (a *FFProbeMediaAnalyzer) detectHDR(videoInfo *ffmpeg.VideoTechnicalInfo) bool {
	for _, stream := range videoInfo.VideoStreams {
		if stream.HDRFormat != "" {
			return true
		}
		// Also check color characteristics that indicate HDR
		if stream.ColorSpace == "bt2020nc" || stream.ColorSpace == "bt2020c" ||
			stream.ColorTransfer == "smpte2084" || stream.ColorTransfer == "arib-std-b67" {
			return true
		}
	}
	return false
}

// isAudioFile determines if a file is an audio file based on extension
func (a *FFProbeMediaAnalyzer) isAudioFile(ext string) bool {
	audioExts := map[string]bool{
		".mp3": true, ".flac": true, ".wav": true, ".m4a": true, ".aac": true,
		".ogg": true, ".wma": true, ".opus": true, ".aiff": true, ".ape": true, ".wv": true,
	}
	return audioExts[ext]
}

// AnalyzeMedia provides analysis based on file extension and conservative defaults
func (a *SimpleMediaAnalyzer) AnalyzeMedia(mediaPath string) (*MediaInfo, error) {
	ext := strings.ToLower(filepath.Ext(mediaPath))

	info := &MediaInfo{
		Container:    getContainerFromExtension(ext),
		VideoCodec:   "h264",  // Conservative default for compatibility
		AudioCodec:   "aac",   // Conservative default for compatibility
		Resolution:   "1080p", // Conservative default
		Bitrate:      6000000, // 6 Mbps conservative default
		Duration:     3600,    // 1 hour default
		HasHDR:       false,   // Conservative default
		HasSubtitles: false,   // Conservative default
	}

	return info, nil
}

// getContainerFromExtension determines container format from file extension
func getContainerFromExtension(ext string) string {
	switch ext {
	case ".mp4", ".m4v":
		return "mp4"
	case ".mkv":
		return "mkv"
	case ".avi":
		return "avi"
	case ".webm":
		return "webm"
	case ".mov":
		return "mov"
	case ".flv":
		return "flv"
	case ".wmv":
		return "wmv"
	case ".m4a":
		return "m4a"
	case ".mp3":
		return "mp3"
	case ".flac":
		return "flac"
	case ".wav":
		return "wav"
	default:
		return "unknown"
	}
}
