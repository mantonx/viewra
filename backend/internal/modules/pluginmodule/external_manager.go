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
	"github.com/mantonx/viewra/pkg/plugins"
	"github.com/mantonx/viewra/pkg/plugins/proto"
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

// TranscodingService - return a basic implementation for ffmpeg_transcoder
func (a *ExternalPluginAdapter) TranscodingService() plugins.TranscodingService {
	if a.pluginInfo != nil && a.pluginInfo.Type == "transcoder" {
		return &BasicTranscodingService{client: a.client}
	}
	return nil
}

// BasicTranscodingService implements GRPC communication with external transcoding plugins
type BasicTranscodingService struct {
	client *ExternalPluginGRPCClient
}

// GetCapabilities returns transcoding capabilities via GRPC
func (s *BasicTranscodingService) GetCapabilities(ctx context.Context) (*plugins.TranscodingCapabilities, error) {
	// Create GRPC client for transcoding service
	transcodingClient := proto.NewTranscodingServiceClient(s.client.conn)

	fmt.Printf("DEBUG: BasicTranscodingService.GetCapabilities calling GRPC\n")
	resp, err := transcodingClient.GetCapabilities(ctx, &proto.GetCapabilitiesRequest{})
	if err != nil {
		fmt.Printf("DEBUG: BasicTranscodingService.GetCapabilities GRPC failed: %v\n", err)
		return nil, fmt.Errorf("failed to get capabilities: %w", err)
	}
	fmt.Printf("DEBUG: BasicTranscodingService.GetCapabilities GRPC succeeded\n")

	if resp.Error != "" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	// DEBUG: Log the raw protobuf response
	fmt.Printf("DEBUG: GRPC response - Name: %s\n", resp.Capabilities.Name)
	fmt.Printf("DEBUG: GRPC response - SupportedCodecs: %v\n", resp.Capabilities.SupportedCodecs)
	fmt.Printf("DEBUG: GRPC response - SupportedResolutions: %v\n", resp.Capabilities.SupportedResolutions)
	fmt.Printf("DEBUG: GRPC response - SupportedContainers: %v\n", resp.Capabilities.SupportedContainers)
	fmt.Printf("DEBUG: GRPC response - Priority: %d\n", resp.Capabilities.Priority)

	// Convert protobuf response to internal format
	capabilities := &plugins.TranscodingCapabilities{
		Name:                  resp.Capabilities.Name,
		SupportedCodecs:       resp.Capabilities.SupportedCodecs,
		SupportedResolutions:  resp.Capabilities.SupportedResolutions,
		SupportedContainers:   resp.Capabilities.SupportedContainers,
		HardwareAcceleration:  resp.Capabilities.HardwareAcceleration,
		MaxConcurrentSessions: int(resp.Capabilities.MaxConcurrentSessions),
		Priority:              int(resp.Capabilities.Priority),
		Features: plugins.TranscodingFeatures{
			SubtitleBurnIn:      resp.Capabilities.Features.SubtitleBurnIn,
			SubtitlePassthrough: resp.Capabilities.Features.SubtitlePassthrough,
			MultiAudioTracks:    resp.Capabilities.Features.MultiAudioTracks,
			HDRSupport:          resp.Capabilities.Features.HdrSupport,
			ToneMapping:         resp.Capabilities.Features.ToneMapping,
			StreamingOutput:     resp.Capabilities.Features.StreamingOutput,
			SegmentedOutput:     resp.Capabilities.Features.SegmentedOutput,
		},
	}

	// DEBUG: Log the converted internal capabilities
	fmt.Printf("DEBUG: Converted capabilities - Name: %s\n", capabilities.Name)
	fmt.Printf("DEBUG: Converted capabilities - SupportedCodecs: %v\n", capabilities.SupportedCodecs)
	fmt.Printf("DEBUG: Converted capabilities - SupportedResolutions: %v\n", capabilities.SupportedResolutions)
	fmt.Printf("DEBUG: Converted capabilities - SupportedContainers: %v\n", capabilities.SupportedContainers)
	fmt.Printf("DEBUG: Converted capabilities - Priority: %d\n", capabilities.Priority)

	return capabilities, nil
}

// StartTranscode starts a transcoding session via GRPC
func (s *BasicTranscodingService) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error) {
	// Create GRPC client for transcoding service
	transcodingClient := proto.NewTranscodingServiceClient(s.client.conn)

	// Convert internal request to protobuf
	protoReq := &proto.StartTranscodeRequest{
		Request: &proto.TranscodeRequest{
			InputPath:       req.InputPath,
			TargetCodec:     req.TargetCodec,
			TargetContainer: req.TargetContainer,
			Resolution:      req.Resolution,
			Bitrate:         int32(req.Bitrate),
			AudioCodec:      req.AudioCodec,
			AudioBitrate:    int32(req.AudioBitrate),
			AudioStream:     int32(req.AudioStream),
			Quality:         int32(req.Quality),
			Preset:          req.Preset,
			Options:         req.Options,
			Priority:        int32(req.Priority),
		},
	}

	// Handle subtitles if present
	if req.Subtitles != nil {
		protoReq.Request.Subtitles = &proto.SubtitleConfig{
			Enabled:   req.Subtitles.Enabled,
			Language:  req.Subtitles.Language,
			BurnIn:    req.Subtitles.BurnIn,
			StreamIdx: int32(req.Subtitles.StreamIdx),
			FontSize:  int32(req.Subtitles.FontSize),
			FontColor: req.Subtitles.FontColor,
		}
	}

	// Handle device profile if present
	if req.DeviceProfile != nil {
		protoReq.Request.DeviceProfile = &proto.DeviceProfile{
			UserAgent:       req.DeviceProfile.UserAgent,
			SupportedCodecs: req.DeviceProfile.SupportedCodecs,
			MaxResolution:   req.DeviceProfile.MaxResolution,
			MaxBitrate:      int32(req.DeviceProfile.MaxBitrate),
			SupportsHevc:    req.DeviceProfile.SupportsHEVC,
			SupportsAv1:     req.DeviceProfile.SupportsAV1,
			SupportsHdr:     req.DeviceProfile.SupportsHDR,
			ClientIp:        req.DeviceProfile.ClientIP,
			Platform:        req.DeviceProfile.Platform,
			Browser:         req.DeviceProfile.Browser,
		}
	}

	fmt.Printf("DEBUG: BasicTranscodingService.StartTranscode calling GRPC with InputPath='%s', TargetCodec='%s', Resolution='%s'\n",
		protoReq.Request.InputPath, protoReq.Request.TargetCodec, protoReq.Request.Resolution)
	resp, err := transcodingClient.StartTranscode(ctx, protoReq)
	if err != nil {
		fmt.Printf("DEBUG: BasicTranscodingService.StartTranscode GRPC failed: %v\n", err)
		return nil, fmt.Errorf("failed to start transcode: %w", err)
	}
	fmt.Printf("DEBUG: BasicTranscodingService.StartTranscode GRPC succeeded\n")

	if resp.Error != "" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	// Convert protobuf session to internal format
	session := s.convertSessionFromProto(resp.Session)
	return session, nil
}

// GetTranscodeSession gets transcoding session info via GRPC
func (s *BasicTranscodingService) GetTranscodeSession(ctx context.Context, sessionID string) (*plugins.TranscodeSession, error) {
	transcodingClient := proto.NewTranscodingServiceClient(s.client.conn)

	resp, err := transcodingClient.GetTranscodeSession(ctx, &proto.GetTranscodeSessionRequest{
		SessionId: sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get transcode session: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	return s.convertSessionFromProto(resp.Session), nil
}

// StopTranscode stops a transcoding session via GRPC
func (s *BasicTranscodingService) StopTranscode(ctx context.Context, sessionID string) error {
	transcodingClient := proto.NewTranscodingServiceClient(s.client.conn)

	resp, err := transcodingClient.StopTranscode(ctx, &proto.StopTranscodeRequest{
		SessionId: sessionID,
	})
	if err != nil {
		return fmt.Errorf("failed to stop transcode: %w", err)
	}

	if resp.Error != "" {
		return fmt.Errorf("plugin error: %s", resp.Error)
	}

	return nil
}

// ListActiveSessions lists active transcoding sessions via GRPC
func (s *BasicTranscodingService) ListActiveSessions(ctx context.Context) ([]*plugins.TranscodeSession, error) {
	transcodingClient := proto.NewTranscodingServiceClient(s.client.conn)

	resp, err := transcodingClient.ListActiveSessions(ctx, &proto.ListActiveSessionsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list active sessions: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	var sessions []*plugins.TranscodeSession
	for _, protoSession := range resp.Sessions {
		sessions = append(sessions, s.convertSessionFromProto(protoSession))
	}

	return sessions, nil
}

// GetTranscodeStream gets the transcoded stream via GRPC
func (s *BasicTranscodingService) GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error) {
	transcodingClient := proto.NewTranscodingServiceClient(s.client.conn)

	stream, err := transcodingClient.GetTranscodeStream(ctx, &proto.GetTranscodeStreamRequest{
		SessionId: sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get transcode stream: %w", err)
	}

	return &grpcStreamReader{stream: stream}, nil
}

// Helper method to convert protobuf session to internal format
func (s *BasicTranscodingService) convertSessionFromProto(protoSession *proto.TranscodeSession) *plugins.TranscodeSession {
	session := &plugins.TranscodeSession{
		ID:        protoSession.Id,
		Status:    plugins.TranscodeStatus(protoSession.Status),
		Progress:  protoSession.Progress,
		StartTime: time.Unix(protoSession.StartTime, 0),
		Backend:   protoSession.Backend,
		Error:     protoSession.Error,
		Metadata:  make(map[string]interface{}),
	}

	// Convert metadata
	for k, v := range protoSession.Metadata {
		session.Metadata[k] = v
	}

	if protoSession.EndTime > 0 {
		endTime := time.Unix(protoSession.EndTime, 0)
		session.EndTime = &endTime
	}

	if protoSession.Request != nil {
		session.Request = &plugins.TranscodeRequest{
			InputPath:       protoSession.Request.InputPath,
			TargetCodec:     protoSession.Request.TargetCodec,
			TargetContainer: protoSession.Request.TargetContainer,
			Resolution:      protoSession.Request.Resolution,
			Bitrate:         int(protoSession.Request.Bitrate),
			AudioCodec:      protoSession.Request.AudioCodec,
			AudioBitrate:    int(protoSession.Request.AudioBitrate),
			AudioStream:     int(protoSession.Request.AudioStream),
			Quality:         int(protoSession.Request.Quality),
			Preset:          protoSession.Request.Preset,
			Options:         protoSession.Request.Options,
			Priority:        int(protoSession.Request.Priority),
		}

		if protoSession.Request.Subtitles != nil {
			session.Request.Subtitles = &plugins.SubtitleConfig{
				Enabled:   protoSession.Request.Subtitles.Enabled,
				Language:  protoSession.Request.Subtitles.Language,
				BurnIn:    protoSession.Request.Subtitles.BurnIn,
				StreamIdx: int(protoSession.Request.Subtitles.StreamIdx),
				FontSize:  int(protoSession.Request.Subtitles.FontSize),
				FontColor: protoSession.Request.Subtitles.FontColor,
			}
		}

		if protoSession.Request.DeviceProfile != nil {
			session.Request.DeviceProfile = &plugins.DeviceProfile{
				UserAgent:       protoSession.Request.DeviceProfile.UserAgent,
				SupportedCodecs: protoSession.Request.DeviceProfile.SupportedCodecs,
				MaxResolution:   protoSession.Request.DeviceProfile.MaxResolution,
				MaxBitrate:      int(protoSession.Request.DeviceProfile.MaxBitrate),
				SupportsHEVC:    protoSession.Request.DeviceProfile.SupportsHevc,
				SupportsAV1:     protoSession.Request.DeviceProfile.SupportsAv1,
				SupportsHDR:     protoSession.Request.DeviceProfile.SupportsHdr,
				ClientIP:        protoSession.Request.DeviceProfile.ClientIp,
				Platform:        protoSession.Request.DeviceProfile.Platform,
				Browser:         protoSession.Request.DeviceProfile.Browser,
			}
		}
	}

	if protoSession.Stats != nil {
		session.Stats = &plugins.TranscodeStats{
			Duration:        time.Duration(protoSession.Stats.Duration),
			BytesProcessed:  protoSession.Stats.BytesProcessed,
			BytesGenerated:  protoSession.Stats.BytesGenerated,
			FramesProcessed: protoSession.Stats.FramesProcessed,
			CurrentFPS:      protoSession.Stats.CurrentFps,
			AverageFPS:      protoSession.Stats.AverageFps,
			CPUUsage:        protoSession.Stats.CpuUsage,
			MemoryUsage:     protoSession.Stats.MemoryUsage,
			Speed:           protoSession.Stats.Speed,
		}
	}

	return session
}

// grpcStreamReader implements io.ReadCloser for GRPC streaming
type grpcStreamReader struct {
	stream   grpc.ServerStreamingClient[proto.TranscodeStreamChunk]
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

		// Check if plugin.cue exists
		if _, err := os.Stat(pluginCuePath); os.IsNotExist(err) {
			m.logger.Debug("skipping directory without plugin.cue", "dir", entry.Name())
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
	inSettingsBlock := false
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

		// Handle block endings first
		blockDepth -= closeBraces
		if blockDepth <= 0 {
			inSettingsBlock = false
		}

		// Check for plugin block start - for CUE compatibility we don't need to track this
		if (strings.Contains(line, "plugin:") || strings.Contains(line, "#Plugin:")) && strings.Contains(line, "{") {
			blockDepth = 1
			continue
		}

		// Check for settings block (for future expansion)
		if strings.Contains(line, "settings:") && strings.Contains(line, "{") {
			inSettingsBlock = true
			blockDepth = 1
			continue
		}

		// Add opening braces to depth
		blockDepth += openBraces

		// Parse lines inside plugin block or at root level (allow all lines for CUE compatibility)
		// Skip only settings block parsing since that's not needed for basic manifest info
		if inSettingsBlock {
			continue
		}

		// Parse basic fields (look for these anywhere, not just in settings block)
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
			// Special handling for type field to handle CUE constraints
			typeValue := m.extractQuotedValue(line)
			// Clean up CUE constraint syntax (e.g., "string | *\"none\"" -> "transcoder")
			if typeValue == "" {
				// Try to extract from CUE constraint syntax
				rest := strings.TrimSpace(line[strings.Index(line, ":")+1:])
				if strings.Contains(rest, "\"transcoder\"") {
					typeValue = "transcoder"
				} else if strings.Contains(rest, "\"metadata_scraper\"") {
					typeValue = "metadata_scraper"
				} else if strings.Contains(rest, "\"scanner_hook\"") {
					typeValue = "scanner_hook"
				}
			}
			manifest.Type = typeValue
		} else if strings.Contains(line, "enabled_by_default:") {
			manifest.EnabledDefault = strings.Contains(line, "true")
		} else if strings.Contains(line, "main:") && strings.Contains(line, "entry_points") {
			// Skip for now - we'll extract from context
		} else if strings.Contains(line, "main:") {
			manifest.EntryPoints["main"] = m.extractQuotedValue(line)
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
			// 3. Binary exists for both cases
			shouldEnable := false
			if cfg.RespectDefaultConfig && manifest.EnabledDefault {
				shouldEnable = true
				m.logger.Info("enabling plugin due to enabled_by_default", "plugin", manifest.ID)
			} else if cfg.EnrichmentEnabled && isEnrichmentPlugin {
				shouldEnable = true
				m.logger.Info("enabling enrichment plugin due to global config", "plugin", manifest.ID)
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
