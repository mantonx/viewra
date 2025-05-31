//go:build ignore
// +build ignore

// This file contains placeholder gRPC server implementation.
// Remove the build ignore tag above after generating protobuf code with:
// ./scripts/generate-proto.sh

package enrichmentmodule

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// =============================================================================
// PLACEHOLDER TYPES - REPLACE WITH GENERATED PROTOBUF CODE
// =============================================================================
// TODO: After running `./scripts/generate-proto.sh`, replace these placeholder
// types with imports from the generated protobuf package:
// import enrichmentpb "github.com/mantonx/viewra/api/proto/enrichment"

// EnrichmentServiceServer interface - will be replaced by generated protobuf interface
type EnrichmentServiceServer interface {
	RegisterEnrichment(context.Context, *RegisterEnrichmentRequest) (*RegisterEnrichmentResponse, error)
	GetEnrichmentStatus(context.Context, *GetEnrichmentStatusRequest) (*GetEnrichmentStatusResponse, error)
	ListEnrichmentSources(context.Context, *ListEnrichmentSourcesRequest) (*ListEnrichmentSourcesResponse, error)
	UpdateEnrichmentSource(context.Context, *UpdateEnrichmentSourceRequest) (*UpdateEnrichmentSourceResponse, error)
	TriggerEnrichmentJob(context.Context, *TriggerEnrichmentJobRequest) (*TriggerEnrichmentJobResponse, error)
}

// Request/Response types - will be replaced by generated protobuf types
type RegisterEnrichmentRequest struct {
	MediaFileId     string
	SourceName      string
	Enrichments     map[string]string
	ConfidenceScore float64
	MatchMetadata   map[string]string
}

type RegisterEnrichmentResponse struct {
	Success bool
	Message string
	JobId   string
}

type GetEnrichmentStatusRequest struct {
	MediaFileId string
}

type GetEnrichmentStatusResponse struct {
	MediaFileId      string
	TotalEnrichments int32
	AppliedCount     int32
	PendingCount     int32
	Sources          map[string]int32
	Fields           map[string]*FieldEnrichmentStatus
}

type FieldEnrichmentStatus struct {
	Enrichments []*EnrichmentInfo
	BestSource  string
	Applied     bool
}

type EnrichmentInfo struct {
	Source     string
	Value      string
	Confidence float64
	Applied    bool
	Priority   int32
}

type ListEnrichmentSourcesRequest struct{}

type ListEnrichmentSourcesResponse struct {
	Sources []*EnrichmentSourcePB
}

type EnrichmentSourcePB struct {
	Id         uint32
	Name       string
	Priority   int32
	MediaTypes []string
	Enabled    bool
	LastSync   *timestamppb.Timestamp
	CreatedAt  *timestamppb.Timestamp
	UpdatedAt  *timestamppb.Timestamp
}

type UpdateEnrichmentSourceRequest struct {
	SourceName string
	Priority   int32
	Enabled    bool
}

type UpdateEnrichmentSourceResponse struct {
	Success bool
	Message string
	Source  *EnrichmentSourcePB
}

type TriggerEnrichmentJobRequest struct {
	MediaFileId string
}

type TriggerEnrichmentJobResponse struct {
	Success bool
	Message string
	JobId   string
}

// =============================================================================
// GRPC SERVER IMPLEMENTATION
// =============================================================================

// GRPCServer implements the enrichment gRPC service
type GRPCServer struct {
	module *Module
}

// NewGRPCServer creates a new gRPC server for the enrichment module
func NewGRPCServer(module *Module) *GRPCServer {
	return &GRPCServer{
		module: module,
	}
}

// RegisterEnrichment registers enriched metadata for a media file
func (s *GRPCServer) RegisterEnrichment(ctx context.Context, req *RegisterEnrichmentRequest) (*RegisterEnrichmentResponse, error) {
	if req.MediaFileId == "" {
		return nil, grpcstatus.Error(codes.InvalidArgument, "media_file_id is required")
	}

	if req.SourceName == "" {
		return nil, grpcstatus.Error(codes.InvalidArgument, "source_name is required")
	}

	if len(req.Enrichments) == 0 {
		return nil, grpcstatus.Error(codes.InvalidArgument, "enrichments cannot be empty")
	}

	if req.ConfidenceScore < 0.0 || req.ConfidenceScore > 1.0 {
		return nil, grpcstatus.Error(codes.InvalidArgument, "confidence_score must be between 0.0 and 1.0")
	}

	// Convert map[string]string to map[string]interface{}
	enrichments := make(map[string]interface{})
	for key, value := range req.Enrichments {
		enrichments[key] = value
	}

	// Register the enrichment data
	if err := s.module.RegisterEnrichmentData(req.MediaFileId, req.SourceName, enrichments, req.ConfidenceScore); err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to register enrichment: %v", err)
	}

	// Get the created job ID if available
	jobID := "pending" // We could enhance this to return the actual job ID

	return &RegisterEnrichmentResponse{
		Success: true,
		Message: fmt.Sprintf("Enrichment registered successfully for media file %s", req.MediaFileId),
		JobId:   jobID,
	}, nil
}

// GetEnrichmentStatus returns enrichment status for a media file
func (s *GRPCServer) GetEnrichmentStatus(ctx context.Context, req *GetEnrichmentStatusRequest) (*GetEnrichmentStatusResponse, error) {
	if req.MediaFileId == "" {
		return nil, grpcstatus.Error(codes.InvalidArgument, "media_file_id is required")
	}

	statusData, err := s.module.GetEnrichmentStatus(req.MediaFileId)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to get enrichment status: %v", err)
	}

	// Convert to protobuf response
	response := &GetEnrichmentStatusResponse{
		MediaFileId:      req.MediaFileId,
		TotalEnrichments: int32(statusData["total_enrichments"].(int)),
		AppliedCount:     int32(statusData["applied_count"].(int)),
		PendingCount:     int32(statusData["pending_count"].(int)),
		Sources:          make(map[string]int32),
		Fields:           make(map[string]*FieldEnrichmentStatus),
	}

	// Convert sources map
	if sources, ok := statusData["sources"].(map[string]int); ok {
		for source, count := range sources {
			response.Sources[source] = int32(count)
		}
	}

	// Convert fields map
	if fields, ok := statusData["fields"].(map[string]interface{}); ok {
		for fieldName, fieldData := range fields {
			if fieldInfo, ok := fieldData.(map[string]interface{}); ok {
				fieldStatus := &FieldEnrichmentStatus{
					BestSource: fieldInfo["best_source"].(string),
					Applied:    fieldInfo["applied"].(bool),
					Enrichments: []*EnrichmentInfo{},
				}

				// Convert enrichments array
				if enrichments, ok := fieldInfo["enrichments"].([]map[string]interface{}); ok {
					for _, enrichment := range enrichments {
						enrichmentInfo := &EnrichmentInfo{
							Source:     enrichment["source"].(string),
							Value:      enrichment["value"].(string),
							Confidence: enrichment["confidence"].(float64),
							Applied:    enrichment["applied"].(bool),
							Priority:   int32(enrichment["priority"].(int)),
						}
						fieldStatus.Enrichments = append(fieldStatus.Enrichments, enrichmentInfo)
					}
				}

				response.Fields[fieldName] = fieldStatus
			}
		}
	}

	return response, nil
}

// ListEnrichmentSources returns all enrichment sources
func (s *GRPCServer) ListEnrichmentSources(ctx context.Context, req *ListEnrichmentSourcesRequest) (*ListEnrichmentSourcesResponse, error) {
	var sources []EnrichmentSource
	if err := s.module.db.Find(&sources).Error; err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to fetch enrichment sources: %v", err)
	}

	response := &ListEnrichmentSourcesResponse{
		Sources: make([]*EnrichmentSourcePB, len(sources)),
	}

	for i, source := range sources {
		// Parse media types JSON array
		var mediaTypes []string
		if source.MediaTypes != "" {
			if err := json.Unmarshal([]byte(source.MediaTypes), &mediaTypes); err != nil {
				// Fallback to single type if JSON parsing fails
				mediaTypes = []string{strings.Trim(source.MediaTypes, `"`)}
			}
		}

		pbSource := &EnrichmentSourcePB{
			Id:         source.ID,
			Name:       source.Name,
			Priority:   int32(source.Priority),
			MediaTypes: mediaTypes,
			Enabled:    source.Enabled,
			CreatedAt:  timestamppb.New(source.CreatedAt),
			UpdatedAt:  timestamppb.New(source.UpdatedAt),
		}

		if source.LastSync != nil {
			pbSource.LastSync = timestamppb.New(*source.LastSync)
		}

		response.Sources[i] = pbSource
	}

	return response, nil
}

// UpdateEnrichmentSource updates an enrichment source configuration
func (s *GRPCServer) UpdateEnrichmentSource(ctx context.Context, req *UpdateEnrichmentSourceRequest) (*UpdateEnrichmentSourceResponse, error) {
	if req.SourceName == "" {
		return nil, grpcstatus.Error(codes.InvalidArgument, "source_name is required")
	}

	// Find the source
	var source EnrichmentSource
	if err := s.module.db.Where("name = ?", req.SourceName).First(&source).Error; err != nil {
		return nil, grpcstatus.Error(codes.NotFound, "enrichment source not found")
	}

	// Update the source
	source.Priority = int(req.Priority)
	source.Enabled = req.Enabled

	if err := s.module.db.Save(&source).Error; err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to update enrichment source: %v", err)
	}

	// Parse media types for response
	var mediaTypes []string
	if source.MediaTypes != "" {
		if err := json.Unmarshal([]byte(source.MediaTypes), &mediaTypes); err != nil {
			mediaTypes = []string{strings.Trim(source.MediaTypes, `"`)}
		}
	}

	pbSource := &EnrichmentSourcePB{
		Id:         source.ID,
		Name:       source.Name,
		Priority:   int32(source.Priority),
		MediaTypes: mediaTypes,
		Enabled:    source.Enabled,
		CreatedAt:  timestamppb.New(source.CreatedAt),
		UpdatedAt:  timestamppb.New(source.UpdatedAt),
	}

	if source.LastSync != nil {
		pbSource.LastSync = timestamppb.New(*source.LastSync)
	}

	return &UpdateEnrichmentSourceResponse{
		Success: true,
		Message: fmt.Sprintf("Enrichment source %s updated successfully", req.SourceName),
		Source:  pbSource,
	}, nil
}

// TriggerEnrichmentJob manually triggers enrichment application for a media file
func (s *GRPCServer) TriggerEnrichmentJob(ctx context.Context, req *TriggerEnrichmentJobRequest) (*TriggerEnrichmentJobResponse, error) {
	if req.MediaFileId == "" {
		return nil, grpcstatus.Error(codes.InvalidArgument, "media_file_id is required")
	}

	// Create a new enrichment job
	job := EnrichmentJob{
		MediaFileID: req.MediaFileId,
		JobType:     "apply_enrichment",
		Status:      "pending",
	}

	if err := s.module.db.Create(&job).Error; err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to create enrichment job: %v", err)
	}

	return &TriggerEnrichmentJobResponse{
		Success: true,
		Message: fmt.Sprintf("Enrichment job created for media file %s", req.MediaFileId),
		JobId:   strconv.Itoa(int(job.ID)),
	}, nil
} 