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

// NvidiaTranscoder provides NVIDIA NVENC hardware-accelerated transcoding
type NvidiaTranscoder struct {
	name        string
	description string
	version     string
	author      string
	priority    int
	transcoder  *transcoding.Transcoder
	// Hardware detection functionality removed - use module implementation
}

// Plugin implementation
func (p *NvidiaTranscoder) Initialize(ctx *plugins.PluginContext) error {
	// Check if NVIDIA hardware is available
	cmd := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("NVIDIA GPU not detected: %w", err)
	}
	
	// Initialize the transcoder with NVIDIA-specific settings
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
	
	// NVENC availability check moved to module implementation
	
	ctx.Logger.Info("ffmpeg nvidia transcoder plugin initialized with NVENC support")
	return nil
}

func (p *NvidiaTranscoder) Start() error {
	return nil
}

func (p *NvidiaTranscoder) Stop() error {
	return nil
}

func (p *NvidiaTranscoder) Info() (*plugins.PluginInfo, error) {
	return &plugins.PluginInfo{
		ID:          "ffmpeg_nvidia",
		Name:        "FFmpeg NVIDIA Transcoder",
		Version:     "1.0.0",
		Description: "High-performance NVIDIA NVENC hardware-accelerated transcoding",
		Author:      "Viewra Team",
		Type:        "transcoding",
	}, nil
}

func (p *NvidiaTranscoder) Health() error {
	return nil
}

// Service implementations
func (p *NvidiaTranscoder) TranscodingProvider() plugins.TranscodingProvider {
	return p
}

// TranscodingProvider interface implementation
func (p *NvidiaTranscoder) GetInfo() plugins.ProviderInfo {
	return plugins.ProviderInfo{
		ID:          "ffmpeg_nvidia",
		Name:        "FFmpeg NVIDIA Transcoder",
		Description: "High-performance hardware transcoding using NVIDIA NVENC",
		Version:     "1.0.0",
		Author:      "Viewra Team",
		Priority:    90, // High priority for hardware acceleration
		Capabilities: []string{
			"h264_nvenc",
			"h265_nvenc",
			"av1_nvenc",
			"hardware_acceleration",
			"fast_encoding",
			"concurrent_sessions",
		},
	}
}

func (p *NvidiaTranscoder) GetSupportedFormats() []plugins.ContainerFormat {
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

func (p *NvidiaTranscoder) GetHardwareAccelerators() []plugins.HardwareAccelerator {
	return []plugins.HardwareAccelerator{
		{
			Type:        "nvidia",
			ID:          "nvenc",
			Name:        "NVIDIA NVENC",
			Available:   true, // Would check nvidia-smi in real implementation
			DeviceCount: 1,    // Would detect actual GPU count
		},
	}
}

func (p *NvidiaTranscoder) GetQualityPresets() []plugins.QualityPreset {
	return []plugins.QualityPreset{
		{
			ID:          "p1",
			Name:        "Fastest (P1)",
			Description: "Maximum speed, lowest quality - best for real-time",
			Quality:     30,
			SpeedRating: 10,
			SizeRating:  2,
		},
		{
			ID:          "p4",
			Name:        "Fast (P4)",
			Description: "Good speed with acceptable quality",
			Quality:     50,
			SpeedRating: 8,
			SizeRating:  4,
		},
		{
			ID:          "p6",
			Name:        "Balanced (P6)",
			Description: "Balanced speed and quality for most use cases",
			Quality:     70,
			SpeedRating: 6,
			SizeRating:  6,
		},
		{
			ID:          "p7",
			Name:        "Quality (P7)",
			Description: "High quality encoding with slower speed",
			Quality:     85,
			SpeedRating: 3,
			SizeRating:  8,
		},
	}
}

// StartTranscode starts a transcoding session using NVENC
func (p *NvidiaTranscoder) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	if p.transcoder == nil {
		return nil, fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to transcoding types with NVIDIA-specific settings
	transcodingReq := transcoding.TranscodeRequest{
		MediaID:        req.MediaID,
		SessionID:      req.SessionID,
		InputPath:      req.InputPath,
		OutputPath:     req.OutputPath,
		Container:      req.Container,
		VideoCodec:     p.getNVENCCodec(req.VideoCodec), // Map to NVENC codec
		AudioCodec:     req.AudioCodec,
		Resolution:     req.Resolution,
		Quality:        req.Quality,
		SpeedPriority:  transcoding.SpeedPriority(req.SpeedPriority),
		Seek:           req.Seek,
		Duration:       req.Duration,
		EnableABR:      req.EnableABR,
		PreferHardware: true, // Always prefer hardware for NVIDIA plugin
		HardwareType:   transcoding.HardwareTypeNVIDIA,
		VideoBitrate:   req.VideoBitrate,
		AudioBitrate:   req.AudioBitrate,
	}
	
	// Add NVIDIA-specific encoding parameters
	hardwareParams := map[string]interface{}{
		"preset": p.getNVENCPreset(int(req.SpeedPriority)),
		"profile": "high",
		"rc": "vbr", // Variable bitrate for quality
		"gpu": "0",  // Use first GPU (could be made configurable)
	}
	// Marshal as JSON for ProviderSettings
	if paramsJSON, err := json.Marshal(hardwareParams); err == nil {
		transcodingReq.ProviderSettings = paramsJSON
	}
	
	handle, err := p.transcoder.StartTranscode(ctx, transcodingReq)
	if err != nil {
		return nil, fmt.Errorf("failed to start NVENC transcode: %w", err)
	}
	
	// Convert to plugin handle
	return &plugins.TranscodeHandle{
		SessionID:   handle.SessionID,
		Provider:    "ffmpeg_nvidia",
		StartTime:   handle.StartTime,
		Directory:   handle.Directory,
		Context:     handle.Context,
		CancelFunc:  handle.CancelFunc,
		PrivateData: handle.PrivateData,
		Status:      plugins.TranscodeStatus(handle.Status),
		Error:       handle.Error,
	}, nil
}

func (p *NvidiaTranscoder) GetProgress(handle *plugins.TranscodeHandle) (*plugins.TranscodingProgress, error) {
	if p.transcoder == nil {
		return nil, fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to transcoding handle
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

func (p *NvidiaTranscoder) StopTranscode(handle *plugins.TranscodeHandle) error {
	if p.transcoder == nil {
		return fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to transcoding handle
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

func (p *NvidiaTranscoder) StartStream(ctx context.Context, req plugins.TranscodeRequest) (*plugins.StreamHandle, error) {
	if p.transcoder == nil {
		return nil, fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to transcoding types with NVIDIA-specific settings
	transcodingReq := transcoding.TranscodeRequest{
		MediaID:        req.MediaID,
		SessionID:      req.SessionID,
		InputPath:      req.InputPath,
		Container:      req.Container,
		VideoCodec:     p.getNVENCCodec(req.VideoCodec),
		AudioCodec:     req.AudioCodec,
		Resolution:     req.Resolution,
		Quality:        req.Quality,
		SpeedPriority:  transcoding.SpeedPriority(req.SpeedPriority),
		Seek:           req.Seek,
		Duration:       req.Duration,
		EnableABR:      req.EnableABR,
		PreferHardware: true,
		HardwareType:   transcoding.HardwareTypeNVIDIA,
		VideoBitrate:   req.VideoBitrate,
		AudioBitrate:   req.AudioBitrate,
	}
	
	// Add NVIDIA-specific streaming parameters
	streamParams := map[string]interface{}{
		"preset": "llhq",     // Low-latency high quality
		"rc": "cbr",          // Constant bitrate for streaming
		"zerolatency": "1",   // Enable zero latency mode
		"gpu": "0",
	}
	// Marshal as JSON for ProviderSettings
	if paramsJSON, err := json.Marshal(streamParams); err == nil {
		transcodingReq.ProviderSettings = paramsJSON
	}
	
	handle, err := p.transcoder.StartStream(ctx, transcodingReq)
	if err != nil {
		return nil, fmt.Errorf("failed to start NVENC stream: %w", err)
	}
	
	// Convert to plugin handle
	return &plugins.StreamHandle{
		SessionID:   handle.SessionID,
		Provider:    "ffmpeg_nvidia",
		StartTime:   time.Now(),
		PrivateData: handle.PrivateData,
	}, nil
}

func (p *NvidiaTranscoder) GetStream(handle *plugins.StreamHandle) (io.ReadCloser, error) {
	if p.transcoder == nil {
		return nil, fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to transcoding handle
	sdkHandle := &transcoding.StreamHandle{
		SessionID:   handle.SessionID,
		PrivateData: handle.PrivateData,
	}
	
	return p.transcoder.GetStream(sdkHandle)
}

func (p *NvidiaTranscoder) StopStream(handle *plugins.StreamHandle) error {
	if p.transcoder == nil {
		return fmt.Errorf("transcoder not initialized")
	}
	
	// Convert to transcoding handle
	sdkHandle := &transcoding.StreamHandle{
		SessionID:   handle.SessionID,
		PrivateData: handle.PrivateData,
	}
	
	return p.transcoder.StopStream(sdkHandle)
}

// getNVENCCodec maps generic codec names to NVENC-specific encoder names
func (p *NvidiaTranscoder) getNVENCCodec(codec string) string {
	switch codec {
	case "h264", "libx264", "":
		return "h264_nvenc"
	case "h265", "hevc", "libx265":
		return "hevc_nvenc"
	case "av1", "libaom-av1":
		// AV1 NVENC availability check moved to module
		// For now, return av1_nvenc and let module handle fallback
		return "av1_nvenc"
		// Fall back to H.265 if AV1 not available
		return "hevc_nvenc"
	default:
		// Default to H.264 for unsupported codecs
		return "h264_nvenc"
	}
}

// getNVENCPreset maps speed priority to NVENC preset
func (p *NvidiaTranscoder) getNVENCPreset(speedPriority int) string {
	// NVENC presets: p1 (fastest) to p7 (highest quality)
	switch speedPriority {
	case 1: // Fastest
		return "p1"
	case 2:
		return "p2"
	case 3:
		return "p3"
	case 4:
		return "p4"
	case 5:
		return "p5"
	case 6:
		return "p6"
	default: // Highest quality
		return "p7"
	}
}

// SupportsIntermediateOutput returns true as NVIDIA encoder outputs intermediate MP4 files
func (p *NvidiaTranscoder) SupportsIntermediateOutput() bool {
	return true
}

// GetIntermediateOutputPath returns the path to intermediate MP4 file
func (p *NvidiaTranscoder) GetIntermediateOutputPath(handle *plugins.TranscodeHandle) (string, error) {
	if handle == nil {
		return "", fmt.Errorf("handle is nil")
	}
	
	// Return the MP4 file path in the encoded subdirectory
	if handle.Directory != "" {
		return filepath.Join(handle.Directory, "encoded", "output.mp4"), nil
	}
	
	return "", fmt.Errorf("no directory information in handle")
}

// GetABRVariants returns the ABR encoding variants optimized for NVENC
func (p *NvidiaTranscoder) GetABRVariants(req plugins.TranscodeRequest) ([]plugins.ABRVariant, error) {
	var variants []plugins.ABRVariant
	
	// Determine maximum resolution from request
	maxHeight := 1080 // Default
	if req.Resolution != nil {
		maxHeight = req.Resolution.Height
	}
	
	// NVENC-optimized ABR ladder with hardware-specific settings
	if maxHeight >= 2160 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "4K",
			Resolution:   &plugins.Resolution{Width: 3840, Height: 2160},
			VideoBitrate: 18000, // Higher bitrate for NVENC
			AudioBitrate: 192,
			FrameRate:    30,
			Preset:       "p4", // Balanced preset for 4K
			Profile:      "high",
			Level:        "5.1",
		})
	}
	
	if maxHeight >= 1440 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "1440p",
			Resolution:   &plugins.Resolution{Width: 2560, Height: 1440},
			VideoBitrate: 10000,
			AudioBitrate: 192,
			FrameRate:    30,
			Preset:       "p4",
			Profile:      "high",
			Level:        "4.1",
		})
	}
	
	if maxHeight >= 1080 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "1080p",
			Resolution:   &plugins.Resolution{Width: 1920, Height: 1080},
			VideoBitrate: 6000,
			AudioBitrate: 192,
			FrameRate:    30,
			Preset:       "p5", // Higher quality for 1080p
			Profile:      "high",
			Level:        "4.0",
		})
	}
	
	if maxHeight >= 720 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "720p",
			Resolution:   &plugins.Resolution{Width: 1280, Height: 720},
			VideoBitrate: 3000,
			AudioBitrate: 128,
			FrameRate:    30,
			Preset:       "p5",
			Profile:      "main",
			Level:        "3.1",
		})
	}
	
	if maxHeight >= 480 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "480p",
			Resolution:   &plugins.Resolution{Width: 854, Height: 480},
			VideoBitrate: 1500,
			AudioBitrate: 128,
			FrameRate:    30,
			Preset:       "p6", // Higher quality for lower res
			Profile:      "main",
			Level:        "3.0",
		})
	}
	
	// Always include a low resolution variant
	variants = append(variants, plugins.ABRVariant{
		Name:         "360p",
		Resolution:   &plugins.Resolution{Width: 640, Height: 360},
		VideoBitrate: 800,
		AudioBitrate: 96,
		FrameRate:    30,
		Preset:       "p6",
		Profile:      "baseline",
		Level:        "3.0",
	})
	
	return variants, nil
}

func (p *NvidiaTranscoder) GetDashboardSections() []plugins.DashboardSection {
	return []plugins.DashboardSection{
		{
			ID:    "overview",
			Title: "NVIDIA Transcoder Overview",
			Type:  "stats",
		},
		{
			ID:    "gpu_usage",
			Title: "GPU Utilization",
			Type:  "chart",
		},
	}
}

func (p *NvidiaTranscoder) GetDashboardData(sectionID string) (interface{}, error) {
	switch sectionID {
	case "overview":
		return map[string]interface{}{
			"active_sessions": 0,
			"completed_jobs":  0,
			"gpu_usage":       0.0,
			"encoder_usage":   0.0,
		}, nil
	case "gpu_usage":
		return map[string]interface{}{
			"current": 0.0,
			"average": 0.0,
			"peak":    0.0,
		}, nil
	default:
		return nil, nil
	}
}

func (p *NvidiaTranscoder) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	return nil
}

// Return nil for unsupported services
func (p *NvidiaTranscoder) MetadataScraperService() plugins.MetadataScraperService         { return nil }
func (p *NvidiaTranscoder) ScannerHookService() plugins.ScannerHookService                 { return nil }
func (p *NvidiaTranscoder) AssetService() plugins.AssetService                             { return nil }
func (p *NvidiaTranscoder) DatabaseService() plugins.DatabaseService                       { return nil }
func (p *NvidiaTranscoder) AdminPageService() plugins.AdminPageService                     { return nil }
func (p *NvidiaTranscoder) APIRegistrationService() plugins.APIRegistrationService         { return nil }
func (p *NvidiaTranscoder) SearchService() plugins.SearchService                           { return nil }
func (p *NvidiaTranscoder) HealthMonitorService() plugins.HealthMonitorService             { return nil }
func (p *NvidiaTranscoder) ConfigurationService() plugins.ConfigurationService             { return nil }
func (p *NvidiaTranscoder) PerformanceMonitorService() plugins.PerformanceMonitorService   { return nil }
func (p *NvidiaTranscoder) EnhancedAdminPageService() plugins.EnhancedAdminPageService     { return nil }

// Plugin factory function
func NewNvidiaTranscoder() plugins.Implementation {
	return &NvidiaTranscoder{
		name:        "ffmpeg_nvidia",
		description: "FFmpeg NVIDIA Transcoder",
		version:     "1.0.0",
		author:      "Viewra Team",
		priority:    90,
	}
}

// Main function for plugin binary
func main() {
	plugin := NewNvidiaTranscoder()
	plugins.StartPlugin(plugin)
}

// loggerAdapter adapts plugins.Logger to types.Logger
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