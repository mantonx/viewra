package plugins

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/mantonx/viewra/internal/plugins/proto"
	"google.golang.org/grpc"
)

// Handshake configuration for plugin communication
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "VIEWRA_PLUGIN",
	MagicCookieValue: "viewra_plugin_magic_cookie_v1",
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
}

// Server implementations

// PluginServer implements the core plugin service
type PluginServer struct {
	proto.UnimplementedPluginServiceServer
	Impl Implementation
}

func (s *PluginServer) Initialize(ctx context.Context, req *proto.InitializeRequest) (*proto.InitializeResponse, error) {
	err := s.Impl.Initialize(req.Context)
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
	return &proto.InfoResponse{Info: info}, nil
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
	err := s.Impl.OnMediaFileScanned(req.MediaFileId, req.FilePath, req.Metadata)
	if err != nil {
		return &proto.OnMediaFileScannedResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	return &proto.OnMediaFileScannedResponse{Success: true}, nil
}

func (s *ScannerHookServer) OnScanStarted(ctx context.Context, req *proto.OnScanStartedRequest) (*proto.OnScanStartedResponse, error) {
	err := s.Impl.OnScanStarted(req.ScanJobId, req.LibraryId, req.LibraryPath)
	if err != nil {
		return &proto.OnScanStartedResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	return &proto.OnScanStartedResponse{Success: true}, nil
}

func (s *ScannerHookServer) OnScanCompleted(ctx context.Context, req *proto.OnScanCompletedRequest) (*proto.OnScanCompletedResponse, error) {
	err := s.Impl.OnScanCompleted(req.ScanJobId, req.LibraryId, req.Stats)
	if err != nil {
		return &proto.OnScanCompletedResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	return &proto.OnScanCompletedResponse{Success: true}, nil
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
	return &proto.GetAdminPagesResponse{Pages: pages}, nil
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
		// Consider how to represent errors; for now, an empty list if error, or log
		return &proto.GetRegisteredRoutesResponse{Routes: []*proto.APIRoute{}}, err // Or return error directly
	}
	return &proto.GetRegisteredRoutesResponse{Routes: routes}, nil
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
		Results:    results,
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
	assetID, hash, relativePath, err := s.Impl.SaveAsset(
		req.MediaFileId,
		req.AssetType,
		req.Category,
		req.Subtype,
		req.Data,
		req.MimeType,
		req.SourceUrl,
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
	exists, assetID, relativePath, err := s.Impl.AssetExists(
		req.MediaFileId,
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