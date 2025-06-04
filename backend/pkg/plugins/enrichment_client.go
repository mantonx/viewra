package plugins

import (
	"context"
	"fmt"
	"time"

	enrichmentpb "github.com/mantonx/viewra/api/proto/enrichment"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCEnrichmentServiceClient implements EnrichmentServiceClient using gRPC
type GRPCEnrichmentServiceClient struct {
	conn   *grpc.ClientConn
	client enrichmentpb.EnrichmentServiceClient
}

// NewEnrichmentServiceClient creates a new enrichment service client connected to the host
func NewEnrichmentServiceClient(hostServiceAddr string) (EnrichmentServiceClient, error) {
	if hostServiceAddr == "" {
		return nil, fmt.Errorf("host service address is required")
	}

	// Create connection context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to host gRPC service
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

	client := enrichmentpb.NewEnrichmentServiceClient(conn)

	return &GRPCEnrichmentServiceClient{
		conn:   conn,
		client: client,
	}, nil
}

// RegisterEnrichment implements EnrichmentServiceClient.RegisterEnrichment
func (c *GRPCEnrichmentServiceClient) RegisterEnrichment(ctx context.Context, req *RegisterEnrichmentRequest) (*RegisterEnrichmentResponse, error) {
	protoReq := &enrichmentpb.RegisterEnrichmentRequest{
		MediaFileId:     req.MediaFileID,
		SourceName:      req.SourceName,
		Enrichments:     req.Enrichments,
		ConfidenceScore: req.ConfidenceScore,
		MatchMetadata:   req.MatchMetadata,
	}

	protoResp, err := c.client.RegisterEnrichment(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	return &RegisterEnrichmentResponse{
		Success: protoResp.Success,
		Message: protoResp.Message,
		JobID:   protoResp.JobId,
	}, nil
}

// Close closes the gRPC connection
func (c *GRPCEnrichmentServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
} 