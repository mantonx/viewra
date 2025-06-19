package plugins

import (
	"context"
	"fmt"
	"time"

	pluginspb "github.com/mantonx/viewra/pkg/plugins/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UnifiedServiceClient provides both asset and enrichment services from a single connection
type UnifiedServiceClient struct {
	conn        *grpc.ClientConn
	assetClient pluginspb.AssetServiceClient
	// Remove enrichment client for now
	// enrichmentClient  enrichmentpb.EnrichmentServiceClient
}

// NewUnifiedServiceClient creates a single connection that provides both asset and enrichment services
func NewUnifiedServiceClient(hostServiceAddr string) (*UnifiedServiceClient, error) {
	if hostServiceAddr == "" {
		return nil, fmt.Errorf("host service address is required")
	}

	// Create connection context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to host gRPC service (both services run on same server)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(16*1024*1024), // 16MB to match server
			grpc.MaxCallSendMsgSize(16*1024*1024), // 16MB to match server
		),
	}

	conn, err := grpc.DialContext(ctx, hostServiceAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to host service: %w", err)
	}

	return &UnifiedServiceClient{
		conn:        conn,
		assetClient: pluginspb.NewAssetServiceClient(conn),
		// Remove enrichment client initialization
		// enrichmentClient:  enrichmentpb.NewEnrichmentServiceClient(conn),
	}, nil
}

// AssetService returns the asset service client
func (c *UnifiedServiceClient) AssetService() AssetServiceClient {
	return &GRPCAssetServiceClient{
		conn:   c.conn,
		client: c.assetClient,
	}
}

// EnrichmentService returns the enrichment service client (stub implementation)
func (c *UnifiedServiceClient) EnrichmentService() EnrichmentServiceClient {
	// Return a stub implementation for now
	return &StubEnrichmentServiceClient{}
}

// Close closes the unified connection
func (c *UnifiedServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
