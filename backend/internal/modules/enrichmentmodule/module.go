package enrichmentmodule

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	enrichmentpb "github.com/mantonx/viewra/api/proto/enrichment"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// Auto-register the module when imported
func init() {
	Register()
}

const (
	ModuleID   = "system.enrichment"
	ModuleName = "Enrichment Manager"
)

// =============================================================================
// ENRICHMENT MODULE
// =============================================================================
// This module handles centralized metadata enrichment with priority-based
// merging and automatic application to database entities using existing
// MediaEnrichment and MediaExternalIDs tables.

// Module handles enrichment application and metadata merging
type Module struct {
	id         string
	name       string
	core       bool
	db         *gorm.DB
	eventBus   events.EventBus
	enabled    bool
	grpcServer *grpc.Server
	grpcPort   int
	initialized bool
}

// Register registers this module with the module system
func Register() {
	enrichmentModule := &Module{
		id:      ModuleID,
		name:    ModuleName,
		core:    true, // Core module
		enabled: true,
		grpcPort: 50051,
	}
	modulemanager.Register(enrichmentModule)
}

// ID returns the module ID
func (m *Module) ID() string {
	return m.id
}

// Name returns the module name
func (m *Module) Name() string {
	return m.name
}

// Core returns whether this is a core module
func (m *Module) Core() bool {
	return m.core
}

// Migrate handles database schema migrations for enrichment
func (m *Module) Migrate(db *gorm.DB) error {
	m.db = db
	
	// Auto-migrate enrichment tables (MediaEnrichment and MediaExternalIDs already exist)
	if err := m.db.AutoMigrate(
		&EnrichmentSource{},
		&EnrichmentJob{},
	); err != nil {
		return fmt.Errorf("failed to migrate enrichment tables: %w", err)
	}
	
	return nil
}

// Init initializes the enrichment module
func (m *Module) Init() error {
	if m.initialized {
		return nil
	}
	
	// Get dependencies if not already set
	if m.db == nil {
		m.db = database.GetDB()
	}
	if m.eventBus == nil {
		m.eventBus = events.GetGlobalEventBus()
	}
	
	m.initialized = true
	return nil
}

// NewModule creates a new enrichment module
func NewModule(db *gorm.DB, eventBus events.EventBus) *Module {
	return &Module{
		id:       ModuleID,
		name:     ModuleName,
		core:     true,
		db:       db,
		eventBus: eventBus,
		enabled:  true,
		grpcPort: 50051, // Default gRPC port
	}
}

// Start initializes the enrichment module
func (m *Module) Start() error {
	if !m.enabled {
		return nil
	}

	log.Println("INFO: Starting enrichment application module")

	// Dependencies should already be set from Init()
	if m.db == nil {
		m.db = database.GetDB()
	}
	if m.eventBus == nil {
		m.eventBus = events.GetGlobalEventBus()
	}

	// Start gRPC server (disabled until protobuf is generated)
	if err := m.startGRPCServer(); err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	// Start background enrichment application worker
	go m.startEnrichmentWorker()

	log.Println("INFO: Enrichment application module started")
	return nil
}

// startGRPCServer starts the gRPC server for external plugins
func (m *Module) startGRPCServer() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", m.grpcPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", m.grpcPort, err)
	}

	m.grpcServer = grpc.NewServer()
	
	// Register the enrichment service
	grpcService := NewGRPCServer(m)
	enrichmentpb.RegisterEnrichmentServiceServer(m.grpcServer, grpcService)

	// Start server in background
	go func() {
		log.Printf("INFO: Enrichment gRPC server listening on port %d", m.grpcPort)
		if err := m.grpcServer.Serve(listen); err != nil {
			log.Printf("ERROR: gRPC server failed: %v", err)
		}
	}()

	return nil
}

// Stop shuts down the enrichment module
func (m *Module) Stop() error {
	log.Println("INFO: Stopping enrichment application module")
	m.enabled = false
	
	if m.grpcServer != nil {
		m.grpcServer.GracefulStop()
	}
	
	return nil
}

// GetName returns the module name
func (m *Module) GetName() string {
	return "enrichment"
}

// EnrichmentSource represents a source of enriched metadata (minimal table for configuration)
type EnrichmentSource struct {
	ID          uint32    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null;index" json:"name"` // e.g., "musicbrainz", "tmdb"
	Priority    int       `gorm:"not null" json:"priority"`   // Lower number = higher priority
	MediaTypes  string    `gorm:"not null" json:"media_types"` // JSON array of supported media types
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	LastSync    *time.Time `json:"last_sync,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EnrichmentJob tracks batch enrichment application jobs
type EnrichmentJob struct {
	ID          uint32    `gorm:"primaryKey" json:"id"`
	MediaFileID string    `gorm:"not null;index" json:"media_file_id"`
	JobType     string    `gorm:"not null" json:"job_type"` // apply_enrichment, merge_conflicts
	Status      string    `gorm:"not null;default:'pending'" json:"status"` // pending, processing, completed, failed
	Results     string    `gorm:"type:text" json:"results"` // JSON results
	Error       string    `gorm:"type:text" json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EnrichmentData represents structured enrichment data for processing
type EnrichmentData struct {
	Source          string                 `json:"source"`
	SourcePriority  int                    `json:"source_priority"`
	Fields          map[string]interface{} `json:"fields"`
	ExternalIDs     map[string]string      `json:"external_ids"`
	ConfidenceScore float64                `json:"confidence_score"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// MergeStrategy defines how to handle field merging
type MergeStrategy int

const (
	MergeStrategyReplace MergeStrategy = iota
	MergeStrategyMerge   // Union for arrays/lists
	MergeStrategySkip    // For user overrides
)

// FieldRule defines enrichment rules for specific fields
type FieldRule struct {
	FieldName     string
	MediaTypes    []string // track, movie, episode, season, etc.
	SourcePriority []string // Ordered list of preferred sources
	MergeStrategy MergeStrategy
	ValidateFunc  func(value string) bool
	NormalizeFunc func(value string) string
}

// GetFieldRules returns the enrichment rules based on the priority table
func (m *Module) GetFieldRules() map[string]FieldRule {
	return map[string]FieldRule{
		"title": {
			FieldName:     "title",
			MediaTypes:    []string{"track", "movie", "episode"},
			SourcePriority: []string{"tmdb", "musicbrainz", "filename", "embedded"},
			MergeStrategy: MergeStrategyReplace,
			ValidateFunc:  func(value string) bool { return strings.TrimSpace(value) != "" },
			NormalizeFunc: func(value string) string { return strings.TrimSpace(value) },
		},
		"artist_name": {
			FieldName:     "artist_name",
			MediaTypes:    []string{"track"},
			SourcePriority: []string{"musicbrainz", "embedded"},
			MergeStrategy: MergeStrategyReplace,
			ValidateFunc:  func(value string) bool { return strings.TrimSpace(value) != "" && value != "Unknown Artist" },
			NormalizeFunc: func(value string) string { return strings.TrimSpace(value) },
		},
		"album_name": {
			FieldName:     "album_name",
			MediaTypes:    []string{"track"},
			SourcePriority: []string{"musicbrainz", "embedded"},
			MergeStrategy: MergeStrategyReplace,
			ValidateFunc:  func(value string) bool { return strings.TrimSpace(value) != "" && value != "Unknown Album" },
			NormalizeFunc: func(value string) string { return strings.TrimSpace(value) },
		},
		"release_year": {
			FieldName:     "release_year",
			MediaTypes:    []string{"track", "movie", "episode"},
			SourcePriority: []string{"tmdb", "musicbrainz", "filename"},
			MergeStrategy: MergeStrategyReplace,
			ValidateFunc: func(value string) bool {
				if year, err := strconv.Atoi(value); err == nil {
					return year >= 1800 && year <= time.Now().Year()+5
				}
				return false
			},
			NormalizeFunc: func(value string) string { return strings.TrimSpace(value) },
		},
		"genres": {
			FieldName:     "genres",
			MediaTypes:    []string{"track", "movie", "episode"},
			SourcePriority: []string{"tmdb", "musicbrainz", "embedded"},
			MergeStrategy: MergeStrategyMerge,
			ValidateFunc:  func(value string) bool { return strings.TrimSpace(value) != "" },
			NormalizeFunc: func(value string) string { return strings.TrimSpace(value) },
		},
		"duration": {
			FieldName:     "duration",
			MediaTypes:    []string{"track", "movie", "episode"},
			SourcePriority: []string{"embedded", "tmdb", "musicbrainz"},
			MergeStrategy: MergeStrategyReplace,
			ValidateFunc: func(value string) bool {
				if duration, err := strconv.Atoi(value); err == nil {
					return duration > 0 && duration < 86400 // Less than 24 hours
				}
				return false
			},
			NormalizeFunc: func(value string) string { return strings.TrimSpace(value) },
		},
		"track_number": {
			FieldName:     "track_number",
			MediaTypes:    []string{"track"},
			SourcePriority: []string{"embedded", "musicbrainz"},
			MergeStrategy: MergeStrategyReplace,
			ValidateFunc: func(value string) bool {
				if trackNum, err := strconv.Atoi(value); err == nil {
					return trackNum > 0 && trackNum <= 999
				}
				return false
			},
			NormalizeFunc: func(value string) string { return strings.TrimSpace(value) },
		},
	}
}

// RegisterEnrichmentData registers enriched metadata for later application
func (m *Module) RegisterEnrichmentData(mediaFileID, sourceName string, enrichments map[string]interface{}, confidence float64) error {
	if !m.enabled {
		return nil
	}

	// Get media file info
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", mediaFileID).First(&mediaFile).Error; err != nil {
		return fmt.Errorf("media file not found: %w", err)
	}

	// Get or create source priority
	var source EnrichmentSource
	if err := m.db.Where("name = ?", sourceName).First(&source).Error; err != nil {
		// Create source if it doesn't exist
		source = EnrichmentSource{
			Name:       sourceName,
			Priority:   m.getDefaultPriority(sourceName),
			MediaTypes: fmt.Sprintf(`["%s"]`, mediaFile.MediaType),
			Enabled:    true,
		}
		if err := m.db.Create(&source).Error; err != nil {
			return fmt.Errorf("failed to create enrichment source: %w", err)
		}
	}

	// Prepare enrichment data
	enrichmentData := EnrichmentData{
		Source:          sourceName,
		SourcePriority:  source.Priority,
		Fields:          enrichments,
		ExternalIDs:     make(map[string]string),
		ConfidenceScore: confidence,
		UpdatedAt:       time.Now(),
	}

	// Extract external IDs if present
	if externalIDs, ok := enrichments["external_ids"]; ok {
		if idMap, ok := externalIDs.(map[string]string); ok {
			enrichmentData.ExternalIDs = idMap
			delete(enrichmentData.Fields, "external_ids") // Don't store in fields
		}
	}

	// Store in MediaEnrichment table
	payload, err := json.Marshal(enrichmentData)
	if err != nil {
		return fmt.Errorf("failed to marshal enrichment data: %w", err)
	}

	mediaEnrichment := database.MediaEnrichment{
		MediaID:   mediaFile.MediaID,
		MediaType: mediaFile.MediaType,
		Plugin:    sourceName,
		Payload:   string(payload),
		UpdatedAt: time.Now(),
	}

	// Upsert the enrichment data
	if err := m.db.Where("media_id = ? AND media_type = ? AND plugin = ?", 
		mediaFile.MediaID, mediaFile.MediaType, sourceName).
		Save(&mediaEnrichment).Error; err != nil {
		return fmt.Errorf("failed to save media enrichment: %w", err)
	}

	// Store external IDs separately if present
	for idType, idValue := range enrichmentData.ExternalIDs {
		externalID := database.MediaExternalIDs{
			MediaID:    mediaFile.MediaID,
			MediaType:  mediaFile.MediaType,
			Source:     idType, // e.g., "musicbrainz", "tmdb"
			ExternalID: idValue,
			UpdatedAt:  time.Now(),
		}

		// Upsert external ID
		if err := m.db.Where("media_id = ? AND media_type = ? AND source = ?", 
			mediaFile.MediaID, mediaFile.MediaType, idType).
			Save(&externalID).Error; err != nil {
			log.Printf("WARN: Failed to save external ID %s=%s: %v", idType, idValue, err)
		}
	}

	// Queue enrichment application job
	job := EnrichmentJob{
		MediaFileID: mediaFileID,
		JobType:     "apply_enrichment",
		Status:      "pending",
	}
	if err := m.db.Create(&job).Error; err != nil {
		return fmt.Errorf("failed to create enrichment job: %w", err)
	}

	log.Printf("INFO: Registered enrichment data for media file %s from source %s", mediaFileID, sourceName)
	return nil
}

// getDefaultPriority returns default priority for known sources
func (m *Module) getDefaultPriority(sourceName string) int {
	priorities := map[string]int{
		"tmdb":        1,
		"musicbrainz": 2,
		"audiodb":     3,
		"embedded":    4,
		"filename":    5,
	}
	
	if priority, exists := priorities[sourceName]; exists {
		return priority
	}
	return 10 // Default low priority
}

// startEnrichmentWorker runs background job to apply enrichments
func (m *Module) startEnrichmentWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		if !m.enabled {
			return
		}

		select {
		case <-ticker.C:
			m.processEnrichmentJobs()
		}
	}
}

// processEnrichmentJobs processes pending enrichment application jobs
func (m *Module) processEnrichmentJobs() {
	var jobs []EnrichmentJob
	if err := m.db.Where("status = ?", "pending").Limit(10).Find(&jobs).Error; err != nil {
		log.Printf("ERROR: Failed to fetch enrichment jobs: %v", err)
		return
	}

	for _, job := range jobs {
		if err := m.processEnrichmentJob(&job); err != nil {
			log.Printf("ERROR: Failed to process enrichment job %d: %v", job.ID, err)
			
			// Mark job as failed
			job.Status = "failed"
			job.Error = err.Error()
			m.db.Save(&job)
		}
	}
}

// processEnrichmentJob processes a single enrichment application job
func (m *Module) processEnrichmentJob(job *EnrichmentJob) error {
	// Mark as processing
	job.Status = "processing"
	job.UpdatedAt = time.Now()
	if err := m.db.Save(job).Error; err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Get media file info
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", job.MediaFileID).First(&mediaFile).Error; err != nil {
		return fmt.Errorf("media file not found: %w", err)
	}

	// Get all enrichments for this media file
	var enrichments []database.MediaEnrichment
	if err := m.db.Where("media_id = ? AND media_type = ?", mediaFile.MediaID, mediaFile.MediaType).
		Find(&enrichments).Error; err != nil {
		return fmt.Errorf("failed to fetch enrichments: %w", err)
	}

	if len(enrichments) == 0 {
		job.Status = "completed"
		job.Results = `{"message": "no enrichments to apply"}`
		return m.db.Save(job).Error
	}

	// Parse enrichment data and merge by priority
	mergedData, err := m.mergeEnrichmentData(enrichments)
	if err != nil {
		return fmt.Errorf("failed to merge enrichment data: %w", err)
	}

	results := make(map[string]interface{})
	rules := m.GetFieldRules()

	// Apply merged enrichments
	for fieldName, value := range mergedData.Fields {
		rule, exists := rules[fieldName]
		if !exists {
			log.Printf("WARN: No rule found for field %s, skipping", fieldName)
			continue
		}

		// Check if this field supports the media type
		if !m.supportsMediaType(rule.MediaTypes, string(mediaFile.MediaType)) {
			continue
		}

		valueStr := fmt.Sprintf("%v", value)
		
		// Validate the value
		if rule.ValidateFunc != nil && !rule.ValidateFunc(valueStr) {
			log.Printf("WARN: Invalid value for field %s: %s", fieldName, valueStr)
			continue
		}

		// Normalize the value
		if rule.NormalizeFunc != nil {
			valueStr = rule.NormalizeFunc(valueStr)
		}

		// Apply the enrichment
		if err := m.applyFieldToEntity(mediaFile.MediaID, string(mediaFile.MediaType), fieldName, valueStr, rule.MergeStrategy); err != nil {
			log.Printf("ERROR: Failed to apply enrichment for field %s: %v", fieldName, err)
			continue
		}

		results[fieldName] = map[string]interface{}{
			"applied":    true,
			"source":     mergedData.Source,
			"value":      valueStr,
			"confidence": mergedData.ConfidenceScore,
		}
	}

	// Mark job as completed
	job.Status = "completed"
	resultsJSON, _ := json.Marshal(results)
	job.Results = string(resultsJSON)
	job.UpdatedAt = time.Now()

	if err := m.db.Save(job).Error; err != nil {
		return fmt.Errorf("failed to save job results: %w", err)
	}

	// Emit enrichment applied event
	if m.eventBus != nil {
		event := events.NewSystemEvent(
			"enrichment.applied",
			"Enrichment Applied",
			fmt.Sprintf("Applied enrichments to media file %s", job.MediaFileID),
		)
		event.Data = map[string]interface{}{
			"media_file_id": job.MediaFileID,
			"results":       results,
		}
		m.eventBus.PublishAsync(event)
	}

	log.Printf("INFO: Applied enrichments to media file %s", job.MediaFileID)
	return nil
}

// mergeEnrichmentData merges enrichment data from multiple sources by priority
func (m *Module) mergeEnrichmentData(enrichments []database.MediaEnrichment) (*EnrichmentData, error) {
	if len(enrichments) == 0 {
		return nil, fmt.Errorf("no enrichments to merge")
	}

	// Parse and sort by priority
	var enrichmentDataList []EnrichmentData
	for _, enrichment := range enrichments {
		var data EnrichmentData
		if err := json.Unmarshal([]byte(enrichment.Payload), &data); err != nil {
			log.Printf("WARN: Failed to parse enrichment data from %s: %v", enrichment.Plugin, err)
			continue
		}
		enrichmentDataList = append(enrichmentDataList, data)
	}

	// Sort by priority (lower = higher priority)
	// For now, just use the first one - we could implement more sophisticated merging
	if len(enrichmentDataList) == 0 {
		return nil, fmt.Errorf("no valid enrichment data found")
	}

	// TODO: Implement proper priority-based merging
	bestEnrichment := enrichmentDataList[0]
	for _, data := range enrichmentDataList[1:] {
		if data.SourcePriority < bestEnrichment.SourcePriority {
			bestEnrichment = data
		}
	}

	return &bestEnrichment, nil
}

// selectBestEnrichment is replaced by mergeEnrichmentData
// (keeping for compatibility but updating signature)
func (m *Module) selectBestEnrichment(enrichments []database.MediaEnrichment, rule FieldRule) (*EnrichmentData, error) {
	return m.mergeEnrichmentData(enrichments)
}

// applyFieldEnrichment is replaced by applyFieldToEntity
func (m *Module) applyFieldToEntity(entityID, mediaType, fieldName, value string, strategy MergeStrategy) error {
	switch mediaType {
	case "track":
		return m.applyTrackEnrichment(entityID, fieldName, value, strategy)
	case "movie":
		return m.applyMovieEnrichment(entityID, fieldName, value, strategy)
	case "episode":
		return m.applyEpisodeEnrichment(entityID, fieldName, value, strategy)
	default:
		return fmt.Errorf("unsupported media type: %s", mediaType)
	}
}

// supportsMediaType checks if a rule supports the given media type
func (m *Module) supportsMediaType(supportedTypes []string, mediaType string) bool {
	for _, supportedType := range supportedTypes {
		if supportedType == mediaType {
			return true
		}
	}
	return false
}

// applyTrackEnrichment applies enrichment to track entities
func (m *Module) applyTrackEnrichment(trackID, fieldName, value string, strategy MergeStrategy) error {
	switch fieldName {
	case "title":
		return m.db.Model(&database.Track{}).Where("id = ?", trackID).Update("title", value).Error
	
	case "artist_name":
		// Get track to find artist
		var track database.Track
		if err := m.db.Preload("Artist").Where("id = ?", trackID).First(&track).Error; err != nil {
			return fmt.Errorf("track not found: %w", err)
		}
		
		// Update artist name
		return m.db.Model(&database.Artist{}).Where("id = ?", track.ArtistID).Update("name", value).Error
	
	case "album_name":
		// Get track to find album
		var track database.Track
		if err := m.db.Preload("Album").Where("id = ?", trackID).First(&track).Error; err != nil {
			return fmt.Errorf("track not found: %w", err)
		}
		
		// Update album title
		return m.db.Model(&database.Album{}).Where("id = ?", track.AlbumID).Update("title", value).Error
	
	case "release_year":
		// Get track to find album
		var track database.Track
		if err := m.db.Preload("Album").Where("id = ?", trackID).First(&track).Error; err != nil {
			return fmt.Errorf("track not found: %w", err)
		}
		
		// Parse year and update album release date
		if year, err := strconv.Atoi(value); err == nil {
			releaseDate := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
			return m.db.Model(&database.Album{}).Where("id = ?", track.AlbumID).Update("release_date", releaseDate).Error
		}
		return fmt.Errorf("invalid year format: %s", value)
	
	case "duration":
		if duration, err := strconv.Atoi(value); err == nil {
			return m.db.Model(&database.Track{}).Where("id = ?", trackID).Update("duration", duration).Error
		}
		return fmt.Errorf("invalid duration format: %s", value)
	
	case "track_number":
		if trackNum, err := strconv.Atoi(value); err == nil {
			return m.db.Model(&database.Track{}).Where("id = ?", trackID).Update("track_number", trackNum).Error
		}
		return fmt.Errorf("invalid track number format: %s", value)
	
	default:
		log.Printf("WARN: Unknown track field: %s", fieldName)
		return nil
	}
}

// applyMovieEnrichment applies enrichment to movie entities
func (m *Module) applyMovieEnrichment(movieID, fieldName, value string, strategy MergeStrategy) error {
	switch fieldName {
	case "title":
		return m.db.Model(&database.Movie{}).Where("id = ?", movieID).Update("title", value).Error
	case "release_year":
		if year, err := strconv.Atoi(value); err == nil {
			releaseDate := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
			return m.db.Model(&database.Movie{}).Where("id = ?", movieID).Update("release_date", releaseDate).Error
		}
		return fmt.Errorf("invalid year format: %s", value)
	default:
		log.Printf("WARN: Unknown movie field: %s", fieldName)
		return nil
	}
}

// applyEpisodeEnrichment applies enrichment to episode entities  
func (m *Module) applyEpisodeEnrichment(episodeID, fieldName, value string, strategy MergeStrategy) error {
	// TODO: Implement when episode model is available
	log.Printf("INFO: Episode enrichment not yet implemented for field: %s", fieldName)
	return nil
}

// GetEnrichmentStatus returns enrichment status for a media file
func (m *Module) GetEnrichmentStatus(mediaFileID string) (map[string]interface{}, error) {
	// Get media file to find media ID
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", mediaFileID).First(&mediaFile).Error; err != nil {
		return nil, fmt.Errorf("media file not found: %w", err)
	}

	var enrichments []database.MediaEnrichment
	if err := m.db.Where("media_id = ? AND media_type = ?", mediaFile.MediaID, mediaFile.MediaType).Find(&enrichments).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch enrichments: %w", err)
	}

	status := map[string]interface{}{
		"media_file_id":     mediaFileID,
		"media_id":          mediaFile.MediaID,
		"media_type":        mediaFile.MediaType,
		"total_enrichments": len(enrichments),
		"applied_count":     0,
		"pending_count":     len(enrichments), // All pending since we don't track applied state anymore
		"sources":           make(map[string]int),
		"fields":            make(map[string]interface{}),
	}

	fieldStatus := make(map[string]interface{})
	sources := make(map[string]int)

	for _, enrichment := range enrichments {
		sources[enrichment.Plugin]++
		
		// Parse enrichment data from payload
		var enrichmentData EnrichmentData
		if err := json.Unmarshal([]byte(enrichment.Payload), &enrichmentData); err != nil {
			log.Printf("WARN: Failed to parse enrichment payload from %s: %v", enrichment.Plugin, err)
			continue
		}

		// Group by fields within the enrichment data
		for fieldName, fieldValue := range enrichmentData.Fields {
			if _, exists := fieldStatus[fieldName]; !exists {
				fieldStatus[fieldName] = map[string]interface{}{
					"enrichments": []map[string]interface{}{},
					"best_source": "",
					"applied":     false,
				}
			}

			fieldInfo := fieldStatus[fieldName].(map[string]interface{})
			enrichmentInfo := map[string]interface{}{
				"source":     enrichment.Plugin,
				"value":      fieldValue,
				"confidence": enrichmentData.ConfidenceScore,
				"applied":    false, // We don't track applied state in this table
				"priority":   enrichmentData.SourcePriority,
			}

			fieldInfo["enrichments"] = append(fieldInfo["enrichments"].([]map[string]interface{}), enrichmentInfo)
			
			// Set best source to the one with highest priority (lowest number)
			currentBestPriority, hasBest := fieldInfo["best_priority"]
			if !hasBest || enrichmentData.SourcePriority < currentBestPriority.(int) {
				fieldInfo["best_source"] = enrichment.Plugin
				fieldInfo["best_priority"] = enrichmentData.SourcePriority
			}

			fieldStatus[fieldName] = fieldInfo
		}
	}

	// Clean up temporary priority fields
	for fieldName, fieldInfo := range fieldStatus {
		if info, ok := fieldInfo.(map[string]interface{}); ok {
			delete(info, "best_priority")
			fieldStatus[fieldName] = info
		}
	}

	status["sources"] = sources
	status["fields"] = fieldStatus

	return status, nil
}

// ForceApplyEnrichment manually applies enrichment for a specific field
func (m *Module) ForceApplyEnrichment(mediaFileID, fieldName, sourceName string) error {
	// Get media file to find media ID
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", mediaFileID).First(&mediaFile).Error; err != nil {
		return fmt.Errorf("media file not found: %w", err)
	}

	var enrichment database.MediaEnrichment
	if err := m.db.Where("media_id = ? AND media_type = ? AND plugin = ?", 
		mediaFile.MediaID, mediaFile.MediaType, sourceName).First(&enrichment).Error; err != nil {
		return fmt.Errorf("enrichment not found: %w", err)
	}

	// Parse enrichment data
	var enrichmentData EnrichmentData
	if err := json.Unmarshal([]byte(enrichment.Payload), &enrichmentData); err != nil {
		return fmt.Errorf("failed to parse enrichment data: %w", err)
	}

	// Check if field exists in enrichment data
	value, exists := enrichmentData.Fields[fieldName]
	if !exists {
		return fmt.Errorf("field %s not found in enrichment data", fieldName)
	}

	rules := m.GetFieldRules()
	rule, exists := rules[fieldName]
	if !exists {
		return fmt.Errorf("no rule found for field: %s", fieldName)
	}

	valueStr := fmt.Sprintf("%v", value)
	if err := m.applyFieldToEntity(mediaFile.MediaID, string(mediaFile.MediaType), fieldName, valueStr, rule.MergeStrategy); err != nil {
		return fmt.Errorf("failed to apply enrichment: %w", err)
	}

	enrichment.UpdatedAt = time.Now()
	return m.db.Save(&enrichment).Error
}

// IntegrateWithScanner integrates the enrichment module with the scanner system
func (m *Module) IntegrateWithScanner() {
	log.Println("INFO: Integrating enrichment module with scanner system")
	
	// This method should be called from the main application to set up
	// the integration between scanning and enrichment
}

// OnMediaFileScanned is called by the scanner when a media file is scanned
// This integrates with the existing scanner plugin hook system
func (m *Module) OnMediaFileScanned(mediaFile *database.MediaFile, metadata interface{}) error {
	if !m.enabled {
		return nil
	}

	log.Printf("INFO: Enrichment module processing scanned file: %s", mediaFile.Path)
	
	// For now, just queue an enrichment job to apply any existing enrichments
	// This ensures that if enrichment data was added by plugins, it gets applied
	job := EnrichmentJob{
		MediaFileID: mediaFile.ID,
		JobType:     "apply_enrichment",
		Status:      "pending",
	}
	
	if err := m.db.Create(&job).Error; err != nil {
		log.Printf("WARN: Failed to create enrichment job for %s: %v", mediaFile.Path, err)
		return nil // Don't fail scanning if enrichment job creation fails
	}

	return nil
}

// OnScanStarted is called when a scan starts (ScannerPluginHook interface)
func (m *Module) OnScanStarted(jobID, libraryID uint, path string) error {
	if !m.enabled {
		return nil
	}

	log.Printf("INFO: Enrichment module notified - scan started (job: %d, library: %d, path: %s)", jobID, libraryID, path)
	return nil
}

// OnScanCompleted is called when a scan completes (ScannerPluginHook interface)
func (m *Module) OnScanCompleted(jobID, libraryID uint, stats map[string]interface{}) error {
	if !m.enabled {
		return nil
	}

	log.Printf("INFO: Enrichment module notified - scan completed (job: %d, library: %d)", jobID, libraryID)
	
	// Optionally: Trigger batch enrichment application for newly scanned files
	// This could queue enrichment jobs for all files in the library
	
	return nil
}

// ListEnrichmentSources returns all enrichment sources
func (m *Module) ListEnrichmentSources() ([]EnrichmentSource, error) {
	var sources []EnrichmentSource
	if err := m.db.Find(&sources).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch enrichment sources: %w", err)
	}
	return sources, nil
}

// GetEnrichmentJobs returns enrichment jobs with optional status filter
func (m *Module) GetEnrichmentJobs(status string) ([]EnrichmentJob, error) {
	var jobs []EnrichmentJob
	query := m.db.Order("created_at DESC")
	
	if status != "" {
		query = query.Where("status = ?", status)
	}
	
	if err := query.Limit(100).Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch enrichment jobs: %w", err)
	}
	return jobs, nil
} 