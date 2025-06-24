package pluginmodule

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	plugins "github.com/mantonx/viewra/sdk"
	"github.com/mantonx/viewra/sdk/proto"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// Local type definitions to avoid importing pkg/plugins directly
// These must match the interfaces in pkg/plugins exactly

// HandshakeConfig for external plugin communication (must match pkg/plugins/grpc_impl.go)
var ExternalPluginHandshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "VIEWRA_PLUGIN",
	MagicCookieValue: "viewra_plugin_magic_cookie_v1",
}

// ExternalPluginInterface represents the client-side interface to external plugins
type ExternalPluginInterface interface {
	// Core plugin methods
	Initialize(ctx *ExternalPluginContext) error
	Start() error
	Stop() error
	Info() (*ExternalPluginInfo, error)
	Health() error

	// Database service for creating plugin tables
	GetModels() []string
	Migrate(connectionString string) error

	// Scanner hook service for enrichment during scanning
	OnMediaFileScanned(mediaFileID string, filePath string, metadata map[string]string) error
	OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error
	OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error
}

// ExternalPluginContext provides context for plugin operations
type ExternalPluginContext struct {
	PluginID        string `json:"plugin_id"`
	DatabaseURL     string `json:"database_url"`
	HostServiceAddr string `json:"host_service_addr"`
	LogLevel        string `json:"log_level"`
	BasePath        string `json:"base_path"`
}

// ExternalPluginInfo represents plugin information
type ExternalPluginInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Author      string `json:"author"`
}

// GRPCExternalPlugin implements the hashicorp/go-plugin interface for external plugins
type GRPCExternalPlugin struct {
	goplugin.Plugin
}

// GRPCClient creates the gRPC client for external plugins
func (p *GRPCExternalPlugin) GRPCClient(ctx context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	// Return a simplified client that implements our ExternalPluginInterface
	return &ExternalPluginGRPCClient{
		broker: broker,
		conn:   c,
	}, nil
}

// GRPCServer is not used on the host side
func (p *GRPCExternalPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	return fmt.Errorf("GRPCServer not implemented on host side")
}

// ExternalPluginGRPCClient implements the client side of external plugin communication
type ExternalPluginGRPCClient struct {
	broker *goplugin.GRPCBroker
	conn   *grpc.ClientConn
}

// ExternalPluginAdapter adapts the GRPC client to implement plugins.Implementation
type ExternalPluginAdapter struct {
	client     *ExternalPluginGRPCClient
	pluginInfo *ExternalPluginInfo
}

// Compile-time interface checks
var _ plugins.Implementation = (*ExternalPluginAdapter)(nil)
var _ plugins.DashboardSectionProvider = (*ExternalPluginAdapter)(nil)
var _ plugins.DashboardDataProvider = (*ExternalPluginAdapter)(nil)

// Implement plugins.Implementation interface (NOT ExternalPluginInterface)
func (a *ExternalPluginAdapter) Initialize(ctx *plugins.PluginContext) error {
	// Convert plugins.PluginContext to ExternalPluginContext
	externalCtx := &ExternalPluginContext{
		PluginID:        ctx.PluginID,
		DatabaseURL:     ctx.DatabaseURL,
		HostServiceAddr: ctx.HostServiceAddr,
		LogLevel:        ctx.LogLevel,
		BasePath:        ctx.PluginBasePath,
	}
	return a.client.Initialize(externalCtx)
}

func (a *ExternalPluginAdapter) Start() error {
	return a.client.Start()
}

func (a *ExternalPluginAdapter) Stop() error {
	return a.client.Stop()
}

func (a *ExternalPluginAdapter) Info() (*plugins.PluginInfo, error) {
	externalInfo, err := a.client.Info()
	if err != nil {
		return nil, err
	}
	// Convert ExternalPluginInfo to plugins.PluginInfo
	return &plugins.PluginInfo{
		ID:          externalInfo.ID,
		Name:        externalInfo.Name,
		Version:     externalInfo.Version,
		Type:        externalInfo.Type,
		Description: externalInfo.Description,
		Author:      externalInfo.Author,
	}, nil
}

func (a *ExternalPluginAdapter) Health() error {
	return a.client.Health()
}

// Service methods - return nil for unsupported services
func (a *ExternalPluginAdapter) MetadataScraperService() plugins.MetadataScraperService { return nil }
func (a *ExternalPluginAdapter) ScannerHookService() plugins.ScannerHookService         { return nil }
func (a *ExternalPluginAdapter) AssetService() plugins.AssetService                     { return nil }
func (a *ExternalPluginAdapter) DatabaseService() plugins.DatabaseService               { return nil }
func (a *ExternalPluginAdapter) AdminPageService() plugins.AdminPageService             { return nil }
func (a *ExternalPluginAdapter) APIRegistrationService() plugins.APIRegistrationService { return nil }
func (a *ExternalPluginAdapter) SearchService() plugins.SearchService                   { return nil }
func (a *ExternalPluginAdapter) HealthMonitorService() plugins.HealthMonitorService     { return nil }
func (a *ExternalPluginAdapter) ConfigurationService() plugins.ConfigurationService     { return nil }
func (a *ExternalPluginAdapter) PerformanceMonitorService() plugins.PerformanceMonitorService {
	return nil
}
func (a *ExternalPluginAdapter) EnhancedAdminPageService() plugins.EnhancedAdminPageService {
	return nil
}

// TranscodingProvider - return a provider implementation for transcoder plugins
func (a *ExternalPluginAdapter) TranscodingProvider() plugins.TranscodingProvider {
	if a.pluginInfo != nil && a.pluginInfo.Type == "transcoder" {
		// Create a basic transcoding provider that uses the GRPC client
		return &ExternalTranscodingProvider{
			pluginID:   a.pluginInfo.ID,
			pluginInfo: a.pluginInfo,
			client:     a.client,
		}
	}
	return nil
}

// Dashboard interface implementations
func (a *ExternalPluginAdapter) GetDashboardSections(ctx context.Context) ([]plugins.DashboardSection, error) {
	// For now, create a simple mock dashboard section for the FFmpeg transcoder
	// In the future, this would call the GRPC DashboardService
	if a.pluginInfo != nil && a.pluginInfo.Type == "transcoder" {
		return []plugins.DashboardSection{
			{
				ID:          a.pluginInfo.ID + "_main",
				PluginID:    a.pluginInfo.ID,
				Type:        "transcoder",
				Title:       a.pluginInfo.Name,
				Description: a.pluginInfo.Description,
				Icon:        "video",
				Priority:    100,
				Config: plugins.DashboardSectionConfig{
					RefreshInterval:  5,
					SupportsRealtime: false,
					HasNerdPanel:     true,
					RequiresAuth:     false,
					MinRefreshRate:   1,
					MaxDataPoints:    100,
				},
				Manifest: plugins.DashboardManifest{
					ComponentType: "builtin",
					DataEndpoints: map[string]plugins.DataEndpoint{
						"main": {
							Path:        "/api/v1/dashboard/sections/" + a.pluginInfo.ID + "_main/data/main",
							Method:      "GET",
							DataType:    "main",
							Description: "Main transcoding dashboard data",
						},
						"nerd": {
							Path:        "/api/v1/dashboard/sections/" + a.pluginInfo.ID + "_main/data/nerd",
							Method:      "GET",
							DataType:    "nerd",
							Description: "Advanced transcoding metrics",
						},
					},
					Actions: []plugins.DashboardAction{
						{
							ID:       "clear_cache",
							Label:    "Clear Cache",
							Icon:     "trash",
							Style:    "secondary",
							Endpoint: "/api/v1/dashboard/sections/" + a.pluginInfo.ID + "_main/actions/clear_cache",
							Method:   "POST",
							Confirm:  true,
						},
						{
							ID:       "restart_service",
							Label:    "Restart",
							Icon:     "refresh",
							Style:    "warning",
							Endpoint: "/api/v1/dashboard/sections/" + a.pluginInfo.ID + "_main/actions/restart_service",
							Method:   "POST",
							Confirm:  true,
						},
					},
					UISchema: map[string]interface{}{
						"layout": "transcoder",
					},
				},
			},
		}, nil
	}

	return []plugins.DashboardSection{}, nil
}

func (a *ExternalPluginAdapter) GetMainData(ctx context.Context, sectionID string) (interface{}, error) {
	// Get real transcoding data via GRPC
	if a.pluginInfo != nil && a.pluginInfo.Type == "transcoder" {
		// TODO: Update to use TranscodingProvider once gRPC support is added
		// For now, return mock data
		return a.getMockTranscoderMainData(), nil
	}

	return nil, fmt.Errorf("unsupported section: %s", sectionID)
}

// Helper method to return mock data as fallback
func (a *ExternalPluginAdapter) getMockTranscoderMainData() plugins.TranscoderMainData {
	return plugins.TranscoderMainData{
		ActiveSessions: []plugins.TranscodeSessionSummary{},
		QueuedSessions: []plugins.TranscodeSessionSummary{},
		RecentSessions: []plugins.TranscodeSessionSummary{},
		EngineStatus: plugins.TranscoderEngineStatus{
			Type:            "ffmpeg",
			Status:          "healthy",
			Version:         "6.0.0",
			MaxConcurrent:   10,
			ActiveSessions:  0,
			QueuedSessions:  0,
			LastHealthCheck: time.Now(),
			Capabilities:    []string{"h264", "h265", "vp8", "vp9", "av1", "aac", "mp3"},
		},
		QuickStats: plugins.TranscoderQuickStats{
			SessionsToday:     3,
			TotalHoursToday:   1.5,
			AverageSpeed:      1.2,
			ErrorRate:         0.02,
			CurrentThroughput: "0 fps",
			PeakConcurrent:    2,
		},
	}
}

// convertToSessionSummary converts a TranscodeSession to TranscodeSessionSummary for dashboard display
// DEPRECATED: This uses the old TranscodeSession type which has been removed
/*
func (a *ExternalPluginAdapter) convertToSessionSummary(session *plugins.TranscodeSession) plugins.TranscodeSessionSummary {
	// Implementation removed - using TranscodingProvider now
	return plugins.TranscodeSessionSummary{}
}
*/

// extractContentTitle extracts a user-friendly title from a file path using database lookup first, then smart filename parsing
func (a *ExternalPluginAdapter) extractContentTitle(inputPath string) string {
	if inputPath == "" {
		return "Unknown Content"
	}

	// TODO: Add database connection to ExternalPluginAdapter to query media_files table
	// For now, use smart filename parsing as the primary method

	// Extract and intelligently clean up filename
	basename := filepath.Base(inputPath)
	nameWithoutExt := strings.TrimSuffix(basename, filepath.Ext(basename))

	// Smart episode title extraction from common TV show filename formats
	// Format: "Show Name (Year) - S01E01 - Episode Title [Quality info]"
	if strings.Contains(nameWithoutExt, " - S") && strings.Contains(nameWithoutExt, "E") {
		parts := strings.Split(nameWithoutExt, " - ")
		if len(parts) >= 3 {
			// Find the show name, season/episode, and episode title
			var showName, seasonEpisode, episodeTitle string

			for i, part := range parts {
				if strings.Contains(part, "S") && strings.Contains(part, "E") {
					// This is the season/episode part (e.g., "S01E01")
					seasonEpisode = part

					// Everything before this is the show name
					if i > 0 {
						showParts := parts[:i]
						showName = strings.Join(showParts, " - ")
						// Remove year from show name if present
						if strings.Contains(showName, "(") && strings.Contains(showName, ")") {
							if parenIndex := strings.Index(showName, " ("); parenIndex != -1 {
								showName = showName[:parenIndex]
							}
						}
					}

					// Everything after this is the episode title
					if i+1 < len(parts) {
						episodeParts := parts[i+1:]
						episodeTitle = strings.Join(episodeParts, " - ")
						// Remove quality tags in brackets
						if bracketIndex := strings.Index(episodeTitle, " ["); bracketIndex != -1 {
							episodeTitle = episodeTitle[:bracketIndex]
						}
						// Remove parenthetical info like (1080p), (BluRay), etc.
						if parenIndex := strings.Index(episodeTitle, " ("); parenIndex != -1 {
							episodeTitle = episodeTitle[:parenIndex]
						}
					}
					break
				}
			}

			// Format as "Show Title - S##E## - Episode Title" (with hyphens for readability)
			if showName != "" && seasonEpisode != "" && episodeTitle != "" {
				return fmt.Sprintf("%s - %s - %s",
					strings.TrimSpace(showName),
					strings.TrimSpace(seasonEpisode),
					strings.TrimSpace(episodeTitle))
			} else if episodeTitle != "" {
				// Fallback: just return episode title if we can't parse the full structure
				return strings.TrimSpace(episodeTitle)
			}
		}
	}

	// Check if this might be a movie (no season/episode indicators)
	// Movie format: "Movie Title (Year) [Quality info]"
	cleanName := nameWithoutExt

	// Remove all quality indicators and technical info
	qualityIndicators := []string{
		" [Bluray-1080p]", " [WEBRip-1080p]", " [HDTV-720p]", " [DVDRip-480p]", " [4K-UHD]",
		" [FLAC 2.0]", " [AAC 2.0]", " [AC3 5.1]", " [DTS 5.1]", " [Atmos]",
		" [x264]", " [x265]", " [HEVC]", " [AVC]", " [H.264]", " [H.265]",
		" (1080p)", " (720p)", " (480p)", " (4K)", " (2160p)",
		" (BluRay)", " (WEBRip)", " (HDTV)", " (DVDRip)", " (BDRip)",
		".1080p.", ".720p.", ".480p.", ".4K.", ".2160p.",
		".BluRay.", ".WEBRip.", ".HDTV.", ".DVDRip.", ".BDRip.",
		".x264.", ".x265.", ".HEVC.", ".AVC.", ".H264.", ".H265.",
	}

	for _, indicator := range qualityIndicators {
		cleanName = strings.ReplaceAll(cleanName, indicator, "")
	}

	// Remove anything in square brackets (quality info)
	for strings.Contains(cleanName, "[") && strings.Contains(cleanName, "]") {
		startBracket := strings.Index(cleanName, "[")
		endBracket := strings.Index(cleanName[startBracket:], "]")
		if endBracket != -1 {
			cleanName = cleanName[:startBracket] + cleanName[startBracket+endBracket+1:]
		} else {
			break
		}
	}

	// Remove group tags at the end (e.g., "-BTN", "-RARBG", "-FGT")
	if lastDash := strings.LastIndex(cleanName, "-"); lastDash != -1 {
		possibleGroup := strings.TrimSpace(cleanName[lastDash+1:])
		// If it's a short alphanumeric string without spaces, it's likely a group tag
		if len(possibleGroup) <= 8 && !strings.Contains(possibleGroup, " ") {
			cleanName = cleanName[:lastDash]
		}
	}

	// Clean up punctuation and spacing
	cleanName = strings.ReplaceAll(cleanName, "_", " ")
	cleanName = strings.ReplaceAll(cleanName, ".", " ")
	cleanName = strings.ReplaceAll(cleanName, "  ", " ")
	cleanName = strings.TrimSpace(cleanName)

	// For movies, often there's a year at the end in parentheses - keep that
	// Format: "Movie Title (2023)" - this is good for movies

	if cleanName == "" {
		return "Unknown Content"
	}

	return cleanName
}

// calculateQuickStats calculates quick statistics from active sessions
// DEPRECATED: This uses the old TranscodeSession type which has been removed
/*
func (a *ExternalPluginAdapter) calculateQuickStats(sessions []*plugins.TranscodeSession) plugins.TranscoderQuickStats {
	// Implementation removed - using TranscodingProvider now
	return plugins.TranscoderQuickStats{}
}
*/

func (a *ExternalPluginAdapter) GetNerdData(ctx context.Context, sectionID string) (interface{}, error) {
	// Mock nerd data for the transcoder
	if a.pluginInfo != nil && a.pluginInfo.Type == "transcoder" {
		return plugins.TranscoderNerdData{
			EncoderQueues: []plugins.EncoderQueueInfo{
				{
					QueueID:     "cpu_queue",
					Type:        "software",
					Pending:     0,
					Processing:  0,
					MaxSlots:    10,
					AvgWaitTime: "0s",
				},
			},
			HardwareStatus: plugins.HardwareStatusInfo{
				GPU: plugins.GPUInfo{
					Name:        "N/A (Software)",
					Driver:      "N/A",
					VRAMTotal:   0,
					VRAMUsed:    0,
					CoreClock:   0,
					MemoryClock: 0,
					FanSpeed:    0,
				},
				Encoders: []plugins.EncoderInfo{
					{
						ID:           "ffmpeg_cpu",
						Type:         "software",
						Status:       "idle",
						CurrentLoad:  0,
						SessionCount: 0,
						MaxSessions:  10,
					},
				},
				Memory: plugins.MemoryInfo{
					System: plugins.SystemMemory{
						Total:  16 * 1024 * 1024 * 1024, // 16GB
						Used:   8 * 1024 * 1024 * 1024,  // 8GB
						Cached: 2 * 1024 * 1024 * 1024,  // 2GB
					},
					GPU: plugins.GPUMemory{
						Total: 0,
						Used:  0,
						Free:  0,
					},
				},
				Temperature:    45.0,
				PowerDraw:      150.0,
				UtilizationPct: 15.0,
			},
			PerformanceMetrics: plugins.PerformanceMetrics{
				EncodingSpeed:    1.2,
				QualityScore:     0.92,
				CompressionRatio: 0.35,
				ErrorCount:       0,
				RestartCount:     0,
				UptimeSeconds:    3600,
			},
			ConfigDiagnostics: []plugins.ConfigDiagnostic{
				{
					Category:   "performance",
					Level:      "info",
					Message:    "Software encoding is active",
					Setting:    "hardware_acceleration",
					Value:      "false",
					Suggestion: "Consider enabling hardware acceleration for better performance",
				},
			},
			SystemResources: plugins.SystemResourceInfo{
				CPU: plugins.CPUInfo{
					Usage:     25.5,
					Cores:     8,
					Threads:   16,
					Frequency: 3400,
				},
				Memory: plugins.MemoryInfo{
					System: plugins.SystemMemory{
						Total:  16 * 1024 * 1024 * 1024,
						Used:   8 * 1024 * 1024 * 1024,
						Cached: 2 * 1024 * 1024 * 1024,
					},
				},
				Disk: plugins.DiskInfo{
					TotalSpace: 1000 * 1024 * 1024 * 1024, // 1TB
					UsedSpace:  400 * 1024 * 1024 * 1024,  // 400GB
					IOReads:    1000,
					IOWrites:   500,
					IOUtil:     25.5,
				},
				Network: plugins.NetworkInfo{
					BytesReceived: 100 * 1024 * 1024,
					BytesSent:     50 * 1024 * 1024,
					PacketsRx:     10000,
					PacketsTx:     5000,
					Bandwidth:     100.0,
				},
			},
		}, nil
	}

	return nil, fmt.Errorf("unsupported section: %s", sectionID)
}

func (a *ExternalPluginAdapter) GetMetrics(ctx context.Context, sectionID string, timeRange plugins.TimeRange) ([]plugins.MetricPoint, error) {
	// Mock metrics data
	if a.pluginInfo != nil && a.pluginInfo.Type == "transcoder" {
		metrics := make([]plugins.MetricPoint, 0)

		// Generate sample metrics for the last hour
		now := time.Now()
		for i := 0; i < 60; i++ {
			timestamp := now.Add(-time.Duration(59-i) * time.Minute)

			metrics = append(metrics, plugins.MetricPoint{
				Timestamp: timestamp,
				Value:     float64(i % 3), // 0-2 active sessions
				Labels: map[string]string{
					"metric": "active_sessions",
					"type":   "ffmpeg",
				},
			})
		}

		return metrics, nil
	}

	return []plugins.MetricPoint{}, nil
}

func (a *ExternalPluginAdapter) StreamData(ctx context.Context, sectionID string) (<-chan plugins.DashboardUpdate, error) {
	// For now, return an empty channel - real-time updates can be added later
	updateChan := make(chan plugins.DashboardUpdate)
	close(updateChan) // Close immediately as we don't have real-time data yet
	return updateChan, nil
}

// GetProviderInfo returns the provider information
// DEPRECATED: This uses the old TranscodingService interface which has been replaced by TranscodingProvider
// They used the old TranscodingService interface which has been replaced by TranscodingProvider

// GetQualityPresets returns available quality presets
// DEPRECATED: This uses the old TranscodingService interface which has been replaced by TranscodingProvider
// They used the old TranscodingService interface which has been replaced by TranscodingProvider

// MapQualityToProvider maps generic quality to provider-specific settings
// DEPRECATED: This uses the old TranscodingService interface which has been replaced by TranscodingProvider
// They used the old TranscodingService interface which has been replaced by TranscodingProvider

// GetHardwareAccelerators returns available hardware accelerators
// DEPRECATED: This uses the old TranscodingService interface which has been replaced by TranscodingProvider
// They used the old TranscodingService interface which has been replaced by TranscodingProvider

// GetSessionProgress returns detailed progress for a session
// DEPRECATED: This uses the old TranscodingService interface which has been replaced by TranscodingProvider
// They used the old TranscodingService interface which has been replaced by TranscodingProvider

// grpcStreamReader implements io.ReadCloser for GRPC streaming
type grpcStreamReader struct {
	stream   grpc.ServerStreamingClient[proto.StreamDataChunk]
	buffer   []byte
	position int
	closed   bool
}

func (r *grpcStreamReader) Read(p []byte) (n int, err error) {
	if r.closed {
		return 0, io.EOF
	}

	// If we have buffered data, read from it first
	if r.position < len(r.buffer) {
		n = copy(p, r.buffer[r.position:])
		r.position += n
		return n, nil
	}

	// Get next chunk from stream
	chunk, err := r.stream.Recv()
	if err != nil {
		r.closed = true
		return 0, err
	}

	if chunk.Error != "" {
		r.closed = true
		return 0, fmt.Errorf("stream error: %s", chunk.Error)
	}

	if chunk.Eof {
		r.closed = true
		return 0, io.EOF
	}

	// Copy data to output buffer
	n = copy(p, chunk.Data)

	// If chunk is larger than output buffer, save remainder
	if len(chunk.Data) > len(p) {
		r.buffer = chunk.Data[n:]
		r.position = 0
	} else {
		r.buffer = nil
		r.position = 0
	}

	return n, nil
}

func (r *grpcStreamReader) Close() error {
	r.closed = true
	return nil
}

// Database service methods - delegate to the client
func (a *ExternalPluginAdapter) GetModels() []string {
	return a.client.GetModels()
}

func (a *ExternalPluginAdapter) Migrate(connectionString string) error {
	return a.client.Migrate(connectionString)
}

// Scanner hook service methods - delegate to the client
func (a *ExternalPluginAdapter) OnMediaFileScanned(mediaFileID string, filePath string, metadata map[string]string) error {
	return a.client.OnMediaFileScanned(mediaFileID, filePath, metadata)
}

func (a *ExternalPluginAdapter) OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error {
	return a.client.OnScanStarted(scanJobID, libraryID, libraryPath)
}

func (a *ExternalPluginAdapter) OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error {
	return a.client.OnScanCompleted(scanJobID, libraryID, stats)
}

// Core plugin service implementations for ExternalPluginGRPCClient
func (c *ExternalPluginGRPCClient) Initialize(ctx *ExternalPluginContext) error {
	client := proto.NewPluginServiceClient(c.conn)

	// Convert ExternalPluginContext to proto.PluginContext
	protoCtx := &proto.PluginContext{
		PluginId:        ctx.PluginID,
		DatabaseUrl:     ctx.DatabaseURL,
		HostServiceAddr: ctx.HostServiceAddr,
		LogLevel:        ctx.LogLevel,
		BasePath:        ctx.BasePath,
		Config:          make(map[string]string), // Empty config for now
	}

	req := &proto.InitializeRequest{Context: protoCtx}
	resp, err := client.Initialize(context.Background(), req)
	if err != nil {
		return fmt.Errorf("plugin Initialize failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("plugin Initialize returned error: %s", resp.Error)
	}

	return nil
}

func (c *ExternalPluginGRPCClient) Start() error {
	client := proto.NewPluginServiceClient(c.conn)

	req := &proto.StartRequest{}
	resp, err := client.Start(context.Background(), req)
	if err != nil {
		return fmt.Errorf("plugin Start failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("plugin Start returned error: %s", resp.Error)
	}

	return nil
}

func (c *ExternalPluginGRPCClient) Stop() error {
	client := proto.NewPluginServiceClient(c.conn)

	req := &proto.StopRequest{}
	resp, err := client.Stop(context.Background(), req)
	if err != nil {
		return fmt.Errorf("plugin Stop failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("plugin Stop returned error: %s", resp.Error)
	}

	return nil
}

func (c *ExternalPluginGRPCClient) Info() (*ExternalPluginInfo, error) {
	client := proto.NewPluginServiceClient(c.conn)

	req := &proto.InfoRequest{}
	resp, err := client.Info(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("plugin Info failed: %w", err)
	}

	if resp.Info == nil {
		return nil, fmt.Errorf("plugin Info returned nil info")
	}

	return &ExternalPluginInfo{
		ID:          resp.Info.Id,
		Name:        resp.Info.Name,
		Version:     resp.Info.Version,
		Type:        resp.Info.Type,
		Description: resp.Info.Description,
		Author:      resp.Info.Author,
	}, nil
}

func (c *ExternalPluginGRPCClient) Health() error {
	client := proto.NewPluginServiceClient(c.conn)

	req := &proto.HealthRequest{}
	resp, err := client.Health(context.Background(), req)
	if err != nil {
		return fmt.Errorf("plugin Health failed: %w", err)
	}

	if !resp.Healthy {
		return fmt.Errorf("plugin Health check failed: %s", resp.Error)
	}

	return nil
}

// Database service implementations
func (c *ExternalPluginGRPCClient) GetModels() []string {
	client := proto.NewDatabaseServiceClient(c.conn)

	req := &proto.GetModelsRequest{}
	resp, err := client.GetModels(context.Background(), req)
	if err != nil {
		// Return empty array - plugin might not implement database service or might not be ready
		return []string{}
	}

	if resp == nil {
		return []string{}
	}

	return resp.ModelNames
}

func (c *ExternalPluginGRPCClient) Migrate(connectionString string) error {
	client := proto.NewDatabaseServiceClient(c.conn)

	req := &proto.MigrateRequest{ConnectionString: connectionString}
	resp, err := client.Migrate(context.Background(), req)
	if err != nil {
		return fmt.Errorf("plugin Migrate failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("plugin Migrate returned error: %s", resp.Error)
	}

	return nil
}

// Scanner hook service implementations
func (c *ExternalPluginGRPCClient) OnMediaFileScanned(mediaFileID string, filePath string, metadata map[string]string) error {
	client := proto.NewScannerHookServiceClient(c.conn)

	req := &proto.OnMediaFileScannedRequest{
		MediaFileId: mediaFileID,
		FilePath:    filePath,
		Metadata:    metadata,
	}

	_, err := client.OnMediaFileScanned(context.Background(), req)
	if err != nil {
		return fmt.Errorf("plugin OnMediaFileScanned failed: %w", err)
	}

	return nil
}

func (c *ExternalPluginGRPCClient) OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error {
	client := proto.NewScannerHookServiceClient(c.conn)

	req := &proto.OnScanStartedRequest{
		ScanJobId:   scanJobID,
		LibraryId:   libraryID,
		LibraryPath: libraryPath,
	}

	_, err := client.OnScanStarted(context.Background(), req)
	if err != nil {
		return fmt.Errorf("plugin OnScanStarted failed: %w", err)
	}

	return nil
}

func (c *ExternalPluginGRPCClient) OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error {
	// Create proto client
	client := proto.NewScannerHookServiceClient(c.conn)

	_, err := client.OnScanCompleted(context.Background(), &proto.OnScanCompletedRequest{
		ScanJobId: scanJobID,
		LibraryId: libraryID,
		Stats:     stats,
	})

	return err
}

// GetAdminPages gets admin pages from the plugin via GRPC
func (c *ExternalPluginGRPCClient) GetAdminPages() ([]*proto.AdminPageConfig, error) {
	// Create proto client
	client := proto.NewAdminPageServiceClient(c.conn)

	resp, err := client.GetAdminPages(context.Background(), &proto.GetAdminPagesRequest{})
	if err != nil {
		return nil, err
	}

	return resp.Pages, nil
}

// ExternalPluginManager manages external plugins
type ExternalPluginManager struct {
	db     *gorm.DB
	logger hclog.Logger
	mu     sync.RWMutex

	// External plugins
	plugins map[string]*ExternalPlugin

	// Host services for external plugin communication
	hostServices *HostServices

	// Context management
	ctx    context.Context
	cancel context.CancelFunc

	// Configuration
	pluginDir string

	// Plugin clients and interfaces
	pluginClients    map[string]*goplugin.Client
	pluginInterfaces map[string]ExternalPluginInterface

	// NEW: Reliability features
	healthMonitor     *PluginHealthMonitor
	fallbackManager   *FallbackManager
	reliabilityConfig *config.PluginReliabilityConfig

	// Dashboard integration
	dashboardManager *DashboardManager
}

// ExternalPluginManifest represents the parsed CUE configuration
type ExternalPluginManifest struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Version        string                 `json:"version"`
	Description    string                 `json:"description"`
	Author         string                 `json:"author"`
	Type           string                 `json:"type"`
	EnabledDefault bool                   `json:"enabled_by_default"`
	Capabilities   map[string]interface{} `json:"capabilities"`
	EntryPoints    map[string]string      `json:"entry_points"`
	Permissions    []string               `json:"permissions"`
}

// NewExternalPluginManager creates a new external plugin manager
func NewExternalPluginManager(db *gorm.DB, logger hclog.Logger) *ExternalPluginManager {
	// Initialize reliability configuration
	reliabilityConfig := config.DefaultPluginReliabilityConfig()

	return &ExternalPluginManager{
		db:               db,
		logger:           logger,
		plugins:          make(map[string]*ExternalPlugin),
		pluginClients:    make(map[string]*goplugin.Client),
		pluginInterfaces: make(map[string]ExternalPluginInterface),

		// NEW: Initialize reliability components
		healthMonitor:     NewPluginHealthMonitor(logger, db),
		fallbackManager:   NewFallbackManager(logger, db, nil), // Use default config
		reliabilityConfig: reliabilityConfig,
	}
}

// Initialize initializes the external plugin manager
func (m *ExternalPluginManager) Initialize(ctx context.Context, pluginDir string, hostServices *HostServices) error {
	m.logger.Info("initializing external plugin manager", "plugin_dir", pluginDir)

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.pluginDir = pluginDir
	m.hostServices = hostServices

	// NEW: Start health monitoring
	go m.healthMonitor.Start()
	go m.fallbackManager.StartCleanupRoutine(m.ctx)

	m.logger.Info("starting reliability monitoring systems")

	// Discover and register plugins from the plugin directory
	if err := m.discoverAndRegisterPlugins(); err != nil {
		return fmt.Errorf("failed to discover plugins: %w", err)
	}

	// Auto-load enabled plugins
	if err := m.autoLoadEnabledPlugins(ctx); err != nil {
		m.logger.Error("failed to auto-load enabled plugins", "error", err)
		// Don't fail initialization if auto-load fails
	}

	m.logger.Info("external plugin manager initialized successfully")
	return nil
}

// SetDashboardManager sets the dashboard manager for plugin integration
func (m *ExternalPluginManager) SetDashboardManager(dashboardManager *DashboardManager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dashboardManager = dashboardManager
	m.logger.Info("dashboard manager set for external plugin integration")

	// Register all already-loaded plugins with the dashboard manager
	m.registerLoadedPluginsWithDashboard()
}

// registerLoadedPluginsWithDashboard registers all currently loaded plugins with the dashboard manager
// This is called when the dashboard manager is set after plugins have already been auto-loaded
func (m *ExternalPluginManager) registerLoadedPluginsWithDashboard() {
	m.logger.Info("registering already-loaded plugins with dashboard manager")

	registeredCount := 0
	for pluginID, pluginInterface := range m.pluginInterfaces {
		if plugin, exists := m.plugins[pluginID]; exists && plugin.Running {
			m.logger.Debug("attempting to register already-loaded plugin with dashboard", "plugin", pluginID, "interface_type", fmt.Sprintf("%T", pluginInterface))

			if grpcClient, ok := pluginInterface.(*ExternalPluginGRPCClient); ok {
				m.logger.Debug("successfully cast already-loaded plugin to ExternalPluginGRPCClient", "plugin", pluginID)
				info, _ := grpcClient.Info()
				adapter := &ExternalPluginAdapter{
					client:     grpcClient,
					pluginInfo: info,
				}

				if err := m.dashboardManager.RegisterPlugin(pluginID, adapter); err != nil {
					m.logger.Warn("failed to register already-loaded plugin with dashboard", "plugin", pluginID, "error", err)
				} else {
					m.logger.Info("registered already-loaded plugin with dashboard manager", "plugin", pluginID)
					registeredCount++
				}
			} else {
				m.logger.Debug("failed to cast already-loaded plugin to ExternalPluginGRPCClient", "plugin", pluginID, "interface_type", fmt.Sprintf("%T", pluginInterface))
			}
		}
	}

	m.logger.Info("completed registration of already-loaded plugins with dashboard", "registered", registeredCount, "total_loaded", len(m.pluginInterfaces))
}

// discoverAndRegisterPlugins scans the plugin directory and registers external plugins
func (m *ExternalPluginManager) discoverAndRegisterPlugins() error {
	m.logger.Info("discovering external plugins", "plugin_dir", m.pluginDir)

	// Check if plugin directory exists
	if _, err := os.Stat(m.pluginDir); os.IsNotExist(err) {
		m.logger.Info("plugin directory does not exist, creating", "dir", m.pluginDir)
		if err := os.MkdirAll(m.pluginDir, 0755); err != nil {
			return fmt.Errorf("failed to create plugin directory: %w", err)
		}
		return nil // Empty directory, no plugins to discover
	}

	// Read plugin directory
	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	discoveredCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDirPath := filepath.Join(m.pluginDir, entry.Name())
		pluginCuePath := filepath.Join(pluginDirPath, "plugin.cue")

		// Check if plugin.cue exists at this level
		if _, err := os.Stat(pluginCuePath); os.IsNotExist(err) {
			// Generic handling for nested plugins - scan one level deeper
			subEntries, err := os.ReadDir(pluginDirPath)
			if err != nil {
				m.logger.Debug("failed to read plugin subdirectory", "path", pluginDirPath, "error", err)
				continue
			}
			
			foundNestedPlugins := false
			for _, subEntry := range subEntries {
				if !subEntry.IsDir() {
					continue
				}
				
				subPluginDirPath := filepath.Join(pluginDirPath, subEntry.Name())
				subPluginCuePath := filepath.Join(subPluginDirPath, "plugin.cue")
				
				if _, err := os.Stat(subPluginCuePath); os.IsNotExist(err) {
					m.logger.Debug("skipping nested directory without plugin.cue", "category", entry.Name(), "dir", subEntry.Name())
					continue
				}
				
				// Parse and register the nested plugin
				manifest, err := m.parsePluginManifest(subPluginCuePath)
				if err != nil {
					m.logger.Error("failed to parse nested plugin manifest", "category", entry.Name(), "plugin", subEntry.Name(), "error", err)
					continue
				}
				
				binaryPath := filepath.Join(subPluginDirPath, manifest.EntryPoints["main"])
				if err := m.registerExternalPlugin(manifest, subPluginDirPath, binaryPath); err != nil {
					m.logger.Error("failed to register nested plugin", "category", entry.Name(), "plugin", manifest.ID, "error", err)
					continue
				}
				
				discoveredCount++
				foundNestedPlugins = true
				m.logger.Info("discovered nested plugin", "category", entry.Name(), "plugin_id", manifest.ID, "name", manifest.Name)
			}
			
			if !foundNestedPlugins {
				m.logger.Debug("skipping directory without plugin.cue or nested plugins", "dir", entry.Name())
			}
			continue
		}

		// Parse plugin manifest
		manifest, err := m.parsePluginManifest(pluginCuePath)
		if err != nil {
			m.logger.Error("failed to parse plugin manifest", "plugin", entry.Name(), "error", err)
			continue
		}

		// Check if binary exists
		binaryPath := filepath.Join(pluginDirPath, manifest.EntryPoints["main"])
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			m.logger.Warn("plugin binary not found", "plugin", manifest.ID, "binary_path", binaryPath)
			// Continue anyway - binary might be built later
		}

		// Register plugin
		if err := m.registerExternalPlugin(manifest, pluginDirPath, binaryPath); err != nil {
			m.logger.Error("failed to register external plugin", "plugin", manifest.ID, "error", err)
			continue
		}

		discoveredCount++
		m.logger.Info("discovered external plugin", "plugin_id", manifest.ID, "name", manifest.Name, "version", manifest.Version)
	}

	m.logger.Info("external plugin discovery completed", "discovered_count", discoveredCount)
	return nil
}

// parsePluginManifest parses a plugin.cue file (simplified parser)
func (m *ExternalPluginManager) parsePluginManifest(cuePath string) (*ExternalPluginManifest, error) {
	// Read the CUE file
	content, err := os.ReadFile(cuePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CUE file: %w", err)
	}

	// Simple CUE parser (basic implementation)
	// This is a simplified implementation - in production you'd want to use a proper CUE parser
	manifest := &ExternalPluginManifest{
		Capabilities: make(map[string]interface{}),
		EntryPoints:  make(map[string]string),
		Permissions:  make([]string, 0),
	}

	lines := strings.Split(string(content), "\n")
	inPluginBlock := false
	inSettingsBlock := false
	inEntryPointsBlock := false
	blockDepth := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(line, "//") || len(line) == 0 {
			continue
		}

		// Track block depth with braces
		openBraces := strings.Count(line, "{")
		closeBraces := strings.Count(line, "}")
		blockDepth += openBraces - closeBraces

		// Check for plugin block start
		if strings.Contains(line, "#Plugin:") && strings.Contains(line, "{") {
			inPluginBlock = true
			continue
		}

		// Handle nested blocks
		if inPluginBlock {
			// Check for settings block (we skip this for manifest parsing)
			if strings.Contains(line, "settings:") && strings.Contains(line, "{") {
				inSettingsBlock = true
				continue
			}

			// Check for entry_points block
			if strings.Contains(line, "entry_points:") && strings.Contains(line, "{") {
				inEntryPointsBlock = true
				continue
			}

			// Skip settings block content
			if inSettingsBlock {
				if blockDepth <= 1 {
					inSettingsBlock = false
				}
				continue
			}

			// Parse entry_points block
			if inEntryPointsBlock {
				if strings.Contains(line, "main:") {
					manifest.EntryPoints["main"] = m.extractQuotedValue(line)
				}
				if blockDepth <= 1 {
					inEntryPointsBlock = false
				}
				continue
			}

			// Parse basic fields
			if strings.Contains(line, "id:") {
				manifest.ID = m.extractQuotedValue(line)
			} else if strings.Contains(line, "name:") {
				manifest.Name = m.extractQuotedValue(line)
			} else if strings.Contains(line, "version:") {
				manifest.Version = m.extractQuotedValue(line)
			} else if strings.Contains(line, "description:") {
				manifest.Description = m.extractQuotedValue(line)
			} else if strings.Contains(line, "author:") {
				manifest.Author = m.extractQuotedValue(line)
			} else if strings.Contains(line, "type:") {
				manifest.Type = m.extractQuotedValue(line)
			} else if strings.Contains(line, "enabled_by_default:") {
				manifest.EnabledDefault = strings.Contains(line, "true")
			}
		}

		// Check if we've exited the plugin block
		if blockDepth <= 0 {
			inPluginBlock = false
		}
	}

	// Set default entry point if not specified
	if manifest.EntryPoints["main"] == "" {
		manifest.EntryPoints["main"] = manifest.ID
	}

	// Validate required fields
	if manifest.ID == "" || manifest.Name == "" {
		return nil, fmt.Errorf("missing required fields (id or name) in plugin manifest")
	}

	return manifest, nil
}

// extractQuotedValue extracts a quoted value from a CUE line
func (m *ExternalPluginManager) extractQuotedValue(line string) string {
	// Find the quoted value after the colon
	colonIndex := strings.Index(line, ":")
	if colonIndex == -1 {
		return ""
	}

	rest := strings.TrimSpace(line[colonIndex+1:])

	// Remove quotes
	if strings.HasPrefix(rest, "\"") && strings.HasSuffix(rest, "\"") {
		return rest[1 : len(rest)-1]
	}

	return rest
}

// registerExternalPlugin registers an external plugin in memory and database
func (m *ExternalPluginManager) registerExternalPlugin(manifest *ExternalPluginManifest, pluginDir, binaryPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create external plugin instance
	plugin := &ExternalPlugin{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Type:        manifest.Type,
		Version:     manifest.Version,
		Description: manifest.Description,
		Running:     false,
		Path:        binaryPath,
	}

	// Store in memory
	m.plugins[manifest.ID] = plugin

	// Ensure plugin exists in database
	if err := m.ensurePluginInDatabase(manifest, pluginDir, binaryPath); err != nil {
		m.logger.Error("failed to register plugin in database", "plugin", manifest.ID, "error", err)
		return fmt.Errorf("failed to register plugin in database: %w", err)
	}

	m.logger.Info("registered external plugin", "plugin_name", manifest.Name, "plugin_id", manifest.ID)
	return nil
}

// ensurePluginInDatabase ensures the plugin is registered in the database
func (m *ExternalPluginManager) ensurePluginInDatabase(manifest *ExternalPluginManifest, pluginDir, binaryPath string) error {
	var dbPlugin database.Plugin

	// Check if plugin exists
	result := m.db.Where("plugin_id = ? AND type = ?", manifest.ID, "external").First(&dbPlugin)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Create new plugin record
			now := time.Now()

			// Determine initial status based on configuration and enabled_by_default
			status := "discovered"

			// Get plugin configuration
			cfg := config.Get().Plugins

			// Check if enrichment plugins should be enabled by default
			isEnrichmentPlugin := manifest.Type == "metadata_scraper" ||
				strings.Contains(manifest.ID, "enricher") ||
				strings.Contains(strings.ToLower(manifest.Name), "enricher")

			// Enable plugin if:
			// 1. It's marked as enabled_by_default AND respect_default_config is true
			// 2. It's an enrichment plugin AND enrichment_enabled is true
			// 3. It's the FFmpeg transcoder (always enabled by default)
			// 4. Binary exists for all cases
			shouldEnable := false
			if cfg.RespectDefaultConfig && manifest.EnabledDefault {
				shouldEnable = true
				m.logger.Info("enabling plugin due to enabled_by_default", "plugin", manifest.ID)
			} else if cfg.EnrichmentEnabled && isEnrichmentPlugin {
				shouldEnable = true
				m.logger.Info("enabling enrichment plugin due to global config", "plugin", manifest.ID)
			} else if manifest.ID == "ffmpeg_transcoder" {
				shouldEnable = true
				m.logger.Info("enabling FFmpeg transcoder by default for seamless operation", "plugin", manifest.ID)
			}

			if shouldEnable {
				if _, err := os.Stat(binaryPath); err == nil {
					status = "enabled"
					m.logger.Info("plugin enabled automatically", "plugin", manifest.ID, "reason", "configuration_default")
				} else {
					m.logger.Warn("plugin marked for enabling but binary not found", "plugin", manifest.ID, "binary_path", binaryPath)
				}
			}

			newPlugin := database.Plugin{
				PluginID:    manifest.ID,
				Name:        manifest.Name, // Use human-readable name from manifest
				Type:        "external",
				Version:     manifest.Version,
				Status:      status,
				Description: manifest.Description,
				InstallPath: pluginDir,
				InstalledAt: now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}

			if err := m.db.Create(&newPlugin).Error; err != nil {
				return fmt.Errorf("failed to create plugin record: %w", err)
			}

			m.logger.Info("registered external plugin", "plugin", manifest.ID, "display_name", manifest.Name, "status", status)
		} else {
			return fmt.Errorf("failed to query plugin: %w", result.Error)
		}
	} else {
		// Plugin exists - update name and version if they have changed
		updated := false

		if dbPlugin.Name != manifest.Name {
			dbPlugin.Name = manifest.Name
			updated = true
		}

		if dbPlugin.Version != manifest.Version {
			dbPlugin.Version = manifest.Version
			updated = true
		}

		if dbPlugin.Description != manifest.Description {
			dbPlugin.Description = manifest.Description
			updated = true
		}

		if updated {
			dbPlugin.UpdatedAt = time.Now()

			if err := m.db.Save(&dbPlugin).Error; err != nil {
				return fmt.Errorf("failed to update plugin record: %w", err)
			}

			m.logger.Info("updated external plugin metadata",
				"plugin", manifest.ID,
				"display_name", manifest.Name,
				"version", manifest.Version)
		}
	}

	return nil
}

// LoadPlugin loads an external plugin using the hashicorp/go-plugin framework
func (m *ExternalPluginManager) LoadPlugin(ctx context.Context, pluginID string) error {
	m.logger.Info("LoadPlugin called", "plugin", pluginID)
	m.mu.Lock()
	defer m.mu.Unlock()

	// NEW: Check circuit breaker before attempting to load
	if !m.healthMonitor.ShouldAllowRequest(pluginID) {
		return fmt.Errorf("plugin %s is circuit broken, refusing to load", pluginID)
	}

	plugin, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	// Check if already running
	if plugin.Running {
		m.logger.Info("plugin already running", "plugin", pluginID)
		return nil
	}

	// NEW: Get plugin-specific configuration
	pluginConfig := m.reliabilityConfig.GetPluginConfig(pluginID)

	// Update database status
	if err := m.updatePluginStatus(pluginID, "loading"); err != nil {
		m.logger.Error("failed to update plugin status", "plugin", pluginID, "error", err)
	}

	// Check if binary exists
	if _, err := os.Stat(plugin.Path); os.IsNotExist(err) {
		m.updatePluginStatus(pluginID, "error")

		// NEW: Record failure in health monitor
		m.healthMonitor.RecordRequest(pluginID, false, 0, err)

		return fmt.Errorf("plugin binary not found: %s", plugin.Path)
	}

	m.logger.Info("starting external plugin via gRPC", "plugin", pluginID, "binary", plugin.Path)

	// NEW: Track start time for performance monitoring
	startTime := time.Now()

	// Create command with environment variables
	cmd := exec.Command(plugin.Path)
	cmd.Dir = filepath.Dir(plugin.Path)
	cmd.Env = append(os.Environ(),
		"VIEWRA_PLUGIN_ID="+pluginID,
		"VIEWRA_DATABASE_URL="+m.getDatabaseURL(),
		"VIEWRA_HOST_SERVICE_ADDR=localhost:50051",
		"VIEWRA_LOG_LEVEL=debug",
		"VIEWRA_BASE_PATH="+filepath.Dir(plugin.Path),
	)

	// Create plugin client using hashicorp/go-plugin with timeout from config
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig: ExternalPluginHandshake,
		Plugins: map[string]goplugin.Plugin{
			"plugin": &GRPCExternalPlugin{},
		},
		Cmd:              cmd,
		Logger:           m.logger.Named(pluginID),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		StartTimeout:     pluginConfig.RequestTimeout, // NEW: Use configured timeout
	})

	// Connect to the plugin
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		m.updatePluginStatus(pluginID, "error")

		// NEW: Record failure in health monitor
		responseTime := time.Since(startTime)
		m.healthMonitor.RecordRequest(pluginID, false, responseTime, err)

		return fmt.Errorf("failed to connect to plugin: %w", err)
	}

	// Request the plugin interface
	raw, err := rpcClient.Dispense("plugin")
	if err != nil {
		client.Kill()
		m.updatePluginStatus(pluginID, "error")

		// NEW: Record failure in health monitor
		responseTime := time.Since(startTime)
		m.healthMonitor.RecordRequest(pluginID, false, responseTime, err)

		return fmt.Errorf("failed to dispense plugin: %w", err)
	}

	// Convert to our plugin interface
	pluginInterface, ok := raw.(ExternalPluginInterface)
	if !ok {
		client.Kill()
		m.updatePluginStatus(pluginID, "error")

		// NEW: Record failure in health monitor
		responseTime := time.Since(startTime)
		interfaceErr := errors.New(ErrPluginInterface)
		m.healthMonitor.RecordRequest(pluginID, false, responseTime, interfaceErr)

		return errors.New(ErrPluginInterface)
	}

	// Initialize the plugin
	pluginCtx := &ExternalPluginContext{
		PluginID:        pluginID,
		DatabaseURL:     m.getDatabaseURL(),
		HostServiceAddr: "localhost:50051", // Enrichment service address
		LogLevel:        "debug",
		BasePath:        filepath.Dir(plugin.Path),
	}

	if err := pluginInterface.Initialize(pluginCtx); err != nil {
		client.Kill()
		m.updatePluginStatus(pluginID, "error")

		// NEW: Record failure in health monitor
		responseTime := time.Since(startTime)
		m.healthMonitor.RecordRequest(pluginID, false, responseTime, err)

		return fmt.Errorf("failed to initialize plugin: %w", err)
	}

	// Start the plugin
	if err := pluginInterface.Start(); err != nil {
		client.Kill()
		m.updatePluginStatus(pluginID, "error")

		// NEW: Record failure in health monitor
		responseTime := time.Since(startTime)
		m.healthMonitor.RecordRequest(pluginID, false, responseTime, err)

		return fmt.Errorf("failed to start plugin: %w", err)
	}

	// Set up database tables for the plugin
	if err := m.setupPluginDatabase(pluginID, pluginInterface); err != nil {
		m.logger.Warn("failed to setup plugin database", "plugin", pluginID, "error", err)
		// Continue anyway - plugin might not need database tables
	}

	// Store the client and interface references
	m.pluginClients[pluginID] = client
	m.pluginInterfaces[pluginID] = pluginInterface

	// Update plugin status
	plugin.Running = true
	plugin.LastStarted = time.Now()

	// NEW: Register plugin with health monitor and record successful start
	m.registerPluginHealth(pluginID, pluginInterface)
	responseTime := time.Since(startTime)
	m.healthMonitor.RecordRequest(pluginID, true, responseTime, nil)

	// Monitor the plugin process in a goroutine
	go m.monitorPluginProcess(pluginID, client)

	// Update database status to running
	if err := m.updatePluginStatus(pluginID, "running"); err != nil {
		m.logger.Error("failed to update plugin status", "plugin", pluginID, "error", err)
	}

	// Discover and register admin pages from the plugin
	if err := m.discoverAndRegisterAdminPages(pluginID, pluginInterface); err != nil {
		m.logger.Warn("failed to discover admin pages", "plugin", pluginID, "error", err)
		// Continue anyway - plugin might not provide admin pages
	}

	// Register plugin with dashboard manager if available
	if m.dashboardManager != nil {
		// Create an adapter to expose the plugin interface
		m.logger.Debug("attempting dashboard registration", "plugin", pluginID, "interface_type", fmt.Sprintf("%T", pluginInterface))
		if grpcClient, ok := pluginInterface.(*ExternalPluginGRPCClient); ok {
			m.logger.Debug("successfully cast to ExternalPluginGRPCClient", "plugin", pluginID)
			info, _ := grpcClient.Info()
			adapter := &ExternalPluginAdapter{
				client:     grpcClient,
				pluginInfo: info,
			}

			// Register with dashboard manager if available
			if m.dashboardManager != nil {
				m.logger.Debug("dashboard manager available, registering plugin", "plugin", pluginID)
				if err := m.dashboardManager.RegisterPlugin(pluginID, adapter); err != nil {
					m.logger.Warn("failed to register plugin with dashboard", "plugin", pluginID, "error", err)
					// Continue anyway - plugin might not provide dashboard sections
				} else {
					m.logger.Info("registered plugin with dashboard manager", "plugin", pluginID)
				}
			} else {
				m.logger.Warn("dashboard manager is nil, cannot register plugin", "plugin", pluginID)
			}
		} else {
			m.logger.Debug("failed to cast to ExternalPluginGRPCClient", "plugin", pluginID, "interface_type", fmt.Sprintf("%T", pluginInterface))
		}
	}

	m.logger.Info("successfully loaded external plugin", "plugin", pluginID, "load_time", responseTime)
	return nil
}

// setupPluginDatabase sets up database tables for the plugin
func (m *ExternalPluginManager) setupPluginDatabase(pluginID string, pluginInterface ExternalPluginInterface) error {
	// Retry getting models in case plugin is still initializing
	maxRetries := 3
	retryDelay := 1 * time.Second

	var models []string
	for attempt := 1; attempt <= maxRetries; attempt++ {
		models = pluginInterface.GetModels()
		if len(models) > 0 {
			break
		}

		if attempt < maxRetries {
			m.logger.Debug("plugin returned no models, retrying", "plugin", pluginID, "attempt", attempt, "max_retries", maxRetries)
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		}
	}

	if len(models) == 0 {
		m.logger.Debug("plugin has no database models after retries", "plugin", pluginID)
		return nil
	}

	m.logger.Info("setting up database for plugin", "plugin", pluginID, "models", len(models))

	// Run migration
	if err := pluginInterface.Migrate(m.getDatabaseURL()); err != nil {
		return fmt.Errorf("failed to migrate plugin database: %w", err)
	}

	m.logger.Info("successfully set up plugin database", "plugin", pluginID)
	return nil
}

// monitorPluginProcess monitors the plugin process and handles cleanup when it exits
func (m *ExternalPluginManager) monitorPluginProcess(pluginID string, client *goplugin.Client) {
	// Wait for the client to exit - monitoring via process check instead of Done()
	go func() {
		// Check if client is alive periodically
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-time.After(5 * time.Second):
				// Check if the plugin process is still running
				if !client.Exited() {
					continue
				}

				// Plugin has exited, clean up
				m.mu.Lock()

				// Update plugin status
				if plugin, exists := m.plugins[pluginID]; exists {
					plugin.Running = false
					plugin.LastStopped = time.Now()
				}

				// Clean up references
				delete(m.pluginClients, pluginID)
				delete(m.pluginInterfaces, pluginID)

				// Update database status
				if err := m.updatePluginStatus(pluginID, "stopped"); err != nil {
					m.logger.Error("failed to update plugin status", "plugin", pluginID, "error", err)
				}

				m.logger.Info("plugin process stopped", "plugin", pluginID)
				m.mu.Unlock()
				return
			}
		}
	}()
}

// UnloadPlugin unloads an external plugin
func (m *ExternalPluginManager) UnloadPlugin(ctx context.Context, pluginID string) error {
	return m.unloadPlugin(pluginID)
}

// Shutdown gracefully shuts down the external plugin manager
func (m *ExternalPluginManager) Shutdown(ctx context.Context) error {
	m.logger.Info("shutting down external plugin manager")

	if m.cancel != nil {
		m.cancel()
	}

	// Shutdown all running plugins
	m.mu.RLock()
	var runningPlugins []string
	for id, plugin := range m.plugins {
		if plugin.Running {
			runningPlugins = append(runningPlugins, id)
		}
	}
	m.mu.RUnlock()

	for _, pluginID := range runningPlugins {
		if err := m.unloadPlugin(pluginID); err != nil {
			m.logger.Error("failed to unload plugin during shutdown", "plugin", pluginID, "error", err)
		}
	}

	m.logger.Info("external plugin manager shutdown complete")
	return nil
}

// GetPlugin returns an external plugin by ID
func (m *ExternalPluginManager) GetPlugin(pluginID string) (*ExternalPlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	plugin, exists := m.plugins[pluginID]
	return plugin, exists
}

// ListPlugins returns all external plugins
func (m *ExternalPluginManager) ListPlugins() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var infos []PluginInfo
	for _, plugin := range m.plugins {
		info := PluginInfo{
			ID:          plugin.ID,   // External plugins use ID field
			Name:        plugin.Name, // Human-readable name from manifest
			Type:        plugin.Type,
			Version:     plugin.Version,
			Description: plugin.Description,
			Enabled:     plugin.Running,
			IsCore:      false, // External plugins are never core
			Category:    fmt.Sprintf("external_%s", plugin.Type),
		}
		infos = append(infos, info)
	}
	return infos
}

// GetRunningPlugins returns all running external plugins
func (m *ExternalPluginManager) GetRunningPlugins() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var infos []PluginInfo
	for _, plugin := range m.plugins {
		if plugin.Running {
			info := PluginInfo{
				ID:          plugin.ID,
				Name:        plugin.Name,
				Type:        plugin.Type,
				Version:     plugin.Version,
				Description: plugin.Description,
				Enabled:     true,
				IsCore:      false,
				Category:    fmt.Sprintf("external_%s", plugin.Type),
			}
			infos = append(infos, info)
		}
	}
	return infos
}

func (m *ExternalPluginManager) unloadPlugin(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	if !plugin.Running {
		m.logger.Info("plugin already stopped", "plugin", pluginID)
		return nil
	}

	m.logger.Info("stopping external plugin", "plugin", pluginID)

	// Stop the plugin gracefully if we have the interface
	if pluginInterface, exists := m.pluginInterfaces[pluginID]; exists {
		if err := pluginInterface.Stop(); err != nil {
			m.logger.Error("failed to stop plugin gracefully", "plugin", pluginID, "error", err)
		}
		delete(m.pluginInterfaces, pluginID)
	}

	// Kill the plugin process
	if client, exists := m.pluginClients[pluginID]; exists {
		client.Kill()
		delete(m.pluginClients, pluginID)
	}

	// Update plugin status
	plugin.Running = false
	plugin.LastStopped = time.Now()

	// Unregister from dashboard manager if available
	if m.dashboardManager != nil {
		if err := m.dashboardManager.UnregisterPlugin(pluginID); err != nil {
			m.logger.Warn("failed to unregister plugin from dashboard", "plugin", pluginID, "error", err)
		} else {
			m.logger.Info("unregistered plugin from dashboard manager", "plugin", pluginID)
		}
	}

	// Update database status
	if err := m.updatePluginStatus(pluginID, "stopped"); err != nil {
		m.logger.Error("failed to update plugin status", "plugin", pluginID, "error", err)
	}

	m.logger.Info("stopped external plugin", "plugin", pluginID)
	return nil
}

// getDatabaseURL returns the database connection URL for plugins
func (m *ExternalPluginManager) getDatabaseURL() string {
	// Get the actual database configuration
	cfg := config.Get().Database

	// For SQLite, return the correct path based on configuration
	if cfg.Type == "sqlite" {
		// Use the configured database path, or construct from data dir
		dbPath := cfg.DatabasePath
		if dbPath == "" {
			dbPath = filepath.Join(cfg.DataDir, "viewra.db")
		}

		// Convert to absolute path if it's not already
		if !filepath.IsAbs(dbPath) {
			// Make relative to current working directory
			if absPath, err := filepath.Abs(dbPath); err == nil {
				dbPath = absPath
			}
		}

		return fmt.Sprintf("sqlite://%s", dbPath)
	}

	// For PostgreSQL, construct URL from components
	if cfg.URL != "" {
		return cfg.URL
	}

	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
}

// GetEnabledFileHandlers returns enabled external plugins that can handle files
func (m *ExternalPluginManager) GetEnabledFileHandlers() []FileHandlerPlugin {
	// For now, external plugins don't directly handle files during scanning
	// They respond to scan events via ScannerHookService
	return []FileHandlerPlugin{}
}

// GetRunningPluginInterface returns the interface for a running plugin
func (m *ExternalPluginManager) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pluginInterface, exists := m.pluginInterfaces[pluginID]
	if !exists {
		fmt.Printf("DEBUG: GetRunningPluginInterface - plugin interface not found: %s\n", pluginID)
		return nil, false
	}

	// For transcoder plugins, return an adapter that implements plugins.Implementation
	plugin, pluginExists := m.plugins[pluginID]
	fmt.Printf("DEBUG: GetRunningPluginInterface - plugin=%s, exists=%v", pluginID, pluginExists)
	if pluginExists {
		fmt.Printf(", type=%s\n", plugin.Type)
	} else {
		fmt.Printf(", plugin not in plugins map\n")
	}

	if pluginExists && plugin.Type == "transcoder" {
		if grpcClient, ok := pluginInterface.(*ExternalPluginGRPCClient); ok {
			info, _ := grpcClient.Info()
			adapter := &ExternalPluginAdapter{
				client:     grpcClient,
				pluginInfo: info,
			}
			fmt.Printf("DEBUG: GetRunningPluginInterface - returning adapter for transcoder: %s\n", pluginID)
			return adapter, true
		} else {
			fmt.Printf("DEBUG: GetRunningPluginInterface - pluginInterface is not *ExternalPluginGRPCClient: %T\n", pluginInterface)
		}
	}

	fmt.Printf("DEBUG: GetRunningPluginInterface - returning raw interface for non-transcoder: %s\n", pluginID)
	return pluginInterface, exists
}

// NotifyMediaFileScanned notifies all running external plugins about a scanned media file
func (m *ExternalPluginManager) NotifyMediaFileScanned(mediaFileID string, filePath string, metadata map[string]string) {
	m.mu.RLock()
	runningPlugins := make(map[string]ExternalPluginInterface)
	for id, iface := range m.pluginInterfaces {
		runningPlugins[id] = iface
	}
	m.mu.RUnlock()

	for pluginID, pluginInterface := range runningPlugins {
		go func(id string, iface ExternalPluginInterface) {
			// NEW: Check circuit breaker before making request
			if !m.healthMonitor.ShouldAllowRequest(id) {
				m.logger.Warn("skipping plugin notification due to circuit breaker", "plugin_id", id)
				return
			}

			// NEW: Track request time
			startTime := time.Now()

			// NEW: Prepare fallback request for both success and failure scenarios
			fallbackRequest := &FallbackRequest{
				PluginID:    id,
				Operation:   "OnMediaFileScanned",
				MediaFileID: mediaFileID,
				RequestTime: startTime,
				Parameters: map[string]interface{}{
					"file_path": filePath,
					"metadata":  metadata,
				},
			}

			err := iface.OnMediaFileScanned(mediaFileID, filePath, metadata)

			// NEW: Record request result in health monitor
			responseTime := time.Since(startTime)
			success := err == nil
			m.healthMonitor.RecordRequest(id, success, responseTime, err)

			if err != nil {
				m.logger.Error("plugin media file notification failed", "plugin", id, "error", err)

				// NEW: Try fallback if available
				fallbackRequest.OriginalError = err

				if fallbackResponse, fallbackErr := m.fallbackManager.HandleFailure(context.Background(), fallbackRequest); fallbackErr == nil {
					m.logger.Info("fallback handled plugin failure",
						"plugin_id", id,
						"strategy", fallbackResponse.Strategy,
						"from_cache", fallbackResponse.FromCache)
				}
			} else {
				// NEW: Cache successful operation for future fallback
				cacheKey := fmt.Sprintf("%s:%s:%s", id, "OnMediaFileScanned", mediaFileID)
				cacheData := map[string]interface{}{
					"media_file_id": mediaFileID,
					"file_path":     filePath,
					"metadata":      metadata,
					"success":       true,
				}
				m.fallbackManager.StoreCacheEntry(cacheKey, cacheData, id, 1.0)
			}
		}(pluginID, pluginInterface)
	}
}

// NotifyScanStarted notifies all running external plugins that a scan has started
func (m *ExternalPluginManager) NotifyScanStarted(scanJobID, libraryID uint32, libraryPath string) {
	m.mu.RLock()
	runningPlugins := make(map[string]ExternalPluginInterface)
	for id, iface := range m.pluginInterfaces {
		runningPlugins[id] = iface
	}
	m.mu.RUnlock()

	for pluginID, pluginInterface := range runningPlugins {
		go func(id string, iface ExternalPluginInterface) {
			if err := iface.OnScanStarted(scanJobID, libraryID, libraryPath); err != nil {
				m.logger.Error("plugin scan start notification failed", "plugin", id, "error", err)
			}
		}(pluginID, pluginInterface)
	}
}

// NotifyScanCompleted notifies all running external plugins that a scan has completed
func (m *ExternalPluginManager) NotifyScanCompleted(scanJobID, libraryID uint32, stats map[string]string) {
	m.mu.RLock()
	runningPlugins := make(map[string]ExternalPluginInterface)
	for id, iface := range m.pluginInterfaces {
		runningPlugins[id] = iface
	}
	m.mu.RUnlock()

	for pluginID, pluginInterface := range runningPlugins {
		go func(id string, iface ExternalPluginInterface) {
			if err := iface.OnScanCompleted(scanJobID, libraryID, stats); err != nil {
				m.logger.Error("plugin scan completion notification failed", "plugin", id, "error", err)
			}
		}(pluginID, pluginInterface)
	}
}

// updatePluginStatus updates the plugin status in the database
func (m *ExternalPluginManager) updatePluginStatus(pluginID, status string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": now,
	}

	// Set enabled_at for enabled status
	if status == "enabled" || status == "running" {
		updates["enabled_at"] = &now
	}

	return m.db.Model(&database.Plugin{}).
		Where("plugin_id = ? AND type = ?", pluginID, "external").
		Updates(updates).Error
}

// RefreshPlugins re-discovers and re-registers external plugins
func (m *ExternalPluginManager) RefreshPlugins() error {
	m.logger.Info("refreshing external plugins")
	return m.discoverAndRegisterPlugins()
}

// EnablePlugin enables an external plugin
func (m *ExternalPluginManager) EnablePlugin(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	// Update database status
	if err := m.updatePluginStatus(pluginID, "enabled"); err != nil {
		return fmt.Errorf("failed to update plugin status in database: %w", err)
	}

	plugin.Running = false // Not actually running yet, just enabled
	m.logger.Info("enabled external plugin", "plugin", pluginID)
	return nil
}

// DisablePlugin disables an external plugin
func (m *ExternalPluginManager) DisablePlugin(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	// Stop if running
	if plugin.Running {
		plugin.Running = false
		plugin.LastStopped = time.Now()
	}

	// Update database status
	if err := m.updatePluginStatus(pluginID, "disabled"); err != nil {
		return fmt.Errorf("failed to update plugin status in database: %w", err)
	}

	m.logger.Info("disabled external plugin", "plugin", pluginID)
	return nil
}

// autoLoadEnabledPlugins auto-loads plugins that are enabled or were previously running
func (m *ExternalPluginManager) autoLoadEnabledPlugins(ctx context.Context) error {
	m.logger.Info("auto-loading enabled plugins")

	// Query the database for plugins marked as 'enabled' or 'running'
	var enabledPlugins []database.Plugin
	if err := m.db.Where("status IN (?, ?)", "enabled", "running").Find(&enabledPlugins).Error; err != nil {
		return fmt.Errorf("failed to query enabled plugins: %w", err)
	}

	loadedCount := 0
	failedCount := 0

	for _, dbPlugin := range enabledPlugins {
		pluginID := dbPlugin.PluginID
		plugin, exists := m.plugins[pluginID]
		if !exists {
			m.logger.Warn("plugin not found in memory", "plugin", pluginID)
			continue
		}

		// Check if already running (avoid duplicate loading)
		if plugin.Running {
			m.logger.Debug("plugin already running, skipping", "plugin", pluginID)
			loadedCount++
			continue
		}

		// Check if binary exists
		if _, err := os.Stat(plugin.Path); os.IsNotExist(err) {
			m.logger.Warn("plugin binary not found, marking as error", "plugin", pluginID, "binary_path", plugin.Path)
			m.updatePluginStatus(pluginID, "error")
			failedCount++
			continue
		}

		m.logger.Info("starting external plugin", "plugin", pluginID)

		// Update database status to loading
		if err := m.updatePluginStatus(pluginID, "loading"); err != nil {
			m.logger.Warn("failed to update plugin status to loading", "plugin", pluginID, "error", err)
		}

		// Create command with environment variables
		cmd := exec.Command(plugin.Path)
		cmd.Dir = filepath.Dir(plugin.Path)
		cmd.Env = append(os.Environ(),
			"VIEWRA_PLUGIN_ID="+pluginID,
			"VIEWRA_DATABASE_URL="+m.getDatabaseURL(),
			"VIEWRA_HOST_SERVICE_ADDR=localhost:50051",
			"VIEWRA_LOG_LEVEL=debug",
			"VIEWRA_BASE_PATH="+filepath.Dir(plugin.Path),
		)

		// Create plugin client using hashicorp/go-plugin
		client := goplugin.NewClient(&goplugin.ClientConfig{
			HandshakeConfig: ExternalPluginHandshake,
			Plugins: map[string]goplugin.Plugin{
				"plugin": &GRPCExternalPlugin{},
			},
			Cmd:              cmd,
			Logger:           m.logger.Named(pluginID),
			AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		})

		// Connect to the plugin
		rpcClient, err := client.Client()
		if err != nil {
			client.Kill()
			m.logger.Error("failed to connect to plugin", "plugin", pluginID, "error", err)
			m.updatePluginStatus(pluginID, "error")
			failedCount++
			continue
		}

		// Request the plugin interface
		raw, err := rpcClient.Dispense("plugin")
		if err != nil {
			client.Kill()
			m.logger.Error("failed to dispense plugin", "plugin", pluginID, "error", err)
			m.updatePluginStatus(pluginID, "error")
			failedCount++
			continue
		}

		// Convert to our plugin interface
		pluginInterface, ok := raw.(ExternalPluginInterface)
		if !ok {
			client.Kill()
			m.logger.Error("plugin does not implement required interface", "plugin", pluginID)
			m.updatePluginStatus(pluginID, "error")
			failedCount++
			continue
		}

		// Initialize the plugin
		pluginCtx := &ExternalPluginContext{
			PluginID:        pluginID,
			DatabaseURL:     m.getDatabaseURL(),
			HostServiceAddr: "localhost:50051", // Enrichment service address
			LogLevel:        "debug",
			BasePath:        filepath.Dir(plugin.Path),
		}

		if err := pluginInterface.Initialize(pluginCtx); err != nil {
			client.Kill()
			m.logger.Error("failed to initialize plugin", "plugin", pluginID, "error", err)
			m.updatePluginStatus(pluginID, "error")
			failedCount++
			continue
		}

		// Start the plugin
		if err := pluginInterface.Start(); err != nil {
			client.Kill()
			m.logger.Error("failed to start plugin", "plugin", pluginID, "error", err)
			m.updatePluginStatus(pluginID, "error")
			failedCount++
			continue
		}

		// Test plugin health
		if err := pluginInterface.Health(); err != nil {
			m.logger.Warn("plugin health check failed", "plugin", pluginID, "error", err)
			// Don't fail loading for health check failures, just log
		}

		// Set up database tables for the plugin
		if err := m.setupPluginDatabase(pluginID, pluginInterface); err != nil {
			m.logger.Warn("failed to setup plugin database", "plugin", pluginID, "error", err)
			// Continue anyway - plugin might not need database tables
		}

		// Store the client and interface references
		m.pluginClients[pluginID] = client
		m.pluginInterfaces[pluginID] = pluginInterface

		// Update plugin status
		plugin.Running = true
		plugin.LastStarted = time.Now()

		// Monitor the plugin process in a goroutine
		go m.monitorPluginProcess(pluginID, client)

		// Update database status to running
		if err := m.updatePluginStatus(pluginID, "running"); err != nil {
			m.logger.Error("failed to update plugin status to running", "plugin", pluginID, "error", err)
		}

		// Register plugin with dashboard manager if available
		if m.dashboardManager != nil {
			// Create an adapter to expose the plugin interface
			m.logger.Debug("attempting dashboard registration for auto-loaded plugin", "plugin", pluginID, "interface_type", fmt.Sprintf("%T", pluginInterface))
			if grpcClient, ok := pluginInterface.(*ExternalPluginGRPCClient); ok {
				m.logger.Debug("successfully cast auto-loaded plugin to ExternalPluginGRPCClient", "plugin", pluginID)
				info, _ := grpcClient.Info()
				adapter := &ExternalPluginAdapter{
					client:     grpcClient,
					pluginInfo: info,
				}

				if err := m.dashboardManager.RegisterPlugin(pluginID, adapter); err != nil {
					m.logger.Warn("failed to register auto-loaded plugin with dashboard", "plugin", pluginID, "error", err)
					// Continue anyway - plugin might not provide dashboard sections
				} else {
					m.logger.Info("registered auto-loaded plugin with dashboard manager", "plugin", pluginID)
				}
			} else {
				m.logger.Debug("failed to cast auto-loaded plugin to ExternalPluginGRPCClient", "plugin", pluginID, "interface_type", fmt.Sprintf("%T", pluginInterface))
			}
		} else {
			m.logger.Warn("dashboard manager is nil, cannot register auto-loaded plugin", "plugin", pluginID)
		}

		loadedCount++
		m.logger.Info("successfully auto-loaded external plugin", "plugin", pluginID)
	}

	m.logger.Info("auto-loading completed", "loaded", loadedCount, "failed", failedCount, "total", len(enabledPlugins))

	// Return error if all plugins failed to load
	if len(enabledPlugins) > 0 && loadedCount == 0 {
		return fmt.Errorf("failed to load any enabled plugins (%d failed)", failedCount)
	}

	return nil
}

// NEW: Health monitoring and reliability methods

// GetPluginHealth returns health metrics for a specific plugin
func (m *ExternalPluginManager) GetPluginHealth(pluginID string) (*PluginHealthState, error) {
	return m.healthMonitor.GetPluginHealth(pluginID)
}

// GetAllPluginHealth returns health metrics for all monitored plugins
func (m *ExternalPluginManager) GetAllPluginHealth() map[string]*PluginHealthState {
	return m.healthMonitor.GetAllPluginHealth()
}

// IsPluginHealthy checks if a plugin is in healthy state
func (m *ExternalPluginManager) IsPluginHealthy(pluginID string) bool {
	if health, err := m.healthMonitor.GetPluginHealth(pluginID); err == nil {
		return health.Status == "healthy"
	}
	return false
}

// GetPluginReliabilityStatus returns overall reliability status
func (m *ExternalPluginManager) GetPluginReliabilityStatus(pluginID string) map[string]interface{} {
	health, err := m.healthMonitor.GetPluginHealth(pluginID)
	if err != nil {
		return map[string]interface{}{
			"plugin_id": pluginID,
			"status":    "unknown",
			"message":   "Plugin not found or not monitored",
		}
	}

	// Check if circuit breaker is open
	circuitBreakerOpen := !m.healthMonitor.ShouldAllowRequest(pluginID)

	return map[string]interface{}{
		"plugin_id":             pluginID,
		"status":                health.Status,
		"consecutive_failures":  health.ConsecutiveFailures,
		"total_requests":        health.GetTotalRequests(),
		"successful_requests":   health.GetSuccessfulRequests(),
		"failed_requests":       health.GetFailedRequests(),
		"success_rate":          health.GetSuccessRate(),
		"error_rate":            health.GetErrorRate(),
		"average_response_time": health.GetAverageResponseTime().String(),
		"last_check_time":       health.GetLastCheckTime(),
		"circuit_breaker_open":  circuitBreakerOpen,
		"uptime":                health.GetUptime().String(),
		"last_error":            health.LastError,
	}
}

// CheckAllPluginsHealth checks the health of all registered plugins and returns a summary
func (m *ExternalPluginManager) CheckAllPluginsHealth() map[string]interface{} {
	m.mu.RLock()
	pluginCount := len(m.plugins)
	m.mu.RUnlock()

	allHealth := m.healthMonitor.GetAllPluginHealth()

	summary := map[string]interface{}{
		"total_plugins":     pluginCount,
		"monitored_plugins": len(allHealth),
		"healthy_count":     0,
		"degraded_count":    0,
		"unhealthy_count":   0,
		"unknown_count":     pluginCount - len(allHealth),
		"plugins":           make(map[string]interface{}),
	}

	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0

	for pluginID, health := range allHealth {
		switch health.Status {
		case "healthy":
			healthyCount++
		case "degraded":
			degradedCount++
		case "unhealthy":
			unhealthyCount++
		}

		// Check if circuit breaker is open
		circuitBreakerOpen := !m.healthMonitor.ShouldAllowRequest(pluginID)

		summary["plugins"].(map[string]interface{})[pluginID] = map[string]interface{}{
			"status":                health.Status,
			"consecutive_failures":  health.ConsecutiveFailures,
			"success_rate":          health.GetSuccessRate(),
			"error_rate":            health.GetErrorRate(),
			"average_response_time": health.GetAverageResponseTime().String(),
			"last_check_time":       health.GetLastCheckTime(),
			"circuit_breaker_open":  circuitBreakerOpen,
		}
	}

	summary["healthy_count"] = healthyCount
	summary["degraded_count"] = degradedCount
	summary["unhealthy_count"] = unhealthyCount

	return summary
}

// Discover and register admin pages from the plugin
func (m *ExternalPluginManager) discoverAndRegisterAdminPages(pluginID string, pluginInterface ExternalPluginInterface) error {
	m.logger.Info("discovering admin pages for plugin", "plugin", pluginID)

	// Get the GRPC client from the stored interfaces
	if _, exists := m.pluginInterfaces[pluginID]; exists {
		// We need to access the GRPC client directly since ExternalPluginInterface doesn't have admin page methods
		m.logger.Debug("plugin interface found, attempting to discover admin pages", "plugin", pluginID)

		// For now, we'll try to call via the GRPC connection directly
		// Get the raw GRPC client
		if grpcClient, ok := m.pluginClients[pluginID]; ok {
			return m.discoverAdminPagesViaGRPC(pluginID, grpcClient)
		}
	}

	m.logger.Debug("no admin page support found for plugin", "plugin", pluginID)
	return nil
}

// discoverAdminPagesViaGRPC discovers admin pages using direct GRPC communication
// Helper function for parsing int strings to int32
func parseIntToInt32(value string) int32 {
	if value == "" {
		return 0
	}
	var val int
	if _, err := fmt.Sscanf(value, "%d", &val); err == nil {
		return int32(val)
	}
	return 0
}

func (m *ExternalPluginManager) discoverAdminPagesViaGRPC(pluginID string, client *goplugin.Client) error {
	// Get the raw GRPC client
	rpcClient, err := client.Client()
	if err != nil {
		return fmt.Errorf("failed to get RPC client: %w", err)
	}

	// Get the plugin interface
	raw, err := rpcClient.Dispense("plugin")
	if err != nil {
		return fmt.Errorf("failed to dispense plugin: %w", err)
	}

	// Cast to ExternalPluginGRPCClient (our external plugin client)
	grpcClient, ok := raw.(*ExternalPluginGRPCClient)
	if !ok {
		return fmt.Errorf("plugin does not support external GRPC interface")
	}

	// Try to get admin pages using our external client's method
	pages, err := grpcClient.GetAdminPages()
	if err != nil {
		// Plugin might not implement admin pages, which is fine
		m.logger.Debug("plugin does not provide admin pages", "plugin", pluginID, "error", err)
		return nil
	}

	if len(pages) == 0 {
		m.logger.Debug("plugin has no admin pages", "plugin", pluginID)
		return nil
	}

	m.logger.Info("discovered admin pages", "plugin", pluginID, "count", len(pages))

	// Store admin pages in database
	for _, page := range pages {
		adminPage := &database.PluginAdminPage{
			PluginID: pluginID,
			PageID:   page.Id,
			Title:    page.Title,
			Path:     page.Path,
			Icon:     page.Icon,
			Category: page.Category,
			URL:      page.Url,
			Type:     page.Type,
			Enabled:  true,
		}

		// Upsert the admin page
		result := m.db.Where("plugin_id = ? AND page_id = ?", pluginID, page.Id).FirstOrCreate(adminPage)
		if result.Error != nil {
			m.logger.Error("failed to save admin page", "plugin", pluginID, "page", page.Id, "error", result.Error)
			continue
		}

		m.logger.Debug("registered admin page", "plugin", pluginID, "page", page.Id, "title", page.Title)
	}

	return nil
}

// ExternalTranscodingProvider wraps an external plugin to provide transcoding services
type ExternalTranscodingProvider struct {
	pluginID   string
	pluginInfo *ExternalPluginInfo
	client     *ExternalPluginGRPCClient
}

// Ensure it implements the interface
var _ plugins.TranscodingProvider = (*ExternalTranscodingProvider)(nil)

// GetInfo returns provider information
func (p *ExternalTranscodingProvider) GetInfo() plugins.ProviderInfo {
	return plugins.ProviderInfo{
		ID:          p.pluginID,
		Name:        p.pluginInfo.Name,
		Version:     p.pluginInfo.Version,
		Description: p.pluginInfo.Description,
		Author:      p.pluginInfo.Author,
		Priority:    50, // Default priority
	}
}

// GetSupportedFormats returns supported container formats
func (p *ExternalTranscodingProvider) GetSupportedFormats() []plugins.ContainerFormat {
	// Create gRPC client
	client := proto.NewTranscodingProviderServiceClient(p.client.conn)
	
	// Make gRPC call to get supported formats
	ctx := context.Background()
	resp, err := client.GetSupportedFormats(ctx, &proto.GetSupportedFormatsRequest{})
	if err != nil {
		// Log gRPC error and fallback to empty list
		fmt.Printf("ERROR: gRPC GetSupportedFormats failed: %v\n", err)
		return []plugins.ContainerFormat{}
	}
	
	fmt.Printf("SUCCESS: gRPC GetSupportedFormats returned %d formats\n", len(resp.Formats))
	
	// Convert proto formats to SDK formats
	formats := make([]plugins.ContainerFormat, len(resp.Formats))
	for i, protoFormat := range resp.Formats {
		formats[i] = plugins.ContainerFormat{
			Format:      protoFormat.Name,
			Description: protoFormat.Description,
			Extensions:  protoFormat.Extensions,
			// Note: protobuf doesn't have MimeType or Adaptive fields, 
			// these will need to be added to proto definition if needed
		}
	}
	
	return formats
}

// GetHardwareAccelerators returns available hardware accelerators
func (p *ExternalTranscodingProvider) GetHardwareAccelerators() []plugins.HardwareAccelerator {
	// TODO: Query plugin via GRPC
	return []plugins.HardwareAccelerator{
		{
			Type:        "auto",
			ID:          "auto",
			Name:        "Auto-detect",
			Available:   true,
			DeviceCount: 0,
		},
	}
}

// GetQualityPresets returns available quality presets
func (p *ExternalTranscodingProvider) GetQualityPresets() []plugins.QualityPreset {
	return []plugins.QualityPreset{
		{
			ID:          "high",
			Name:        "High Quality",
			Description: "Best quality, slower encoding",
			Quality:     90,
			SpeedRating: 3,
			SizeRating:  8,
		},
		{
			ID:          "balanced",
			Name:        "Balanced",
			Description: "Good quality/speed balance",
			Quality:     70,
			SpeedRating: 6,
			SizeRating:  5,
		},
		{
			ID:          "fast",
			Name:        "Fast",
			Description: "Faster encoding, lower quality",
			Quality:     50,
			SpeedRating: 9,
			SizeRating:  3,
		},
	}
}

// StartTranscode starts a new transcoding job
func (p *ExternalTranscodingProvider) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	logger := hclog.Default().Named("external-transcoding-provider")
	logger.Info("StartTranscode called on external provider",
		"plugin_id", p.pluginID,
		"session_id", req.SessionID,
		"input_path", req.InputPath,
		"output_path", req.OutputPath,
		"container", req.Container)

	// Create gRPC client
	client := proto.NewTranscodingProviderServiceClient(p.client.conn)
	
	// Convert SDK request to proto request
	protoReq := &proto.StartTranscodeProviderRequest{
		Request: &proto.TranscodeProviderRequest{
			SessionId:         req.SessionID,
			InputPath:         req.InputPath,
			OutputDir:         "", // Let the plugin handle directory creation
			Quality:           int32(req.Quality),
			SpeedPriority:     string(req.SpeedPriority),
			Container:         req.Container,
			VideoCodec:        req.VideoCodec,
			AudioCodec:        req.AudioCodec,
			PreferHardware:    req.PreferHardware,
			HardwareType:      string(req.HardwareType),
			// EnableAbr:         req.EnableABR, // TODO: Uncomment after proto regeneration
			SeekNs:            int64(req.Seek), // Convert time.Duration to nanoseconds
			ExtraOptions:      map[string]string{
				"enable_abr": fmt.Sprintf("%t", req.EnableABR), // Pass ABR flag via extra options
			},
		},
	}
	
	// Handle resolution if provided
	if req.Resolution != nil {
		protoReq.Request.Resolution = fmt.Sprintf("%dx%d", req.Resolution.Width, req.Resolution.Height)
	}
	
	logger.Info("Sending gRPC StartTranscode request",
		"plugin_id", p.pluginID,
		"proto_request", protoReq.Request)
	
	// Make gRPC call
	resp, err := client.StartTranscode(ctx, protoReq)
	if err != nil {
		logger.Error("gRPC StartTranscode failed",
			"plugin_id", p.pluginID,
			"error", err.Error())
		return nil, fmt.Errorf("gRPC StartTranscode failed: %w", err)
	}
	
	if resp.Error != "" {
		logger.Error("plugin returned error",
			"plugin_id", p.pluginID,
			"error", resp.Error)
		return nil, fmt.Errorf("plugin returned error: %s", resp.Error)
	}
	
	if resp.Handle == nil {
		logger.Error("plugin returned nil handle",
			"plugin_id", p.pluginID)
		return nil, fmt.Errorf("plugin returned nil handle")
	}
	
	logger.Info("gRPC StartTranscode successful",
		"plugin_id", p.pluginID,
		"handle_session_id", resp.Handle.SessionId,
		"handle_directory", resp.Handle.Directory)
	
	// Convert proto handle to SDK handle
	handle := &plugins.TranscodeHandle{
		SessionID:   resp.Handle.SessionId,
		Provider:    resp.Handle.Provider,
		StartTime:   time.Unix(0, resp.Handle.StartTime),
		Directory:   resp.Handle.Directory,
		Context:     ctx,
		PrivateData: resp.Handle.PrivateData,
	}
	
	return handle, nil
}

// GetProgress returns the progress of a transcoding job
func (p *ExternalTranscodingProvider) GetProgress(handle *plugins.TranscodeHandle) (*plugins.TranscodingProgress, error) {
	// Create gRPC client
	client := proto.NewTranscodingProviderServiceClient(p.client.conn)
	
	// Convert SDK handle to proto handle
	protoHandle := &proto.TranscodeHandle{
		SessionId:   handle.SessionID,
		Provider:    handle.Provider,
		StartTime:   handle.StartTime.UnixNano(),
		Directory:   handle.Directory,
		PrivateData: fmt.Sprintf("%v", handle.PrivateData),
	}
	
	// Make gRPC call
	resp, err := client.GetProgress(context.Background(), &proto.GetProgressRequest{
		Handle: protoHandle,
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC GetProgress failed: %w", err)
	}
	
	if resp.Error != "" {
		return nil, fmt.Errorf("plugin returned error: %s", resp.Error)
	}
	
	if resp.Progress == nil {
		return nil, fmt.Errorf("plugin returned nil progress")
	}
	
	// Convert proto progress to SDK progress
	progress := &plugins.TranscodingProgress{
		PercentComplete: float64(resp.Progress.PercentComplete),
		TimeElapsed:     time.Duration(resp.Progress.TimeElapsed),
		TimeRemaining:   time.Duration(resp.Progress.TimeRemaining),
		CurrentSpeed:    resp.Progress.CurrentSpeed,
		BytesRead:       resp.Progress.BytesRead,
		BytesWritten:    resp.Progress.BytesWritten,
	}
	
	return progress, nil
}

// StopTranscode stops a transcoding job
func (p *ExternalTranscodingProvider) StopTranscode(handle *plugins.TranscodeHandle) error {
	// Create gRPC client
	client := proto.NewTranscodingProviderServiceClient(p.client.conn)
	
	// Convert SDK handle to proto handle
	protoHandle := &proto.TranscodeHandle{
		SessionId:   handle.SessionID,
		Provider:    handle.Provider,
		StartTime:   handle.StartTime.UnixNano(),
		Directory:   handle.Directory,
		PrivateData: fmt.Sprintf("%v", handle.PrivateData),
	}
	
	// Make gRPC call
	resp, err := client.StopTranscode(context.Background(), &proto.StopTranscodeProviderRequest{
		Handle: protoHandle,
	})
	if err != nil {
		return fmt.Errorf("gRPC StopTranscode failed: %w", err)
	}
	
	if resp.Error != "" {
		return fmt.Errorf("plugin returned error: %s", resp.Error)
	}
	
	if !resp.Success {
		return fmt.Errorf("plugin reported failure to stop transcoding")
	}
	
	return nil
}

// StartStream starts a streaming transcode operation
func (p *ExternalTranscodingProvider) StartStream(ctx context.Context, req plugins.TranscodeRequest) (*plugins.StreamHandle, error) {
	// Create gRPC client
	client := proto.NewTranscodingProviderServiceClient(p.client.conn)
	
	// Convert SDK request to proto request
	protoReq := &proto.StartStreamRequest{
		Request: &proto.TranscodeProviderRequest{
			SessionId:         req.SessionID,
			InputPath:         req.InputPath,
			OutputDir:         req.OutputPath, // Use OutputPath as OutputDir
			Quality:           int32(req.Quality),
			SpeedPriority:     string(req.SpeedPriority),
			Container:         req.Container,
			VideoCodec:        req.VideoCodec,
			AudioCodec:        req.AudioCodec,
			PreferHardware:    req.PreferHardware,
			HardwareType:      string(req.HardwareType),
			SeekNs:            int64(req.Seek), // Convert time.Duration to nanoseconds
			ExtraOptions:      make(map[string]string), // Empty for now
		},
	}
	
	// Handle resolution if provided
	if req.Resolution != nil {
		protoReq.Request.Resolution = fmt.Sprintf("%dx%d", req.Resolution.Width, req.Resolution.Height)
	}
	
	// Make gRPC call
	resp, err := client.StartStream(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("gRPC StartStream failed: %w", err)
	}
	
	if resp.Error != "" {
		return nil, fmt.Errorf("plugin returned error: %s", resp.Error)
	}
	
	if resp.Handle == nil {
		return nil, fmt.Errorf("plugin returned nil handle")
	}
	
	// Convert proto handle to SDK handle
	handle := &plugins.StreamHandle{
		SessionID:   resp.Handle.SessionId,
		Provider:    resp.Handle.Provider,
		StartTime:   time.Unix(0, resp.Handle.StartTime),
		PrivateData: resp.Handle.PrivateData,
	}
	
	return handle, nil
}

// GetStream returns the stream reader
func (p *ExternalTranscodingProvider) GetStream(handle *plugins.StreamHandle) (io.ReadCloser, error) {
	// Create gRPC client
	client := proto.NewTranscodingProviderServiceClient(p.client.conn)
	
	// Convert SDK handle to proto handle
	protoHandle := &proto.StreamHandle{
		SessionId:   handle.SessionID,
		Provider:    handle.Provider,
		StartTime:   handle.StartTime.UnixNano(),
		PrivateData: fmt.Sprintf("%v", handle.PrivateData),
	}
	
	// Start streaming
	stream, err := client.GetStreamData(context.Background(), &proto.GetStreamDataRequest{
		Handle: protoHandle,
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC GetStreamData failed: %w", err)
	}
	
	// Return a stream reader wrapper
	return &grpcStreamReader{stream: stream}, nil
}

// StopStream stops a streaming operation
func (p *ExternalTranscodingProvider) StopStream(handle *plugins.StreamHandle) error {
	// Create gRPC client
	client := proto.NewTranscodingProviderServiceClient(p.client.conn)
	
	// Convert SDK handle to proto handle
	protoHandle := &proto.StreamHandle{
		SessionId:   handle.SessionID,
		Provider:    handle.Provider,
		StartTime:   handle.StartTime.UnixNano(),
		PrivateData: fmt.Sprintf("%v", handle.PrivateData),
	}
	
	// Make gRPC call
	resp, err := client.StopStream(context.Background(), &proto.StopStreamRequest{
		Handle: protoHandle,
	})
	if err != nil {
		return fmt.Errorf("gRPC StopStream failed: %w", err)
	}
	
	if resp.Error != "" {
		return fmt.Errorf("plugin returned error: %s", resp.Error)
	}
	
	if !resp.Success {
		return fmt.Errorf("plugin reported failure to stop stream")
	}
	
	return nil
}

// GetDashboardSections returns dashboard sections
func (p *ExternalTranscodingProvider) GetDashboardSections() []plugins.DashboardSection {
	// Delegate to the adapter's implementation
	return []plugins.DashboardSection{
		{
			ID:          p.pluginID + "_main",
			PluginID:    p.pluginID,
			Type:        "transcoder",
			Title:       p.pluginInfo.Name,
			Description: p.pluginInfo.Description,
			Icon:        "video",
			Priority:    100,
		},
	}
}

// GetDashboardData returns dashboard data
func (p *ExternalTranscodingProvider) GetDashboardData(sectionID string) (interface{}, error) {
	// TODO: Implement GRPC call
	return nil, fmt.Errorf("dashboard data not yet implemented")
}

// ExecuteDashboardAction executes a dashboard action
func (p *ExternalTranscodingProvider) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	// TODO: Implement GRPC call
	return fmt.Errorf("dashboard actions not yet implemented")
}
