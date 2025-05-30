package plugins

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/pkg/plugins/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCAssetServiceClient implements AssetServiceClient using gRPC
type GRPCAssetServiceClient struct {
	conn   *grpc.ClientConn
	client proto.AssetServiceClient
}

// NewAssetServiceClient creates a new asset service client connected to the host
func NewAssetServiceClient(hostServiceAddr string) (AssetServiceClient, error) {
	if hostServiceAddr == "" {
		return nil, fmt.Errorf("host service address is required")
	}

	// Connect to host gRPC service
	conn, err := grpc.Dial(hostServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to host service: %w", err)
	}

	client := proto.NewAssetServiceClient(conn)

	return &GRPCAssetServiceClient{
		conn:   conn,
		client: client,
	}, nil
}

// SaveAsset implements AssetServiceClient.SaveAsset
func (c *GRPCAssetServiceClient) SaveAsset(ctx context.Context, req *SaveAssetRequest) (*SaveAssetResponse, error) {
	protoReq := &proto.SaveAssetRequest{
		MediaFileId: req.MediaFileID,
		AssetType:   req.AssetType,
		Category:    req.Category,
		Subtype:     req.Subtype,
		Data:        req.Data,
		MimeType:    req.MimeType,
		SourceUrl:   req.SourceURL,
		Metadata:    req.Metadata,
	}

	protoResp, err := c.client.SaveAsset(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	return &SaveAssetResponse{
		Success:      protoResp.Success,
		Error:        protoResp.Error,
		AssetID:      protoResp.AssetId,
		Hash:         protoResp.Hash,
		RelativePath: protoResp.RelativePath,
	}, nil
}

// AssetExists implements AssetServiceClient.AssetExists
func (c *GRPCAssetServiceClient) AssetExists(ctx context.Context, req *AssetExistsRequest) (*AssetExistsResponse, error) {
	protoReq := &proto.AssetExistsRequest{
		MediaFileId: req.MediaFileID,
		AssetType:   req.AssetType,
		Category:    req.Category,
		Subtype:     req.Subtype,
		Hash:        req.Hash,
	}

	protoResp, err := c.client.AssetExists(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	return &AssetExistsResponse{
		Exists:       protoResp.Exists,
		AssetID:      protoResp.AssetId,
		RelativePath: protoResp.RelativePath,
	}, nil
}

// RemoveAsset implements AssetServiceClient.RemoveAsset
func (c *GRPCAssetServiceClient) RemoveAsset(ctx context.Context, req *RemoveAssetRequest) (*RemoveAssetResponse, error) {
	protoReq := &proto.RemoveAssetRequest{
		AssetId: req.AssetID,
	}

	protoResp, err := c.client.RemoveAsset(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	return &RemoveAssetResponse{
		Success: protoResp.Success,
		Error:   protoResp.Error,
	}, nil
}

// Close closes the gRPC connection
func (c *GRPCAssetServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
} 