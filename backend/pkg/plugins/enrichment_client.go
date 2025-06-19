package plugins

import (
	"context"
	"fmt"
)

// StubEnrichmentServiceClient implements EnrichmentServiceClient with stub functionality
type StubEnrichmentServiceClient struct{}

// NewEnrichmentServiceClient creates a new enrichment service client connected to the host
func NewEnrichmentServiceClient(hostServiceAddr string) (EnrichmentServiceClient, error) {
	if hostServiceAddr == "" {
		return nil, fmt.Errorf("host service address is required")
	}

	// Return stub implementation for now
	return &StubEnrichmentServiceClient{}, nil
}

// RegisterEnrichment implements EnrichmentServiceClient.RegisterEnrichment
func (c *StubEnrichmentServiceClient) RegisterEnrichment(ctx context.Context, req *RegisterEnrichmentRequest) (*RegisterEnrichmentResponse, error) {
	// Stub implementation - enrichment functionality not needed for transcoding
	return &RegisterEnrichmentResponse{
		Success: true,
		Message: "Enrichment functionality is not available (stub implementation)",
		JobID:   "stub",
	}, nil
}

// Close implements the close method (no-op for stub)
func (c *StubEnrichmentServiceClient) Close() error {
	return nil
}
