package enrichmentmodule

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/modules/assetmodule"
	pluginspb "github.com/mantonx/viewra/pkg/plugins/proto"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// =============================================================================
// ASSET GRPC SERVICE
// =============================================================================
// This service handles asset (artwork, posters, etc.) management via gRPC.
// It runs on the same gRPC server (port 50051) as the EnrichmentService, providing
// a consolidated interface for external plugins to:
// 1. Save/manage assets like artwork (this service)
// 2. Register enrichment metadata (EnrichmentService in grpc_server.go)
//
// Both services are registered in module.go Start() method.

// AssetGRPCServer implements the asset management service for external plugins
type AssetGRPCServer struct {
	pluginspb.UnimplementedAssetServiceServer
	logger hclog.Logger
	config *config.Config
	db     *gorm.DB
}

// NewAssetGRPCServer creates a new asset gRPC server instance
func NewAssetGRPCServer(logger hclog.Logger, config *config.Config, db *gorm.DB) *AssetGRPCServer {
	return &AssetGRPCServer{
		logger: logger.Named("asset-grpc-server"),
		config: config,
		db:     db,
	}
}

// SaveAsset saves an asset file (artwork, etc.) through the proper asset management system
func (s *AssetGRPCServer) SaveAsset(ctx context.Context, req *pluginspb.SaveAssetRequest) (*pluginspb.SaveAssetResponse, error) {
	log.Printf("DEBUG: AssetGRPCServer.SaveAsset called - media_file_id=%s, asset_type=%s, subtype=%s, data_size=%d", 
		req.MediaFileId, req.AssetType, req.Subtype, len(req.Data))
	
	if req.MediaFileId == "" {
		return nil, grpcstatus.Error(codes.InvalidArgument, "media_file_id is required")
	}

	if req.AssetType == "" {
		return nil, grpcstatus.Error(codes.InvalidArgument, "asset_type is required")
	}

	if len(req.Data) == 0 {
		return nil, grpcstatus.Error(codes.InvalidArgument, "asset data is required")
	}

	// Get the asset manager
	assetManager := assetmodule.GetAssetManager()
	if assetManager == nil {
		s.logger.Error("Asset manager not available")
		return &pluginspb.SaveAssetResponse{
			Success: false,
			Error:   "asset manager not available",
		}, nil
	}

	// Find the media file to get the associated album
	var mediaFile struct {
		ID       string
		MediaID  string
		MediaType string
	}
	
	err := s.db.Table("media_files").
		Select("id, media_id, media_type").
		Where("id = ?", req.MediaFileId).
		First(&mediaFile).Error
	
	if err != nil {
		s.logger.Error("Failed to find media file", "media_file_id", req.MediaFileId, "error", err)
		return &pluginspb.SaveAssetResponse{
			Success: false,
			Error:   fmt.Sprintf("media file not found: %v", err),
		}, nil
	}

	// Determine entity type and ID based on asset type and category
	var entityType assetmodule.EntityType
	var entityID uuid.UUID
	var assetType assetmodule.AssetType

	// Parse the media ID as UUID
	if mediaFile.MediaID != "" {
		if parsedID, err := uuid.Parse(mediaFile.MediaID); err == nil {
			entityID = parsedID
		} else {
			s.logger.Error("Invalid media ID format", "media_id", mediaFile.MediaID, "error", err)
			return &pluginspb.SaveAssetResponse{
				Success: false,
				Error:   fmt.Sprintf("invalid media ID format: %v", err),
			}, nil
		}
	} else {
		s.logger.Error("Media file has no associated media entity", "media_file_id", req.MediaFileId)
		return &pluginspb.SaveAssetResponse{
			Success: false,
			Error:   "media file has no associated media entity",
		}, nil
	}

	// Map plugin asset request to asset module types
	switch strings.ToLower(req.Category) {
	case "album":
		entityType = assetmodule.EntityTypeAlbum
		// For music assets, we typically want album artwork
		if mediaFile.MediaType == "track" {
			// Get the track to find the album
			var track struct {
				AlbumID uuid.UUID
			}
			err := s.db.Table("tracks").
				Select("album_id").
				Where("id = ?", mediaFile.MediaID).
				First(&track).Error
			
			if err == nil && track.AlbumID != uuid.Nil {
				entityID = track.AlbumID
			}
		}
	case "artist":
		entityType = assetmodule.EntityTypeArtist
	case "track":
		entityType = assetmodule.EntityTypeTrack
	default:
		// Default to album for music content
		if req.AssetType == "music" {
			entityType = assetmodule.EntityTypeAlbum
			// For music assets, get the album from the track
			if mediaFile.MediaType == "track" {
				var track struct {
					AlbumID uuid.UUID
				}
				err := s.db.Table("tracks").
					Select("album_id").
					Where("id = ?", mediaFile.MediaID).
					First(&track).Error
				
				if err == nil && track.AlbumID != uuid.Nil {
					entityID = track.AlbumID
				}
			}
		} else {
			entityType = assetmodule.EntityType(req.Category)
		}
	}

	// Map asset subtype to AssetType
	switch strings.ToLower(req.Subtype) {
	case "album_front", "front", "cover", "artwork":
		assetType = assetmodule.AssetTypeCover
	case "album_back", "back":
		assetType = assetmodule.AssetTypeCover // Could be a different type if we add back cover support
	case "album_booklet", "booklet":
		assetType = assetmodule.AssetTypeBooklet
	case "album_disc", "disc", "cd":
		assetType = assetmodule.AssetTypeDisc
	case "artist_photo", "photo":
		assetType = assetmodule.AssetTypePhoto
	case "fanart":
		assetType = assetmodule.AssetTypeFanart
	case "banner":
		assetType = assetmodule.AssetTypeBanner
	case "logo":
		assetType = assetmodule.AssetTypeLogo
	case "thumb", "thumbnail":
		assetType = assetmodule.AssetTypeThumb
	default:
		assetType = assetmodule.AssetTypeCover // Default to cover
	}

	// Create asset request for the asset manager
	assetRequest := &assetmodule.AssetRequest{
		EntityType: entityType,
		EntityID:   entityID,
		Type:       assetType,
		Source:     assetmodule.SourcePlugin,
		PluginID:   req.PluginId,
		Data:       req.Data,
		Format:     req.MimeType,
		Preferred:  true, // Mark plugin assets as preferred by default
		Language:   "",   // Could be extracted from metadata if needed
	}

	s.logger.Debug("Saving asset via asset manager", 
		"entity_type", entityType,
		"entity_id", entityID,
		"asset_type", assetType,
		"source", assetmodule.SourcePlugin,
		"plugin_id", req.PluginId,
		"data_size", len(req.Data),
		"mime_type", req.MimeType)

	// Save the asset using the proper asset manager
	response, err := assetManager.SaveAsset(assetRequest)
	if err != nil {
		s.logger.Error("Failed to save asset via asset manager", "error", err)
		return &pluginspb.SaveAssetResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to save asset: %v", err),
		}, nil
	}

	s.logger.Info("Successfully saved asset via asset manager", 
		"asset_id", response.ID,
		"entity_type", response.EntityType,
		"entity_id", response.EntityID,
		"asset_type", response.Type,
		"path", response.Path,
		"format", response.Format,
		"plugin_id", req.PluginId)

	return &pluginspb.SaveAssetResponse{
		Success:      true,
		Error:        "",
		AssetId:      s.uuidToUint32(response.ID), // Convert UUID to uint32 for compatibility
		Hash:         "",                          // Hash is handled internally by asset manager
		RelativePath: response.Path,
	}, nil
}

// uuidToUint32 converts a UUID to uint32 for legacy gRPC compatibility
func (s *AssetGRPCServer) uuidToUint32(id uuid.UUID) uint32 {
	// Create a hash of the UUID and take the first 4 bytes
	hash := md5.Sum(id[:])
	return binary.BigEndian.Uint32(hash[:4])
}

// AssetExists checks if an asset exists for a media file
func (s *AssetGRPCServer) AssetExists(ctx context.Context, req *pluginspb.AssetExistsRequest) (*pluginspb.AssetExistsResponse, error) {
	if req.MediaFileId == "" {
		return nil, grpcstatus.Error(codes.InvalidArgument, "media_file_id is required")
	}

	if req.AssetType == "" {
		return nil, grpcstatus.Error(codes.InvalidArgument, "asset_type is required")
	}

	// Get the asset manager
	assetManager := assetmodule.GetAssetManager()
	if assetManager == nil {
		return &pluginspb.AssetExistsResponse{
			Exists:       false,
			AssetId:      0,
			RelativePath: "",
		}, nil
	}

	// Find the media file to get the associated entity
	var mediaFile struct {
		ID       string
		MediaID  string
		MediaType string
	}
	
	err := s.db.Table("media_files").
		Select("id, media_id, media_type").
		Where("id = ?", req.MediaFileId).
		First(&mediaFile).Error
	
	if err != nil {
		s.logger.Debug("Media file not found for asset existence check", "media_file_id", req.MediaFileId)
		return &pluginspb.AssetExistsResponse{
			Exists:       false,
			AssetId:      0,
			RelativePath: "",
		}, nil
	}

	// Parse the media ID as UUID
	var entityID uuid.UUID
	if mediaFile.MediaID != "" {
		if parsedID, err := uuid.Parse(mediaFile.MediaID); err == nil {
			entityID = parsedID
		} else {
			return &pluginspb.AssetExistsResponse{
				Exists:       false,
				AssetId:      0,
				RelativePath: "",
			}, nil
		}
	}

	// Determine entity type based on request
	var entityType assetmodule.EntityType
	var assetType assetmodule.AssetType

	switch strings.ToLower(req.Category) {
	case "album":
		entityType = assetmodule.EntityTypeAlbum
		if mediaFile.MediaType == "track" {
			// Get the album from the track
			var track struct {
				AlbumID uuid.UUID
			}
			err := s.db.Table("tracks").
				Select("album_id").
				Where("id = ?", mediaFile.MediaID).
				First(&track).Error
			
			if err == nil && track.AlbumID != uuid.Nil {
				entityID = track.AlbumID
			}
		}
	case "artist":
		entityType = assetmodule.EntityTypeArtist
	case "track":
		entityType = assetmodule.EntityTypeTrack
	default:
		entityType = assetmodule.EntityTypeAlbum
	}

	// Map subtype to asset type
	switch strings.ToLower(req.Category) {
	case "front", "cover", "artwork":
		assetType = assetmodule.AssetTypeCover
	default:
		assetType = assetmodule.AssetTypeCover
	}

	// Check if asset exists
	assets, err := assetManager.GetAssetsByEntity(entityType, entityID, &assetmodule.AssetFilter{
		Type: assetType,
	})

	if err != nil || len(assets) == 0 {
		s.logger.Debug("No existing assets found", 
			"entity_type", entityType,
			"entity_id", entityID,
			"asset_type", assetType)
		return &pluginspb.AssetExistsResponse{
			Exists:       false,
			AssetId:      0,
			RelativePath: "",
		}, nil
	}

	// Return the first asset found
	asset := assets[0]
	s.logger.Debug("Found existing asset", 
		"asset_id", asset.ID,
		"path", asset.Path)

	return &pluginspb.AssetExistsResponse{
		Exists:       true,
		AssetId:      s.uuidToUint32(asset.ID),
		RelativePath: asset.Path,
	}, nil
}

// RemoveAsset removes an asset for external plugins via gRPC
func (s *AssetGRPCServer) RemoveAsset(ctx context.Context, req *pluginspb.RemoveAssetRequest) (*pluginspb.RemoveAssetResponse, error) {
	s.logger.Debug("received asset remove request", "asset_id", req.AssetId)

	if req.AssetId == 0 {
		return &pluginspb.RemoveAssetResponse{
			Success: false,
			Error:   "asset_id is required",
		}, nil
	}

	// Get the asset manager
	assetManager := assetmodule.GetAssetManager()
	if assetManager == nil {
		return &pluginspb.RemoveAssetResponse{
			Success: false,
			Error:   "asset manager not available",
		}, nil
	}

	// Convert uint32 to UUID (this is a limitation of the current protobuf definition)
	// For now, we'll look up assets by plugin ID or other means
	s.logger.Warn("Asset removal by uint32 ID not fully supported with UUID-based asset system", "asset_id", req.AssetId)

	return &pluginspb.RemoveAssetResponse{
		Success: false,
		Error:   "asset removal by ID not implemented in UUID-based system",
	}, nil
} 