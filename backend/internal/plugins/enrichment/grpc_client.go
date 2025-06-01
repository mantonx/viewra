//go:build !ignore

// This file contains the gRPC client implementation for external enrichment plugins.
// Protobuf code has been generated successfully.

package enrichment

import (
	"context"
	"fmt"
	"log"
	"time"

	enrichmentpb "github.com/mantonx/viewra/api/proto/enrichment"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// =============================================================================
// GRPC CLIENT FOR EXTERNAL PLUGINS
// =============================================================================
// This client is for external plugins that run as separate processes and need
// to communicate with the core enrichment service via gRPC.
//
// Internal plugins should use the enrichment module directly, not this client.

// EnrichmentClient provides a gRPC client for external enrichment plugins
type EnrichmentClient struct {
	conn   *grpc.ClientConn
	client enrichmentpb.EnrichmentServiceClient
	addr   string
}

// NewEnrichmentClient creates a new enrichment gRPC client
func NewEnrichmentClient(serverAddr string) *EnrichmentClient {
	if serverAddr == "" {
		serverAddr = "localhost:50051" // Default enrichment server address
	}
	
	return &EnrichmentClient{
		addr: serverAddr,
	}
}

// Connect establishes connection to the enrichment service
func (c *EnrichmentClient) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, c.addr, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to enrichment service at %s: %w", c.addr, err)
	}

	c.conn = conn
	c.client = enrichmentpb.NewEnrichmentServiceClient(conn)
	
	log.Printf("INFO: Connected to enrichment service at %s", c.addr)
	return nil
}

// Close closes the connection to the enrichment service
func (c *EnrichmentClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// RegisterEnrichmentData registers enriched metadata with the core system
func (c *EnrichmentClient) RegisterEnrichmentData(mediaFileID, sourceName string, enrichments map[string]string, confidence float64) error {
	if c.conn == nil {
		return fmt.Errorf("client not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &enrichmentpb.RegisterEnrichmentRequest{
		MediaFileId:     mediaFileID,
		SourceName:      sourceName,
		Enrichments:     enrichments,
		ConfidenceScore: confidence,
	}

	resp, err := c.client.RegisterEnrichment(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register enrichment: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("enrichment registration failed: %s", resp.Message)
	}

	log.Printf("INFO: Successfully registered enrichment for media file %s (job: %s)", mediaFileID, resp.JobId)
	return nil
}

// GetEnrichmentStatus gets the enrichment status for a media file
func (c *EnrichmentClient) GetEnrichmentStatus(mediaFileID string) (map[string]interface{}, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("client not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &enrichmentpb.GetEnrichmentStatusRequest{
		MediaFileId: mediaFileID,
	}

	resp, err := c.client.GetEnrichmentStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get enrichment status: %w", err)
	}

	// Convert response to map for easier handling
	status := map[string]interface{}{
		"media_file_id":     resp.MediaFileId,
		"total_enrichments": resp.TotalEnrichments,
		"applied_count":     resp.AppliedCount,
		"pending_count":     resp.PendingCount,
		"sources":           resp.Sources,
		"fields":            resp.Fields,
	}

	return status, nil
}

// TriggerEnrichmentJob manually triggers enrichment application
func (c *EnrichmentClient) TriggerEnrichmentJob(mediaFileID string) (string, error) {
	if c.conn == nil {
		return "", fmt.Errorf("client not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &enrichmentpb.TriggerEnrichmentJobRequest{
		MediaFileId: mediaFileID,
	}

	resp, err := c.client.TriggerEnrichmentJob(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to trigger enrichment job: %w", err)
	}

	if !resp.Success {
		return "", fmt.Errorf("enrichment job trigger failed: %s", resp.Message)
	}

	return resp.JobId, nil
}

// Example usage function for plugins
func ExampleUsage() {
	// Create client
	client := NewEnrichmentClient("localhost:50051")
	
	// Connect to service
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Register enrichment data (example from MusicBrainz plugin)
	enrichments := map[string]string{
		"artist_name": "The Beatles",
		"album_name":  "Abbey Road",
		"release_year": "1969",
		"track_number": "1",
	}
	
	if err := client.RegisterEnrichmentData("media-file-123", "musicbrainz", enrichments, 0.95); err != nil {
		log.Printf("Failed to register enrichment: %v", err)
		return
	}
	
	// Check status
	status, err := client.GetEnrichmentStatus("media-file-123")
	if err != nil {
		log.Printf("Failed to get status: %v", err)
		return
	}
	
	log.Printf("Enrichment status: %+v", status)
	
	// Trigger application
	jobID, err := client.TriggerEnrichmentJob("media-file-123")
	if err != nil {
		log.Printf("Failed to trigger job: %v", err)
		return
	}
	
	log.Printf("Triggered enrichment job: %s", jobID)
} 