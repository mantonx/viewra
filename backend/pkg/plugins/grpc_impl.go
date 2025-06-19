package plugins

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/mantonx/viewra/pkg/plugins/proto"
	"google.golang.org/grpc"
)

// HCLogAdapter adapts hclog.Logger to our Logger interface
type HCLogAdapter struct {
	logger hclog.Logger
}

func (h *HCLogAdapter) Debug(msg string, args ...interface{}) {
	h.logger.Debug(msg, args...)
}

func (h *HCLogAdapter) Info(msg string, args ...interface{}) {
	h.logger.Info(msg, args...)
}

func (h *HCLogAdapter) Warn(msg string, args ...interface{}) {
	h.logger.Warn(msg, args...)
}

func (h *HCLogAdapter) Error(msg string, args ...interface{}) {
	h.logger.Error(msg, args...)
}

func (h *HCLogAdapter) With(args ...interface{}) hclog.Logger {
	return h.logger.With(args...)
}

// Handshake configuration for plugin communication
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "VIEWRA_PLUGIN",
	MagicCookieValue: "viewra_plugin_magic_cookie_v1",
}

// StartPlugin is a helper function for plugin main() functions
func StartPlugin(impl Implementation) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]goplugin.Plugin{
			"plugin": &GRPCPlugin{Impl: impl},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}

// Type conversion functions
func convertPluginContextFromProto(protoCtx *proto.PluginContext) *PluginContext {
	if protoCtx == nil {
		return nil
	}
	return &PluginContext{
		PluginID:        protoCtx.PluginId,
		DatabaseURL:     protoCtx.DatabaseUrl,
		HostServiceAddr: protoCtx.HostServiceAddr,
		LogLevel:        protoCtx.LogLevel,
		BasePath:        protoCtx.BasePath,
		PluginBasePath:  protoCtx.BasePath, // Use BasePath as PluginBasePath until protobuf is updated
		// Note: Logger will need to be set separately as it's not in protobuf
	}
}

func convertPluginInfoToProto(info *PluginInfo) *proto.PluginInfo {
	if info == nil {
		return nil
	}
	return &proto.PluginInfo{
		Id:          info.ID,
		Name:        info.Name,
		Version:     info.Version,
		Type:        info.Type,
		Description: info.Description,
		Author:      info.Author,
	}
}

func convertAdminPageConfigToProto(configs []*AdminPageConfig) []*proto.AdminPageConfig {
	if configs == nil {
		return nil
	}
	result := make([]*proto.AdminPageConfig, len(configs))
	for i, config := range configs {
		if config != nil {
			result[i] = &proto.AdminPageConfig{
				Id:       config.ID,
				Title:    config.Title,
				Path:     config.URL,      // Using URL field from plugin SDK as Path
				Icon:     config.Icon,     // Now properly set from config
				Category: config.Category, // Now properly set
				Url:      config.URL,      // Set URL field as well
				Type:     config.Type,     // Critical: Set the Type field
			}
		}
	}
	return result
}

func convertAPIRouteToProto(routes []*APIRoute) []*proto.APIRoute {
	if routes == nil {
		return nil
	}
	result := make([]*proto.APIRoute, len(routes))
	for i, route := range routes {
		if route != nil {
			result[i] = &proto.APIRoute{
				Method:      route.Method,
				Path:        route.Path,
				Description: route.Description,
			}
		}
	}
	return result
}

func convertSearchResultToProto(results []*SearchResult) []*proto.SearchResult {
	if results == nil {
		return nil
	}
	result := make([]*proto.SearchResult, len(results))
	for i, searchResult := range results {
		if searchResult != nil {
			// Extract artist and album from metadata if available
			artist := ""
			album := ""
			score := 0.0
			if searchResult.Metadata != nil {
				if a, ok := searchResult.Metadata["artist"]; ok {
					artist = a
				}
				if al, ok := searchResult.Metadata["album"]; ok {
					album = al
				}
			}

			result[i] = &proto.SearchResult{
				Id:       searchResult.ID,
				Title:    searchResult.Title,
				Artist:   artist,
				Album:    album,
				Score:    score,
				Metadata: searchResult.Metadata,
			}
		}
	}
	return result
}

// GRPCPlugin implements the HashiCorp go-plugin interface
type GRPCPlugin struct {
	goplugin.Plugin
	Impl Implementation
}

// GRPCServer creates the gRPC server for this plugin
func (p *GRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	// Register core plugin service
	proto.RegisterPluginServiceServer(s, &PluginServer{Impl: p.Impl})

	// Register optional services based on plugin capabilities
	if metadataService := p.Impl.MetadataScraperService(); metadataService != nil {
		proto.RegisterMetadataScraperServiceServer(s, &MetadataScraperServer{Impl: metadataService})
	}

	if scannerService := p.Impl.ScannerHookService(); scannerService != nil {
		proto.RegisterScannerHookServiceServer(s, &ScannerHookServer{Impl: scannerService})
	}

	if dbService := p.Impl.DatabaseService(); dbService != nil {
		proto.RegisterDatabaseServiceServer(s, &DatabaseServer{Impl: dbService})
	}

	if adminService := p.Impl.AdminPageService(); adminService != nil {
		proto.RegisterAdminPageServiceServer(s, &AdminPageServer{Impl: adminService})
	}

	// Register APIRegistrationService if implemented
	if apiRegService := p.Impl.APIRegistrationService(); apiRegService != nil {
		proto.RegisterAPIRegistrationServiceServer(s, &APIRegistrationServer{Impl: apiRegService})
	}

	// Register SearchService if implemented
	if searchService := p.Impl.SearchService(); searchService != nil {
		proto.RegisterSearchServiceServer(s, &SearchServer{Impl: searchService})
	}

	// Register AssetService if implemented
	if assetService := p.Impl.AssetService(); assetService != nil {
		proto.RegisterAssetServiceServer(s, &AssetServer{Impl: assetService})
	}

	// Register TranscodingService if implemented
	if transcodingService := p.Impl.TranscodingService(); transcodingService != nil {
		server := &TranscodingServer{Impl: transcodingService}
		proto.RegisterTranscodingServiceServer(s, server)
	}

	return nil
}

// GRPCClient creates the gRPC client for this plugin
func (p *GRPCPlugin) GRPCClient(ctx context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{
		PluginServiceClient:          proto.NewPluginServiceClient(c),
		MetadataScraperServiceClient: proto.NewMetadataScraperServiceClient(c),
		ScannerHookServiceClient:     proto.NewScannerHookServiceClient(c),
		DatabaseServiceClient:        proto.NewDatabaseServiceClient(c),
		AdminPageServiceClient:       proto.NewAdminPageServiceClient(c),
		APIRegistrationServiceClient: proto.NewAPIRegistrationServiceClient(c),
		SearchServiceClient:          proto.NewSearchServiceClient(c),
		AssetServiceClient:           proto.NewAssetServiceClient(c),
		TranscodingServiceClient:     proto.NewTranscodingServiceClient(c),
	}, nil
}

// GRPCClient represents the client side of the plugin
type GRPCClient struct {
	proto.PluginServiceClient
	proto.MetadataScraperServiceClient
	proto.ScannerHookServiceClient
	proto.DatabaseServiceClient
	proto.AdminPageServiceClient
	proto.APIRegistrationServiceClient
	proto.SearchServiceClient
	proto.AssetServiceClient
	proto.TranscodingServiceClient
}

// Server implementations

// PluginServer implements the core plugin service
type PluginServer struct {
	proto.UnimplementedPluginServiceServer
	Impl Implementation
}

func (s *PluginServer) Initialize(ctx context.Context, req *proto.InitializeRequest) (*proto.InitializeResponse, error) {
	pluginCtx := convertPluginContextFromProto(req.Context)

	// Create a logger for the plugin using hclog
	if pluginCtx != nil && pluginCtx.Logger == nil {
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  pluginCtx.PluginID,
			Level: hclog.LevelFromString(pluginCtx.LogLevel),
		})
		pluginCtx.Logger = &HCLogAdapter{logger: logger}
	}

	err := s.Impl.Initialize(pluginCtx)
	if err != nil {
		return &proto.InitializeResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	return &proto.InitializeResponse{Success: true}, nil
}

func (s *PluginServer) Start(ctx context.Context, req *proto.StartRequest) (*proto.StartResponse, error) {
	err := s.Impl.Start()
	if err != nil {
		return &proto.StartResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	return &proto.StartResponse{Success: true}, nil
}

func (s *PluginServer) Stop(ctx context.Context, req *proto.StopRequest) (*proto.StopResponse, error) {
	err := s.Impl.Stop()
	if err != nil {
		return &proto.StopResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	return &proto.StopResponse{Success: true}, nil
}

func (s *PluginServer) Info(ctx context.Context, req *proto.InfoRequest) (*proto.InfoResponse, error) {
	info, err := s.Impl.Info()
	if err != nil {
		return &proto.InfoResponse{}, err
	}
	return &proto.InfoResponse{Info: convertPluginInfoToProto(info)}, nil
}

func (s *PluginServer) Health(ctx context.Context, req *proto.HealthRequest) (*proto.HealthResponse, error) {
	err := s.Impl.Health()
	if err != nil {
		return &proto.HealthResponse{
			Healthy: false,
			Error:   err.Error(),
		}, nil
	}
	return &proto.HealthResponse{Healthy: true}, nil
}

// MetadataScraperServer implements the metadata scraper service
type MetadataScraperServer struct {
	proto.UnimplementedMetadataScraperServiceServer
	Impl MetadataScraperService
}

func (s *MetadataScraperServer) CanHandle(ctx context.Context, req *proto.CanHandleRequest) (*proto.CanHandleResponse, error) {
	canHandle := s.Impl.CanHandle(req.FilePath, req.MimeType)
	return &proto.CanHandleResponse{CanHandle: canHandle}, nil
}

func (s *MetadataScraperServer) ExtractMetadata(ctx context.Context, req *proto.ExtractMetadataRequest) (*proto.ExtractMetadataResponse, error) {
	metadata, err := s.Impl.ExtractMetadata(req.FilePath)
	if err != nil {
		return &proto.ExtractMetadataResponse{
			Error: err.Error(),
		}, nil
	}
	return &proto.ExtractMetadataResponse{Metadata: metadata}, nil
}

func (s *MetadataScraperServer) GetSupportedTypes(ctx context.Context, req *proto.GetSupportedTypesRequest) (*proto.GetSupportedTypesResponse, error) {
	types := s.Impl.GetSupportedTypes()
	return &proto.GetSupportedTypesResponse{Types: types}, nil
}

// ScannerHookServer implements the scanner hook service
type ScannerHookServer struct {
	proto.UnimplementedScannerHookServiceServer
	Impl ScannerHookService
}

func (s *ScannerHookServer) OnMediaFileScanned(ctx context.Context, req *proto.OnMediaFileScannedRequest) (*proto.OnMediaFileScannedResponse, error) {
	// Pass the UUID string directly to the plugin implementation
	err := s.Impl.OnMediaFileScanned(req.MediaFileId, req.FilePath, req.Metadata)
	if err != nil {
		return &proto.OnMediaFileScannedResponse{}, err
	}
	return &proto.OnMediaFileScannedResponse{}, nil
}

func (s *ScannerHookServer) OnScanStarted(ctx context.Context, req *proto.OnScanStartedRequest) (*proto.OnScanStartedResponse, error) {
	err := s.Impl.OnScanStarted(req.ScanJobId, req.LibraryId, req.LibraryPath)
	if err != nil {
		return &proto.OnScanStartedResponse{}, err
	}
	return &proto.OnScanStartedResponse{}, nil
}

func (s *ScannerHookServer) OnScanCompleted(ctx context.Context, req *proto.OnScanCompletedRequest) (*proto.OnScanCompletedResponse, error) {
	err := s.Impl.OnScanCompleted(req.ScanJobId, req.LibraryId, req.Stats)
	if err != nil {
		return &proto.OnScanCompletedResponse{}, err
	}
	return &proto.OnScanCompletedResponse{}, nil
}

// DatabaseServer implements the database service
type DatabaseServer struct {
	proto.UnimplementedDatabaseServiceServer
	Impl DatabaseService
}

func (s *DatabaseServer) GetModels(ctx context.Context, req *proto.GetModelsRequest) (*proto.GetModelsResponse, error) {
	models := s.Impl.GetModels()
	return &proto.GetModelsResponse{ModelNames: models}, nil
}

func (s *DatabaseServer) Migrate(ctx context.Context, req *proto.MigrateRequest) (*proto.MigrateResponse, error) {
	err := s.Impl.Migrate(req.ConnectionString)
	if err != nil {
		return &proto.MigrateResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	return &proto.MigrateResponse{Success: true}, nil
}

func (s *DatabaseServer) Rollback(ctx context.Context, req *proto.RollbackRequest) (*proto.RollbackResponse, error) {
	err := s.Impl.Rollback(req.ConnectionString)
	if err != nil {
		return &proto.RollbackResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	return &proto.RollbackResponse{Success: true}, nil
}

// AdminPageServer implements the admin page service
type AdminPageServer struct {
	proto.UnimplementedAdminPageServiceServer
	Impl AdminPageService
}

func (s *AdminPageServer) GetAdminPages(ctx context.Context, req *proto.GetAdminPagesRequest) (*proto.GetAdminPagesResponse, error) {
	pages := s.Impl.GetAdminPages()
	return &proto.GetAdminPagesResponse{Pages: convertAdminPageConfigToProto(pages)}, nil
}

func (s *AdminPageServer) RegisterRoutes(ctx context.Context, req *proto.RegisterRoutesRequest) (*proto.RegisterRoutesResponse, error) {
	err := s.Impl.RegisterRoutes(req.BasePath)
	if err != nil {
		return &proto.RegisterRoutesResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	return &proto.RegisterRoutesResponse{Success: true}, nil
}

// APIRegistrationServer implements the APIRegistrationService
type APIRegistrationServer struct {
	proto.UnimplementedAPIRegistrationServiceServer
	Impl APIRegistrationService // Uses the interface defined in types.go
}

func (s *APIRegistrationServer) GetRegisteredRoutes(ctx context.Context, req *proto.GetRegisteredRoutesRequest) (*proto.GetRegisteredRoutesResponse, error) {
	routes, err := s.Impl.GetRegisteredRoutes(ctx)
	if err != nil {
		return &proto.GetRegisteredRoutesResponse{Routes: []*proto.APIRoute{}}, err
	}
	return &proto.GetRegisteredRoutesResponse{Routes: convertAPIRouteToProto(routes)}, nil
}

// APIRegistrationServiceGRPCClient is the gRPC client for APIRegistrationService
type APIRegistrationServiceGRPCClient struct {
	client proto.APIRegistrationServiceClient
	broker *goplugin.GRPCBroker
}

func (m *APIRegistrationServiceGRPCClient) GetRegisteredRoutes(ctx context.Context) ([]*proto.APIRoute, error) {
	resp, err := m.client.GetRegisteredRoutes(ctx, &proto.GetRegisteredRoutesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Routes, nil
}

// SearchServer implements the search service
type SearchServer struct {
	proto.UnimplementedSearchServiceServer
	Impl SearchService
}

func (s *SearchServer) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {
	results, totalCount, hasMore, err := s.Impl.Search(ctx, req.Query, req.Limit, req.Offset)
	if err != nil {
		return &proto.SearchResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &proto.SearchResponse{
		Success:    true,
		Results:    convertSearchResultToProto(results),
		TotalCount: totalCount,
		HasMore:    hasMore,
	}, nil
}

func (s *SearchServer) GetSearchCapabilities(ctx context.Context, req *proto.GetSearchCapabilitiesRequest) (*proto.GetSearchCapabilitiesResponse, error) {
	supportedFields, supportsPagination, maxResults, err := s.Impl.GetSearchCapabilities(ctx)
	if err != nil {
		return &proto.GetSearchCapabilitiesResponse{}, err
	}

	return &proto.GetSearchCapabilitiesResponse{
		SupportedFields:    supportedFields,
		SupportsPagination: supportsPagination,
		MaxResults:         maxResults,
	}, nil
}

// AssetServer implements the asset service for plugins
type AssetServer struct {
	proto.UnimplementedAssetServiceServer
	Impl AssetService
}

func (s *AssetServer) SaveAsset(ctx context.Context, req *proto.SaveAssetRequest) (*proto.SaveAssetResponse, error) {
	// Pass the UUID string directly to the plugin implementation including the pluginID
	assetID, hash, relativePath, err := s.Impl.SaveAsset(
		req.MediaFileId, // Pass string UUID directly
		req.AssetType,
		req.Category,
		req.Subtype,
		req.Data,
		req.MimeType,
		req.SourceUrl,
		req.PluginId, // Pass the plugin ID
		req.Metadata,
	)

	if err != nil {
		return &proto.SaveAssetResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &proto.SaveAssetResponse{
		Success:      true,
		AssetId:      assetID,
		Hash:         hash,
		RelativePath: relativePath,
	}, nil
}

func (s *AssetServer) AssetExists(ctx context.Context, req *proto.AssetExistsRequest) (*proto.AssetExistsResponse, error) {
	// Pass the UUID string directly to the plugin implementation
	exists, assetID, relativePath, err := s.Impl.AssetExists(
		req.MediaFileId, // Pass string UUID directly
		req.AssetType,
		req.Category,
		req.Subtype,
		req.Hash,
	)

	if err != nil {
		return &proto.AssetExistsResponse{}, err
	}

	return &proto.AssetExistsResponse{
		Exists:       exists,
		AssetId:      assetID,
		RelativePath: relativePath,
	}, nil
}

func (s *AssetServer) RemoveAsset(ctx context.Context, req *proto.RemoveAssetRequest) (*proto.RemoveAssetResponse, error) {
	err := s.Impl.RemoveAsset(req.AssetId)
	if err != nil {
		return &proto.RemoveAssetResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &proto.RemoveAssetResponse{Success: true}, nil
}

// TranscodingServer implements the transcoding service
type TranscodingServer struct {
	proto.UnimplementedTranscodingServiceServer
	Impl TranscodingService
}

func (s *TranscodingServer) GetCapabilities(ctx context.Context, req *proto.GetCapabilitiesRequest) (*proto.GetCapabilitiesResponse, error) {
	capabilities, err := s.Impl.GetCapabilities(ctx)
	if err != nil {
		return &proto.GetCapabilitiesResponse{
			Error: err.Error(),
		}, nil
	}

	// Convert to protobuf
	protoCapabilities := &proto.TranscodingCapabilities{
		Name:                  capabilities.Name,
		SupportedCodecs:       capabilities.SupportedCodecs,
		SupportedResolutions:  capabilities.SupportedResolutions,
		SupportedContainers:   capabilities.SupportedContainers,
		HardwareAcceleration:  capabilities.HardwareAcceleration,
		MaxConcurrentSessions: int32(capabilities.MaxConcurrentSessions),
		Priority:              int32(capabilities.Priority),
		Features: &proto.TranscodingFeatures{
			SubtitleBurnIn:      capabilities.Features.SubtitleBurnIn,
			SubtitlePassthrough: capabilities.Features.SubtitlePassthrough,
			MultiAudioTracks:    capabilities.Features.MultiAudioTracks,
			HdrSupport:          capabilities.Features.HDRSupport,
			ToneMapping:         capabilities.Features.ToneMapping,
			StreamingOutput:     capabilities.Features.StreamingOutput,
			SegmentedOutput:     capabilities.Features.SegmentedOutput,
		},
	}

	return &proto.GetCapabilitiesResponse{
		Capabilities: protoCapabilities,
	}, nil
}

func (s *TranscodingServer) StartTranscode(ctx context.Context, req *proto.StartTranscodeRequest) (*proto.StartTranscodeResponse, error) {
	// DEBUG: Log what we received from GRPC
	fmt.Printf("DEBUG: TranscodingServer.StartTranscode received: InputPath='%s', TargetCodec='%s', Resolution='%s'\n",
		req.Request.InputPath, req.Request.TargetCodec, req.Request.Resolution)

	// Convert protobuf request to internal modern request format
	internalReq := &TranscodeRequest{
		InputPath:  req.Request.InputPath,
		OutputPath: "",                    // This will be set by the transcode manager
		SessionID:  req.Request.SessionId, // Pass session ID from GRPC
		CodecOpts: &CodecOptions{
			Video:     req.Request.TargetCodec,
			Audio:     req.Request.AudioCodec,
			Container: req.Request.TargetContainer,
			Bitrate:   fmt.Sprintf("%dk", req.Request.Bitrate),
			Quality:   int(req.Request.Quality),
			Preset:    req.Request.Preset,
		},
		Environment: make(map[string]string),
	}

	// Add additional fields to environment
	if req.Request.Priority > 0 {
		internalReq.Environment["priority"] = fmt.Sprintf("%d", req.Request.Priority)
	}
	if req.Request.Resolution != "" {
		internalReq.Environment["resolution"] = req.Request.Resolution
	}
	if req.Request.AudioBitrate > 0 {
		internalReq.Environment["audio_bitrate"] = fmt.Sprintf("%dk", req.Request.AudioBitrate)
	}
	if req.Request.AudioStream > 0 {
		internalReq.Environment["audio_stream"] = fmt.Sprintf("%d", req.Request.AudioStream)
	}
	if len(req.Request.Options) > 0 {
		for k, v := range req.Request.Options {
			internalReq.Environment[k] = v
		}
	}

	fmt.Printf("DEBUG: Converted internal request: InputPath='%s', VideoCodec='%s', Container='%s'\n",
		internalReq.InputPath, internalReq.CodecOpts.Video, internalReq.CodecOpts.Container)

	// Handle subtitles if present - add to environment
	if req.Request.Subtitles != nil {
		internalReq.Environment["subtitles_enabled"] = fmt.Sprintf("%t", req.Request.Subtitles.Enabled)
		if req.Request.Subtitles.Language != "" {
			internalReq.Environment["subtitles_language"] = req.Request.Subtitles.Language
		}
		internalReq.Environment["subtitles_burn_in"] = fmt.Sprintf("%t", req.Request.Subtitles.BurnIn)
		internalReq.Environment["subtitles_stream_idx"] = fmt.Sprintf("%d", req.Request.Subtitles.StreamIdx)
		if req.Request.Subtitles.FontSize > 0 {
			internalReq.Environment["subtitles_font_size"] = fmt.Sprintf("%d", req.Request.Subtitles.FontSize)
		}
		if req.Request.Subtitles.FontColor != "" {
			internalReq.Environment["subtitles_font_color"] = req.Request.Subtitles.FontColor
		}
	}

	// Handle device profile if present
	if req.Request.DeviceProfile != nil {
		internalReq.DeviceProfile = &DeviceProfile{
			UserAgent:       req.Request.DeviceProfile.UserAgent,
			SupportedCodecs: req.Request.DeviceProfile.SupportedCodecs,
			MaxResolution:   req.Request.DeviceProfile.MaxResolution,
			MaxBitrate:      int(req.Request.DeviceProfile.MaxBitrate),
			SupportsHEVC:    req.Request.DeviceProfile.SupportsHevc,
			SupportsAV1:     req.Request.DeviceProfile.SupportsAv1,
			SupportsHDR:     req.Request.DeviceProfile.SupportsHdr,
			ClientIP:        req.Request.DeviceProfile.ClientIp,
			Platform:        req.Request.DeviceProfile.Platform,
			Browser:         req.Request.DeviceProfile.Browser,
		}
	}

	session, err := s.Impl.StartTranscode(ctx, internalReq)
	if err != nil {
		return &proto.StartTranscodeResponse{
			Error: err.Error(),
		}, nil
	}

	if session == nil {
		return &proto.StartTranscodeResponse{
			Error: "StartTranscode returned nil session",
		}, nil
	}

	// Convert session to protobuf
	protoSession := convertSessionToProto(session)
	if protoSession == nil {
		return &proto.StartTranscodeResponse{
			Error: "Failed to convert session to protobuf",
		}, nil
	}

	return &proto.StartTranscodeResponse{
		Session: protoSession,
	}, nil
}

func (s *TranscodingServer) GetTranscodeSession(ctx context.Context, req *proto.GetTranscodeSessionRequest) (*proto.GetTranscodeSessionResponse, error) {
	session, err := s.Impl.GetTranscodeSession(ctx, req.SessionId)
	if err != nil {
		return &proto.GetTranscodeSessionResponse{
			Error: err.Error(),
		}, nil
	}

	protoSession := convertSessionToProto(session)

	return &proto.GetTranscodeSessionResponse{
		Session: protoSession,
	}, nil
}

func (s *TranscodingServer) StopTranscode(ctx context.Context, req *proto.StopTranscodeRequest) (*proto.StopTranscodeResponse, error) {
	err := s.Impl.StopTranscode(ctx, req.SessionId)
	if err != nil {
		return &proto.StopTranscodeResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &proto.StopTranscodeResponse{
		Success: true,
	}, nil
}

func (s *TranscodingServer) ListActiveSessions(ctx context.Context, req *proto.ListActiveSessionsRequest) (*proto.ListActiveSessionsResponse, error) {
	sessions, err := s.Impl.ListActiveSessions(ctx)
	if err != nil {
		return &proto.ListActiveSessionsResponse{
			Error: err.Error(),
		}, nil
	}

	var protoSessions []*proto.TranscodeSession
	for _, session := range sessions {
		protoSessions = append(protoSessions, convertSessionToProto(session))
	}

	return &proto.ListActiveSessionsResponse{
		Sessions: protoSessions,
	}, nil
}

func (s *TranscodingServer) GetTranscodeStream(req *proto.GetTranscodeStreamRequest, stream proto.TranscodingService_GetTranscodeStreamServer) error {
	reader, err := s.Impl.GetTranscodeStream(context.Background(), req.SessionId)
	if err != nil {
		return stream.Send(&proto.TranscodeStreamChunk{
			Error: err.Error(),
		})
	}
	defer reader.Close()

	buffer := make([]byte, 32*1024) // 32KB chunks
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			chunk := &proto.TranscodeStreamChunk{
				Data: buffer[:n],
			}
			if sendErr := stream.Send(chunk); sendErr != nil {
				return sendErr
			}
		}
		if err != nil {
			if err.Error() != "EOF" {
				return stream.Send(&proto.TranscodeStreamChunk{
					Error: err.Error(),
				})
			}
			// Send EOF marker
			return stream.Send(&proto.TranscodeStreamChunk{
				Eof: true,
			})
		}
	}
}

// Helper function to convert internal session to protobuf
func convertSessionToProto(session *TranscodeSession) *proto.TranscodeSession {
	if session == nil {
		return nil
	}

	protoSession := &proto.TranscodeSession{
		Id:        session.ID,
		Status:    string(session.Status),
		Progress:  session.Progress,
		StartTime: session.StartTime.Unix(),
		Backend:   session.Backend,
		Error:     session.Error,
		Metadata:  make(map[string]string),
	}

	// Convert metadata interface{} to string
	for k, v := range session.Metadata {
		if str, ok := v.(string); ok {
			protoSession.Metadata[k] = str
		}
	}

	if session.EndTime != nil {
		protoSession.EndTime = session.EndTime.Unix()
	}

	if session.Request != nil {
		protoSession.Request = &proto.TranscodeRequest{
			InputPath:       session.Request.InputPath,
			TargetCodec:     session.Request.CodecOpts.Video,
			TargetContainer: session.Request.CodecOpts.Container,
			Resolution:      session.Request.Environment["resolution"],
			Bitrate:         getBitrateFromString(session.Request.CodecOpts.Bitrate),
			AudioCodec:      session.Request.CodecOpts.Audio,
			AudioBitrate:    getBitrateFromString(session.Request.Environment["audio_bitrate"]),
			AudioStream:     getIntFromString(session.Request.Environment["audio_stream"]),
			Quality:         int32(session.Request.CodecOpts.Quality),
			Preset:          session.Request.CodecOpts.Preset,
			Options:         session.Request.Environment, // Pass through environment as options
			Priority:        getIntFromString(session.Request.Environment["priority"]),
		}

		// Handle subtitles from environment
		if session.Request.Environment["subtitles_enabled"] == "true" {
			protoSession.Request.Subtitles = &proto.SubtitleConfig{
				Enabled:   session.Request.Environment["subtitles_enabled"] == "true",
				Language:  session.Request.Environment["subtitles_language"],
				BurnIn:    session.Request.Environment["subtitles_burn_in"] == "true",
				StreamIdx: getIntFromString(session.Request.Environment["subtitles_stream_idx"]),
				FontSize:  getIntFromString(session.Request.Environment["subtitles_font_size"]),
				FontColor: session.Request.Environment["subtitles_font_color"],
			}
		}

		if session.Request.DeviceProfile != nil {
			protoSession.Request.DeviceProfile = &proto.DeviceProfile{
				UserAgent:       session.Request.DeviceProfile.UserAgent,
				SupportedCodecs: session.Request.DeviceProfile.SupportedCodecs,
				MaxResolution:   session.Request.DeviceProfile.MaxResolution,
				MaxBitrate:      int32(session.Request.DeviceProfile.MaxBitrate),
				SupportsHevc:    session.Request.DeviceProfile.SupportsHEVC,
				SupportsAv1:     session.Request.DeviceProfile.SupportsAV1,
				SupportsHdr:     session.Request.DeviceProfile.SupportsHDR,
				ClientIp:        session.Request.DeviceProfile.ClientIP,
				Platform:        session.Request.DeviceProfile.Platform,
				Browser:         session.Request.DeviceProfile.Browser,
			}
		}
	}

	if session.Stats != nil {
		protoSession.Stats = &proto.TranscodeStats{
			Duration:        session.Stats.Duration.Nanoseconds(),
			BytesProcessed:  session.Stats.BytesProcessed,
			BytesGenerated:  session.Stats.BytesGenerated,
			FramesProcessed: session.Stats.FramesProcessed,
			CurrentFps:      session.Stats.CurrentFPS,
			AverageFps:      session.Stats.AverageFPS,
			CpuUsage:        session.Stats.CPUUsage,
			MemoryUsage:     session.Stats.MemoryUsage,
			Speed:           session.Stats.Speed,
		}
	}

	return protoSession
}

// Helper functions for converting string values from environment

func getBitrateFromString(bitrate string) int32 {
	if bitrate == "" {
		return 0
	}
	// Parse bitrate string like "1000k" to int
	var val int
	if _, err := fmt.Sscanf(bitrate, "%dk", &val); err == nil {
		return int32(val)
	}
	return 0
}

func getIntFromString(value string) int32 {
	if value == "" {
		return 0
	}
	var val int
	if _, err := fmt.Sscanf(value, "%d", &val); err == nil {
		return int32(val)
	}
	return 0
}
