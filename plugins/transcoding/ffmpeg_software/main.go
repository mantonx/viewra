package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	plugins "github.com/mantonx/viewra/sdk"
	"github.com/mantonx/viewra/sdk/transcoding"
)

// SoftwareTranscoder provides CPU-only transcoding using FFmpeg
type SoftwareTranscoder struct {
	name        string
	description string
	version     string
	author      string
	priority    int
	
	// Active transcoding sessions
	sessions map[string]*TranscodingSession
	mu       sync.RWMutex
}

// TranscodingSession represents an active transcoding operation
type TranscodingSession struct {
	SessionID      string
	MediaID        string
	InputPath      string
	OutputPath     string
	Container      string
	StartTime      time.Time
	Directory      string
	Context        context.Context
	CancelFunc     context.CancelFunc
	Process        *exec.Cmd
	ProgressData   *plugins.TranscodingProgress
	Status         transcoding.TranscodeStatus
	mu             sync.RWMutex
}

// Plugin implementation
func (p *SoftwareTranscoder) Initialize(ctx *plugins.PluginContext) error {
	// Initialize session tracking
	p.sessions = make(map[string]*TranscodingSession)
	
	// Check if FFmpeg is available
	if err := p.checkFFmpegAvailability(); err != nil {
		return fmt.Errorf("FFmpeg not available: %w", err)
	}
	
	ctx.Logger.Info("ffmpeg software transcoder plugin initialized")
	return nil
}

func (p *SoftwareTranscoder) Start() error {
	return nil
}

func (p *SoftwareTranscoder) Stop() error {
	// Stop all active sessions
	p.mu.Lock()
	defer p.mu.Unlock()
	
	for sessionID, session := range p.sessions {
		if session.CancelFunc != nil {
			session.CancelFunc()
		}
		if session.Process != nil && session.Process.Process != nil {
			_ = session.Process.Process.Kill()
		}
		delete(p.sessions, sessionID)
	}
	
	return nil
}

func (p *SoftwareTranscoder) Info() (*plugins.PluginInfo, error) {
	return &plugins.PluginInfo{
		ID:          "ffmpeg_software",
		Name:        "FFmpeg Software Transcoder",
		Version:     "1.0.0",
		Description: "High-quality CPU-based transcoding using FFmpeg",
		Author:      "Viewra Team",
		Type:        "transcoding",
	}, nil
}

func (p *SoftwareTranscoder) Health() error {
	return p.checkFFmpegAvailability()
}

// Service implementations
func (p *SoftwareTranscoder) TranscodingProvider() plugins.TranscodingProvider {
	return p
}

// TranscodingProvider interface implementation
func (p *SoftwareTranscoder) GetInfo() plugins.ProviderInfo {
	return plugins.ProviderInfo{
		ID:          "ffmpeg_software",
		Name:        "FFmpeg Software Transcoder",
		Description: "High-quality CPU-based transcoding with broad codec support",
		Version:     "1.0.0",
		Author:      "Viewra Team",
		Priority:    10, // Lower than hardware encoders
		Capabilities: []string{
			"h264_encoding",
			"h265_encoding",
			"vp9_encoding",
			"av1_encoding",
			"multi_pass",
			"high_quality",
		},
	}
}

func (p *SoftwareTranscoder) GetSupportedFormats() []plugins.ContainerFormat {
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

func (p *SoftwareTranscoder) GetHardwareAccelerators() []plugins.HardwareAccelerator {
	return []plugins.HardwareAccelerator{
		{
			Type:        "software",
			ID:          "cpu",
			Name:        "Software CPU Encoding",
			Available:   true,
			DeviceCount: 1,
		},
	}
}

func (p *SoftwareTranscoder) GetQualityPresets() []plugins.QualityPreset {
	return []plugins.QualityPreset{
		{
			ID:          "ultra_high",
			Name:        "Ultra High Quality",
			Description: "Maximum quality, very slow encoding",
			Quality:     95,
			SpeedRating: 1,
			SizeRating:  10,
		},
		{
			ID:          "high",
			Name:        "High Quality",
			Description: "High quality with reasonable encoding time",
			Quality:     80,
			SpeedRating: 3,
			SizeRating:  8,
		},
		{
			ID:          "balanced",
			Name:        "Balanced",
			Description: "Good balance of quality and speed",
			Quality:     65,
			SpeedRating: 5,
			SizeRating:  6,
		},
		{
			ID:          "fast",
			Name:        "Fast",
			Description: "Quick encoding with acceptable quality",
			Quality:     50,
			SpeedRating: 8,
			SizeRating:  4,
		},
	}
}

// Basic transcoding operations
func (p *SoftwareTranscoder) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	// Generate session ID if not provided
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("ffmpeg_sw_%d", time.Now().UnixNano())
	}
	
	// Create session directory
	sessionDir := filepath.Join("/tmp", "transcoding", sessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}
	
	// Create encoded subdirectory
	encodedDir := filepath.Join(sessionDir, "encoded")
	if err := os.MkdirAll(encodedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create encoded directory: %w", err)
	}
	
	// Determine output path
	outputPath := req.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(encodedDir, "output."+req.Container)
	}
	
	// Create session context
	sessionCtx, cancel := context.WithCancel(ctx)
	
	session := &TranscodingSession{
		SessionID:    sessionID,
		MediaID:      req.MediaID,
		InputPath:    req.InputPath,
		OutputPath:   outputPath,
		Container:    req.Container,
		StartTime:    time.Now(),
		Directory:    sessionDir,
		Context:      sessionCtx,
		CancelFunc:   cancel,
		Status:       transcoding.TranscodeStatusStarting,
		ProgressData: &plugins.TranscodingProgress{},
	}
	
	// Store session
	p.mu.Lock()
	p.sessions[sessionID] = session
	p.mu.Unlock()
	
	// Build FFmpeg command
	args, err := p.buildFFmpegArgs(req, session)
	if err != nil {
		return nil, fmt.Errorf("failed to build FFmpeg arguments: %w", err)
	}
	
	// Create and start FFmpeg process
	cmd := exec.CommandContext(sessionCtx, "ffmpeg", args...)
	
	// Set up progress monitoring
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start FFmpeg: %w", err)
	}
	
	session.Process = cmd
	session.Status = transcoding.TranscodeStatusRunning
	
	// Start progress monitoring goroutine
	go p.monitorProgress(session, stderr)
	
	// Start process monitoring goroutine
	go p.monitorProcess(session)
	
	return &plugins.TranscodeHandle{
		SessionID:   sessionID,
		Provider:    "ffmpeg_software",
		StartTime:   session.StartTime,
		Directory:   sessionDir,
		Context:     sessionCtx,
		CancelFunc:  cancel,
		PrivateData: session,
		Status:      plugins.TranscodeStatus(session.Status),
	}, nil
}

func (p *SoftwareTranscoder) GetProgress(handle *plugins.TranscodeHandle) (*plugins.TranscodingProgress, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	session, exists := p.sessions[handle.SessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", handle.SessionID)
	}
	
	session.mu.RLock()
	defer session.mu.RUnlock()
	
	return session.ProgressData, nil
}

func (p *SoftwareTranscoder) StopTranscode(handle *plugins.TranscodeHandle) error {
	p.mu.RLock()
	session, exists := p.sessions[handle.SessionID]
	p.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("session not found: %s", handle.SessionID)
	}
	
	// Cancel the session
	if session.CancelFunc != nil {
		session.CancelFunc()
	}
	
	// Kill the process if still running
	if session.Process != nil && session.Process.Process != nil {
		if err := session.Process.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill FFmpeg process: %w", err)
		}
	}
	
	// Update status
	session.mu.Lock()
	session.Status = transcoding.TranscodeStatusCancelled
	session.mu.Unlock()
	
	// Clean up session
	p.mu.Lock()
	delete(p.sessions, handle.SessionID)
	p.mu.Unlock()
	
	return nil
}

func (p *SoftwareTranscoder) StartStream(ctx context.Context, req plugins.TranscodeRequest) (*plugins.StreamHandle, error) {
	// For streaming, we just start a transcode and return a handle
	// The actual streaming would be handled by reading the output file
	handle, err := p.StartTranscode(ctx, req)
	if err != nil {
		return nil, err
	}
	
	return &plugins.StreamHandle{
		SessionID:   handle.SessionID,
		Provider:    "ffmpeg_software",
		StartTime:   time.Now(),
		PrivateData: handle,
	}, nil
}

func (p *SoftwareTranscoder) GetStream(handle *plugins.StreamHandle) (io.ReadCloser, error) {
	// For software transcoding, we don't support live streaming
	// This would need to be implemented with named pipes or similar
	return nil, fmt.Errorf("live streaming not supported by software transcoder")
}

func (p *SoftwareTranscoder) StopStream(handle *plugins.StreamHandle) error {
	if transHandle, ok := handle.PrivateData.(*plugins.TranscodeHandle); ok {
		return p.StopTranscode(transHandle)
	}
	return fmt.Errorf("invalid stream handle")
}

func (p *SoftwareTranscoder) GetDashboardSections() []plugins.DashboardSection {
	return []plugins.DashboardSection{
		{
			ID:    "overview",
			Title: "Software Transcoder Overview",
			Type:  "stats",
		},
	}
}

func (p *SoftwareTranscoder) GetDashboardData(sectionID string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	activeSessions := 0
	for _, session := range p.sessions {
		if session.Status == transcoding.TranscodeStatusRunning {
			activeSessions++
		}
	}
	
	return map[string]interface{}{
		"active_sessions": activeSessions,
		"total_sessions":  len(p.sessions),
		"cpu_usage":       0.0, // TODO: Implement CPU monitoring
	}, nil
}

func (p *SoftwareTranscoder) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	return nil
}

// Helper methods

func (p *SoftwareTranscoder) checkFFmpegAvailability() error {
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FFmpeg not found or not executable: %w", err)
	}
	return nil
}

func (p *SoftwareTranscoder) buildFFmpegArgs(req plugins.TranscodeRequest, session *TranscodingSession) ([]string, error) {
	var args []string
	
	// Overwrite output files
	args = append(args, "-y")
	
	// Progress reporting
	args = append(args, "-progress", "pipe:2")
	
	// Input file
	args = append(args, "-i", req.InputPath)
	
	// Seek if specified
	if req.Seek > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", req.Seek.Seconds()))
	}
	
	// Duration if specified
	if req.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%.2f", req.Duration.Seconds()))
	}
	
	// Video codec
	videoCodec := req.VideoCodec
	if videoCodec == "" {
		videoCodec = "libx264" // Default
	}
	args = append(args, "-c:v", videoCodec)
	
	// Audio codec
	audioCodec := req.AudioCodec
	if audioCodec == "" {
		audioCodec = "aac" // Default
	}
	args = append(args, "-c:a", audioCodec)
	
	// Resolution
	if req.Resolution != nil {
		args = append(args, "-s", fmt.Sprintf("%dx%d", req.Resolution.Width, req.Resolution.Height))
	}
	
	// Bitrate
	if req.VideoBitrate > 0 {
		args = append(args, "-b:v", fmt.Sprintf("%dk", req.VideoBitrate))
	}
	if req.AudioBitrate > 0 {
		args = append(args, "-b:a", fmt.Sprintf("%dk", req.AudioBitrate))
	}
	
	// Quality preset
	preset := p.getFFmpegPreset(int(req.SpeedPriority))
	args = append(args, "-preset", preset)
	
	// Container-specific settings
	switch req.Container {
	case "mp4":
		args = append(args, "-f", "mp4")
		args = append(args, "-movflags", "faststart")
	case "webm":
		args = append(args, "-f", "webm")
	case "mkv":
		args = append(args, "-f", "matroska")
	case "dash", "hls":
		// For adaptive streaming, we'll just create MP4 for now
		// The actual DASH/HLS packaging should be handled by the pipeline provider
		args = append(args, "-f", "mp4")
		args = append(args, "-movflags", "faststart")
	}
	
	// Output file
	args = append(args, session.OutputPath)
	
	return args, nil
}

func (p *SoftwareTranscoder) getFFmpegPreset(speedPriority int) string {
	switch speedPriority {
	case 1: // Fastest
		return "ultrafast"
	case 2:
		return "superfast"
	case 3:
		return "veryfast"
	case 4:
		return "faster"
	case 5:
		return "fast"
	case 6:
		return "medium"
	case 7:
		return "slow"
	case 8:
		return "slower"
	default:
		return "veryslow"
	}
}

func (p *SoftwareTranscoder) monitorProgress(session *TranscodingSession, stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	
	// Regex patterns for parsing FFmpeg progress
	progressRegex := regexp.MustCompile(`frame=\s*(\d+)`)
	timeRegex := regexp.MustCompile(`time=(\d{2}):(\d{2}):(\d{2}\.\d{2})`)
	speedRegex := regexp.MustCompile(`speed=\s*([\d\.]+)x`)
	
	for scanner.Scan() {
		line := scanner.Text()
		
		session.mu.Lock()
		
		// Parse frame count
		if match := progressRegex.FindStringSubmatch(line); len(match) > 1 {
			if frame, err := strconv.ParseInt(match[1], 10, 64); err == nil {
				session.ProgressData.CurrentFrame = frame
			}
		}
		
		// Parse time elapsed
		if match := timeRegex.FindStringSubmatch(line); len(match) > 3 {
			hours, _ := strconv.Atoi(match[1])
			minutes, _ := strconv.Atoi(match[2])
			seconds, _ := strconv.ParseFloat(match[3], 64)
			elapsed := time.Duration(hours)*time.Hour + 
					  time.Duration(minutes)*time.Minute + 
					  time.Duration(seconds*float64(time.Second))
			session.ProgressData.TimeElapsed = elapsed
		}
		
		// Parse speed
		if match := speedRegex.FindStringSubmatch(line); len(match) > 1 {
			if speed, err := strconv.ParseFloat(match[1], 64); err == nil {
				session.ProgressData.CurrentSpeed = speed
			}
		}
		
		// Estimate progress percentage (rough estimate)
		if session.ProgressData.TimeElapsed > 0 {
			// This is a rough estimate - would need duration from input file for accuracy
			session.ProgressData.PercentComplete = float64(session.ProgressData.TimeElapsed) / float64(time.Hour) * 100
			if session.ProgressData.PercentComplete > 100 {
				session.ProgressData.PercentComplete = 100
			}
		}
		
		session.mu.Unlock()
	}
}

func (p *SoftwareTranscoder) monitorProcess(session *TranscodingSession) {
	err := session.Process.Wait()
	
	session.mu.Lock()
	defer session.mu.Unlock()
	
	if err != nil {
		if session.Context.Err() == context.Canceled {
			session.Status = transcoding.TranscodeStatusCancelled
		} else {
			session.Status = transcoding.TranscodeStatusFailed
		}
	} else {
		session.Status = transcoding.TranscodeStatusCompleted
		session.ProgressData.PercentComplete = 100.0
	}
}

// SupportsIntermediateOutput returns true as FFmpeg outputs intermediate MP4 files
func (p *SoftwareTranscoder) SupportsIntermediateOutput() bool {
	return true
}

// GetIntermediateOutputPath returns the path to the intermediate MP4 file
func (p *SoftwareTranscoder) GetIntermediateOutputPath(handle *plugins.TranscodeHandle) (string, error) {
	if handle == nil {
		return "", fmt.Errorf("handle is nil")
	}
	
	// Return the MP4 file path in the encoded subdirectory
	if handle.Directory != "" {
		return filepath.Join(handle.Directory, "encoded", "output.mp4"), nil
	}
	
	return "", fmt.Errorf("no directory information in handle")
}

// GetABRVariants returns the ABR encoding variants for software encoding
func (p *SoftwareTranscoder) GetABRVariants(req plugins.TranscodeRequest) ([]plugins.ABRVariant, error) {
	// Define standard ABR variants optimized for software encoding
	var variants []plugins.ABRVariant
	
	// Determine maximum resolution from request
	maxHeight := 1080 // Default
	if req.Resolution != nil {
		maxHeight = req.Resolution.Height
	}
	
	// Standard ABR ladder for software encoding
	if maxHeight >= 2160 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "4K",
			Resolution:   &plugins.Resolution{Width: 3840, Height: 2160},
			VideoBitrate: 15000,
			AudioBitrate: 192,
			FrameRate:    30,
			Preset:       "medium",
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
			Preset:       "medium",
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
			Preset:       "medium",
			Profile:      "main",
			Level:        "3.1",
		})
	}
	
	if maxHeight >= 480 {
		variants = append(variants, plugins.ABRVariant{
			Name:         "480p",
			Resolution:   &plugins.Resolution{Width: 854, Height: 480},
			VideoBitrate: 1000,
			AudioBitrate: 128,
			FrameRate:    30,
			Preset:       "fast",
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
		Preset:       "fast",
		Profile:      "baseline",
		Level:        "3.0",
	})
	
	return variants, nil
}

// Return nil for unsupported services
func (p *SoftwareTranscoder) MetadataScraperService() plugins.MetadataScraperService         { return nil }
func (p *SoftwareTranscoder) ScannerHookService() plugins.ScannerHookService                 { return nil }
func (p *SoftwareTranscoder) AssetService() plugins.AssetService                             { return nil }
func (p *SoftwareTranscoder) DatabaseService() plugins.DatabaseService                       { return nil }
func (p *SoftwareTranscoder) AdminPageService() plugins.AdminPageService                     { return nil }
func (p *SoftwareTranscoder) APIRegistrationService() plugins.APIRegistrationService         { return nil }
func (p *SoftwareTranscoder) SearchService() plugins.SearchService                           { return nil }
func (p *SoftwareTranscoder) HealthMonitorService() plugins.HealthMonitorService             { return nil }
func (p *SoftwareTranscoder) ConfigurationService() plugins.ConfigurationService             { return nil }
func (p *SoftwareTranscoder) PerformanceMonitorService() plugins.PerformanceMonitorService   { return nil }
func (p *SoftwareTranscoder) EnhancedAdminPageService() plugins.EnhancedAdminPageService     { return nil }

// Plugin factory function
func NewSoftwareTranscoder() plugins.Implementation {
	return &SoftwareTranscoder{
		name:        "ffmpeg_software",
		description: "FFmpeg Software Transcoder",
		version:     "1.0.0",
		author:      "Viewra Team",
		priority:    10,
	}
}

// Main function for plugin binary
func main() {
	plugin := NewSoftwareTranscoder()
	plugins.StartPlugin(plugin)
}