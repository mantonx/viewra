package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"

	plugins "github.com/mantonx/viewra/sdk"
	"github.com/mantonx/viewra/sdk/transcoding"
)

// QsvTranscoder provides Intel Quick Sync Video hardware-accelerated transcoding
type QsvTranscoder struct {
	name        string
	description string
	version     string
	author      string
	priority    int
	transcoder  *transcoding.Transcoder
	// Hardware detection functionality removed - use module implementation
}

// Plugin implementation
func (p *QsvTranscoder) Initialize(ctx *plugins.PluginContext) error {
	// Check if Intel QSV hardware is available
	cmd := exec.Command("vainfo")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Intel QSV not detected: %w", err)
	}
	
	// Initialize the transcoder with QSV-specific settings
	p.transcoder = transcoding.NewTranscoder(
		p.name, 
		p.description, 
		p.version, 
		p.author, 
		p.priority,
	)
	// Wrap the plugin logger to match SDK logger interface
	loggerAdapter := &loggerAdapter{logger: ctx.Logger}
	p.transcoder.SetLogger(loggerAdapter)
	
	// QSV availability check moved to module implementation
	
	ctx.Logger.Info("ffmpeg qsv transcoder plugin initialized with Intel QSV support")
	return nil
}

func (p *QsvTranscoder) Start() error {
	return nil
}

func (p *QsvTranscoder) Stop() error {
	return nil
}

func (p *QsvTranscoder) Info() (*plugins.PluginInfo, error) {
	return &plugins.PluginInfo{
		ID:          "ffmpeg_qsv",
		Name:        "FFmpeg QSV Transcoder",
		Version:     "1.0.0",
		Description: "Intel Quick Sync Video hardware-accelerated transcoding",
		Author:      "Viewra Team",
		Type:        "transcoding",
	}, nil
}

func (p *QsvTranscoder) Health() error {
	return nil
}

// Service implementations
func (p *QsvTranscoder) TranscodingProvider() plugins.TranscodingProvider {
	return p
}

// TranscodingProvider interface implementation
func (p *QsvTranscoder) GetInfo() plugins.ProviderInfo {
	return plugins.ProviderInfo{
		ID:          "ffmpeg_qsv",
		Name:        "FFmpeg QSV Transcoder",
		Description: "Intel Quick Sync Video hardware acceleration for fast transcoding",
		Version:     "1.0.0",
		Author:      "Viewra Team",
		Priority:    85, // High priority for hardware acceleration
		Capabilities: []string{
			"h264_qsv",
			"h265_qsv",
			"vp9_qsv",
			"av1_qsv",
			"hardware_acceleration",
			"intel_qsv",
			"fast_encoding",
			"low_latency",
		},
	}
}

func (p *QsvTranscoder) GetSupportedFormats() []plugins.ContainerFormat {
	return []plugins.ContainerFormat{
		{
			Format:      "mp4",
			MimeType:    "video/mp4",
			Extensions:  []string{".mp4"},
			Description: "MPEG-4 Container",
			Adaptive:    false,
		},
		{
			Format:      "webm",
			MimeType:    "video/webm",
			Extensions:  []string{".webm"},
			Description: "WebM Container",
			Adaptive:    false,
		},
		{
			Format:      "mkv",
			MimeType:    "video/x-matroska",
			Extensions:  []string{".mkv"},
			Description: "Matroska Container",
			Adaptive:    false,
		},
		{
			Format:      "dash",
			MimeType:    "application/dash+xml",
			Extensions:  []string{".mpd", ".m4s"},
			Description: "MPEG-DASH Adaptive Streaming",
			Adaptive:    true,
		},
		{
			Format:      "hls",
			MimeType:    "application/vnd.apple.mpegurl",
			Extensions:  []string{".m3u8", ".ts"},
			Description: "HLS Adaptive Streaming",
			Adaptive:    true,
		},
	}
}

func (p *QsvTranscoder) GetHardwareAccelerators() []plugins.HardwareAccelerator {
	return []plugins.HardwareAccelerator{
		{
			Type:        "qsv",
			ID:          "intel_qsv",
			Name:        "Intel Quick Sync Video",
			Available:   true, // Would check for Intel QSV support in real implementation
			DeviceCount: 1,    // Would detect actual QSV-capable devices
		},
	}
}

func (p *QsvTranscoder) GetQualityPresets() []plugins.QualityPreset {
	return []plugins.QualityPreset{
		{
			ID:          "speed",
			Name:        "Speed",
			Description: "Optimized for maximum encoding speed",
			Quality:     45,
			SpeedRating: 10,
			SizeRating:  3,
		},
		{
			ID:          "balanced",
			Name:        "Balanced",
			Description: "Good balance of speed, quality, and file size",
			Quality:     65,
			SpeedRating: 7,
			SizeRating:  6,
		},
		{
			ID:          "quality",
			Name:        "Quality",
			Description: "Optimized for better quality with moderate speed",
			Quality:     80,
			SpeedRating: 5,
			SizeRating:  8,
		},
		{
			ID:          "veryslow",
			Name:        "Best Quality",
			Description: "Maximum quality encoding with slower speed",
			Quality:     90,
			SpeedRating: 3,
			SizeRating:  9,
		},
	}
}

// StartTranscode starts a transcoding session using Intel QSV
func (p *QsvTranscoder) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	if p.transcoder == nil {
		return nil, fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to transcoding types with QSV-specific settings
	transcodingReq := transcoding.TranscodeRequest{
		MediaID:        req.MediaID,
		SessionID:      req.SessionID,
		InputPath:      req.InputPath,
		OutputPath:     req.OutputPath,
		Container:      req.Container,
		VideoCodec:     p.getQSVCodec(req.VideoCodec), // Map to QSV codec
		AudioCodec:     req.AudioCodec,
		Resolution:     req.Resolution,
		Quality:        req.Quality,
		SpeedPriority:  transcoding.SpeedPriority(req.SpeedPriority),
		Seek:           req.Seek,
		Duration:       req.Duration,
		EnableABR:      req.EnableABR,
		PreferHardware: true, // Always prefer hardware for QSV plugin
		HardwareType:   transcoding.HardwareTypeQSV,
		VideoBitrate:   req.VideoBitrate,
		AudioBitrate:   req.AudioBitrate,
	}
	
	// Add QSV-specific encoding parameters
	hardwareParams := map[string]interface{}{
		"preset": p.getQSVPreset(int(req.SpeedPriority)),
		"profile": "high",
		"look_ahead": "1", // Enable look-ahead for better quality
		"rdo": "1",       // Rate-distortion optimization
	}
	// Marshal as JSON for ProviderSettings
	if paramsJSON, err := json.Marshal(hardwareParams); err == nil {
		transcodingReq.ProviderSettings = paramsJSON
	}
	
	handle, err := p.transcoder.StartTranscode(ctx, transcodingReq)
	if err != nil {
		return nil, fmt.Errorf("failed to start QSV transcode: %w", err)
	}
	
	// Convert to plugin handle
	return &plugins.TranscodeHandle{
		SessionID:   handle.SessionID,
		Provider:    "ffmpeg_qsv",
		StartTime:   handle.StartTime,
		Directory:   handle.Directory,
		Context:     handle.Context,
		CancelFunc:  handle.CancelFunc,
		PrivateData: handle.PrivateData,
		Status:      plugins.TranscodeStatus(handle.Status),
		Error:       handle.Error,
	}, nil
}

func (p *QsvTranscoder) GetProgress(handle *plugins.TranscodeHandle) (*plugins.TranscodingProgress, error) {
	if p.transcoder == nil {
		return nil, fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to SDK handle
	sdkHandle := &transcoding.TranscodeHandle{
		SessionID:   handle.SessionID,
		Provider:    handle.Provider,
		StartTime:   handle.StartTime,
		Directory:   handle.Directory,
		Context:     handle.Context,
		PrivateData: handle.PrivateData,
	}
	
	progress, err := p.transcoder.GetProgress(sdkHandle)
	if err != nil {
		return nil, err
	}
	
	// Convert to plugin progress
	return &plugins.TranscodingProgress{
		PercentComplete: progress.PercentComplete,
		TimeElapsed:     progress.TimeElapsed,
		TimeRemaining:   progress.TimeRemaining,
		BytesRead:       progress.BytesRead,
		BytesWritten:    progress.BytesWritten,
		CurrentSpeed:    progress.CurrentSpeed,
		AverageSpeed:    progress.AverageSpeed,
	}, nil
}

func (p *QsvTranscoder) StopTranscode(handle *plugins.TranscodeHandle) error {
	if p.transcoder == nil {
		return fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to SDK handle
	sdkHandle := &transcoding.TranscodeHandle{
		SessionID:   handle.SessionID,
		Provider:    handle.Provider,
		StartTime:   handle.StartTime,
		Directory:   handle.Directory,
		Context:     handle.Context,
		CancelFunc:  handle.CancelFunc,
		PrivateData: handle.PrivateData,
	}
	
	return p.transcoder.StopTranscode(sdkHandle)
}

func (p *QsvTranscoder) StartStream(ctx context.Context, req plugins.TranscodeRequest) (*plugins.StreamHandle, error) {
	if p.transcoder == nil {
		return nil, fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to SDK types with QSV-specific settings
	transcodingReq := transcoding.TranscodeRequest{
		MediaID:        req.MediaID,
		SessionID:      req.SessionID,
		InputPath:      req.InputPath,
		Container:      req.Container,
		VideoCodec:     p.getQSVCodec(req.VideoCodec),
		AudioCodec:     req.AudioCodec,
		Resolution:     req.Resolution,
		Quality:        req.Quality,
		SpeedPriority:  transcoding.SpeedPriority(req.SpeedPriority),
		Seek:           req.Seek,
		Duration:       req.Duration,
		EnableABR:      req.EnableABR,
		PreferHardware: true,
		HardwareType:   transcoding.HardwareTypeQSV,
		VideoBitrate:   req.VideoBitrate,
		AudioBitrate:   req.AudioBitrate,
	}
	
	// Add QSV-specific streaming parameters
	streamParams := map[string]interface{}{
		"preset": "speed",     // Optimized for low latency
		"look_ahead": "0",    // Disable look-ahead for streaming
		"low_delay_hrd": "1", // Low delay mode
	}
	// Marshal as JSON for ProviderSettings
	if paramsJSON, err := json.Marshal(streamParams); err == nil {
		transcodingReq.ProviderSettings = paramsJSON
	}
	
	handle, err := p.transcoder.StartStream(ctx, transcodingReq)
	if err != nil {
		return nil, fmt.Errorf("failed to start QSV stream: %w", err)
	}
	
	// Convert to plugin handle
	return &plugins.StreamHandle{
		SessionID:   handle.SessionID,
		Provider:    "ffmpeg_qsv",
		StartTime:   time.Now(),
		PrivateData: handle.PrivateData,
	}, nil
}

func (p *QsvTranscoder) GetStream(handle *plugins.StreamHandle) (io.ReadCloser, error) {
	if p.transcoder == nil {
		return nil, fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to SDK handle
	sdkHandle := &transcoding.StreamHandle{
		SessionID:   handle.SessionID,
		PrivateData: handle.PrivateData,
	}
	
	return p.transcoder.GetStream(sdkHandle)
}

func (p *QsvTranscoder) StopStream(handle *plugins.StreamHandle) error {
	if p.transcoder == nil {
		return fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to SDK handle
	sdkHandle := &transcoding.StreamHandle{
		SessionID:   handle.SessionID,
		PrivateData: handle.PrivateData,
	}
	
	return p.transcoder.StopStream(sdkHandle)
}

func (p *QsvTranscoder) GetDashboardSections() []plugins.DashboardSection {
	return []plugins.DashboardSection{
		{
			ID:    "overview",
			Title: "QSV Transcoder Overview",
			Type:  "stats",
		},
		{
			ID:    "performance",
			Title: "QSV Performance Metrics",
			Type:  "chart",
		},
	}
}

func (p *QsvTranscoder) GetDashboardData(sectionID string) (interface{}, error) {
	switch sectionID {
	case "overview":
		return map[string]interface{}{
			"active_sessions": 0,
			"completed_jobs":  0,
			"qsv_usage":       0.0,
			"throughput":      0.0,
		}, nil
	case "performance":
		return map[string]interface{}{
			"fps_current": 0.0,
			"fps_average": 0.0,
			"fps_peak":    0.0,
		}, nil
	default:
		return nil, nil
	}
}

func (p *QsvTranscoder) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	return nil
}

// Return nil for unsupported services
func (p *QsvTranscoder) MetadataScraperService() plugins.MetadataScraperService         { return nil }
func (p *QsvTranscoder) ScannerHookService() plugins.ScannerHookService                 { return nil }
func (p *QsvTranscoder) AssetService() plugins.AssetService                             { return nil }
func (p *QsvTranscoder) DatabaseService() plugins.DatabaseService                       { return nil }
func (p *QsvTranscoder) AdminPageService() plugins.AdminPageService                     { return nil }
func (p *QsvTranscoder) APIRegistrationService() plugins.APIRegistrationService         { return nil }
func (p *QsvTranscoder) SearchService() plugins.SearchService                           { return nil }
func (p *QsvTranscoder) HealthMonitorService() plugins.HealthMonitorService             { return nil }
func (p *QsvTranscoder) ConfigurationService() plugins.ConfigurationService             { return nil }
func (p *QsvTranscoder) PerformanceMonitorService() plugins.PerformanceMonitorService   { return nil }
func (p *QsvTranscoder) EnhancedAdminPageService() plugins.EnhancedAdminPageService     { return nil }

// Plugin factory function
func NewQsvTranscoder() plugins.Implementation {
	return &QsvTranscoder{
		name:        "ffmpeg_qsv",
		description: "FFmpeg QSV Transcoder",
		version:     "1.0.0",
		author:      "Viewra Team",
		priority:    85,
	}
}

// Main function for plugin binary
// getQSVCodec maps generic codec names to QSV-specific encoder names
func (p *QsvTranscoder) getQSVCodec(codec string) string {
	switch codec {
	case "h264", "libx264", "":
		return "h264_qsv"
	case "h265", "hevc", "libx265":
		return "hevc_qsv"
	case "vp9":
		// VP9 QSV availability check moved to module
		// Fall back to H.265 if VP9 not available
		return "hevc_qsv"
	case "av1":
		// AV1 QSV availability check moved to module
		// Fall back to H.265 if AV1 not available
		return "hevc_qsv"
	default:
		// Default to H.264 for unsupported codecs
		return "h264_qsv"
	}
}

// getQSVPreset maps speed priority to QSV preset
func (p *QsvTranscoder) getQSVPreset(speedPriority int) string {
	// QSV presets: veryfast, faster, fast, medium, slow, slower, veryslow
	switch speedPriority {
	case 1: // Fastest
		return "veryfast"
	case 2:
		return "faster"
	case 3:
		return "fast"
	case 4:
		return "medium"
	case 5:
		return "slow"
	case 6:
		return "slower"
	default: // Highest quality
		return "veryslow"
	}
}

// SupportsIntermediateOutput returns true as QSV encoder outputs intermediate MP4 files
func (p *QsvTranscoder) SupportsIntermediateOutput() bool {
	return true
}

// GetIntermediateOutputPath returns the path to intermediate MP4 file
func (p *QsvTranscoder) GetIntermediateOutputPath(handle *plugins.TranscodeHandle) (string, error) {
	if handle == nil {
		return "", fmt.Errorf("handle is nil")
	}
	
	// Return the MP4 file path in the encoded subdirectory
	if handle.Directory != "" {
		return filepath.Join(handle.Directory, "encoded", "output.mp4"), nil
	}
	
	return "", fmt.Errorf("no directory information in handle")
}

// GetABRVariants returns the ABR encoding variants optimized for QSV
func (p *QsvTranscoder) GetABRVariants(req plugins.TranscodeRequest) ([]plugins.ABRVariant, error) {
	var variants []plugins.ABRVariant
	
	// Determine maximum resolution from request
	maxHeight := 1080 // Default
	if req.Resolution != nil {
		maxHeight = req.Resolution.Height
	}
	
	// QSV-optimized ABR ladder
	if maxHeight >= 2160 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "4K",
			Resolution:   &plugins.Resolution{Width: 3840, Height: 2160},
			VideoBitrate: 15000, // Slightly lower for QSV efficiency
			AudioBitrate: 192,
			FrameRate:    30,
			Preset:       "medium", // Balanced preset for 4K
			Profile:      "high",
			Level:        "5.1",
		})
	}
	
	if maxHeight >= 1440 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "1440p",
			Resolution:   &plugins.Resolution{Width: 2560, Height: 1440},
			VideoBitrate: 8000,
			AudioBitrate: 192,
			FrameRate:    30,
			Preset:       "medium",
			Profile:      "high",
			Level:        "4.1",
		})
	}
	
	if maxHeight >= 1080 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "1080p",
			Resolution:   &plugins.Resolution{Width: 1920, Height: 1080},
			VideoBitrate: 5000,
			AudioBitrate: 192,
			FrameRate:    30,
			Preset:       "slow", // Higher quality for 1080p
			Profile:      "high",
			Level:        "4.0",
		})
	}
	
	if maxHeight >= 720 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "720p",
			Resolution:   &plugins.Resolution{Width: 1280, Height: 720},
			VideoBitrate: 2500,
			AudioBitrate: 128,
			FrameRate:    30,
			Preset:       "slow",
			Profile:      "main",
			Level:        "3.1",
		})
	}
	
	if maxHeight >= 480 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "480p",
			Resolution:   &plugins.Resolution{Width: 854, Height: 480},
			VideoBitrate: 1200,
			AudioBitrate: 128,
			FrameRate:    30,
			Preset:       "slower", // Higher quality for lower res
			Profile:      "main",
			Level:        "3.0",
		})
	}
	
	// Always include a low resolution variant
	variants = append(variants, plugins.ABRVariant{
		Name:         "360p",
		Resolution:   &plugins.Resolution{Width: 640, Height: 360},
		VideoBitrate: 600,
		AudioBitrate: 96,
		FrameRate:    30,
		Preset:       "slower",
		Profile:      "baseline",
		Level:        "3.0",
	})
	
	return variants, nil
}

// Main function for plugin binary
func main() {
	plugin := NewQsvTranscoder()
	plugins.StartPlugin(plugin)
}

// loggerAdapter adapts plugins.Logger to transcoding.Logger
type loggerAdapter struct {
	logger plugins.Logger
}

func (l *loggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg, keysAndValues...)
}

func (l *loggerAdapter) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warn(msg, keysAndValues...)
}

func (l *loggerAdapter) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Error(msg, keysAndValues...)
}

func (l *loggerAdapter) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debug(msg, keysAndValues...)
}

func (l *loggerAdapter) With(keysAndValues ...interface{}) transcoding.Logger {
	return &loggerAdapter{
		logger: l.logger.With(keysAndValues...),
	}
}