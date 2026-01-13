package monitor

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	domainevents "github.com/mantonx/viewra/internal/domain/events"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
)

// EnrichmentEnqueuer is the interface for enqueueing enrichment jobs.
type EnrichmentEnqueuer interface {
	// EnqueueFirstStage enqueues a media item for the first enrichment stage.
	EnqueueFirstStage(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType, priority int) error
	// EnqueueStage enqueues a media item for a specific enrichment stage.
	EnqueueStage(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType, stage string, priority int) error
}

// ScanOrchestrator is the interface for triggering library scans.
type ScanOrchestrator interface {
	// StartTargetedScan initiates a scan for specific paths within a library.
	StartTargetedScan(ctx context.Context, libraryID int64, targetPaths []string) (interface{}, error)
}

// Handler processes debounced file events and triggers appropriate enrichment.
type Handler struct {
	libraryRepo      library.Repository
	mediaRepo        media.Repository
	enqueuer         EnrichmentEnqueuer
	scanOrchestrator ScanOrchestrator // For triggering targeted scans on new files
	eventBus         domainevents.Publisher
	logger           *slog.Logger

	// Cache of library paths for efficient lookup
	libraryCache   map[int64]*library.Library
	libraryCacheMu sync.RWMutex
}

// NewHandler creates a new event handler.
func NewHandler(
	libraryRepo library.Repository,
	mediaRepo media.Repository,
	enqueuer EnrichmentEnqueuer,
	scanOrchestrator ScanOrchestrator,
	eventBus domainevents.Publisher,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		libraryRepo:      libraryRepo,
		mediaRepo:        mediaRepo,
		enqueuer:         enqueuer,
		scanOrchestrator: scanOrchestrator,
		eventBus:         eventBus,
		logger:           logger.With("component", "monitor_handler"),
		libraryCache:     make(map[int64]*library.Library),
	}
}

// HandleEvent processes a debounced file event.
func (h *Handler) HandleEvent(ctx context.Context, event DebouncedEvent, lib *library.Library) error {
	h.logger.Debug("Handling file event",
		"path", event.Path,
		"change_type", event.ChangeType,
		"op", event.LastOp,
		"event_count", event.EventCount)

	// Handle deletions differently
	if event.LastOp == OpRemove {
		return h.handleDelete(ctx, event, lib)
	}

	// For creates/modifications, find or create the media item and trigger enrichment
	return h.handleCreateOrModify(ctx, event, lib)
}

// handleDelete processes a file deletion event.
// For now, we don't automatically delete media items - that would be too destructive.
// The user can manually rescan or the orphan detection can handle it.
func (h *Handler) handleDelete(ctx context.Context, event DebouncedEvent, lib *library.Library) error {
	relPath := h.getRelativePath(event.Path, lib.Path)
	if relPath == "" {
		return nil
	}

	h.logger.Info("File deleted (no action taken)",
		"library_id", lib.ID,
		"path", relPath,
		"change_type", event.ChangeType)

	// Publish event for SSE subscribers
	if h.eventBus != nil {
		h.eventBus.Publish(domainevents.NewEvent(domainevents.EventMediaRemoved, "monitor").
			WithLibraryID(lib.ID).
			WithData("path", relPath).
			Build())
	}

	return nil
}

// handleCreateOrModify processes file create/modify events.
func (h *Handler) handleCreateOrModify(ctx context.Context, event DebouncedEvent, lib *library.Library) error {
	relPath := h.getRelativePath(event.Path, lib.Path)
	if relPath == "" {
		h.logger.Warn("File path outside library",
			"path", event.Path,
			"library_path", lib.Path)
		return nil
	}

	// Get the enrichment stage to trigger based on file type
	stage := GetEnrichmentStage(event.ChangeType)
	mediaType := h.getMediaTypeFromLibrary(lib)
	priority := h.getPriority(lib)

	// For media files, we need to find the existing media item
	// For NFO/images, we need to find the parent media item
	mediaItem, err := h.findMediaItem(ctx, event, lib, relPath)
	if err != nil || mediaItem == nil {
		h.logger.Debug("Could not find media item for file",
			"path", relPath,
			"error", err)
		// If it's a new media file, trigger a targeted scan to process it
		if event.ChangeType == FileChangeMedia && event.LastOp == OpCreate && h.scanOrchestrator != nil {
			h.logger.Info("New media file detected - triggering targeted scan",
				"library_id", lib.ID,
				"path", relPath)

			// Get parent directory for scanning
			// This handles cases where multiple files might be added at once
			parentDir := filepath.Dir(relPath)
			targetPaths := []string{parentDir}

			_, scanErr := h.scanOrchestrator.StartTargetedScan(ctx, lib.ID, targetPaths)
			if scanErr != nil {
				h.logger.Error("Failed to start targeted scan for new file",
					"library_id", lib.ID,
					"path", relPath,
					"error", scanErr)
			}
		}
		return nil
	}

	h.logger.Info("Triggering enrichment for file change",
		"library_id", lib.ID,
		"media_id", mediaItem.ID,
		"path", relPath,
		"change_type", event.ChangeType,
		"stage", stage,
		"priority", priority)

	// Trigger enrichment
	if stage == "" {
		// Media file changed - trigger full pipeline from first stage
		return h.enqueuer.EnqueueFirstStage(ctx, mediaItem.ID, lib.ID, mediaType, priority)
	}

	// NFO or local image changed - trigger specific stage
	return h.enqueuer.EnqueueStage(ctx, mediaItem.ID, lib.ID, mediaType, stage, priority)
}

// findMediaItem attempts to find the media item for a file.
func (h *Handler) findMediaItem(ctx context.Context, event DebouncedEvent, lib *library.Library, relPath string) (*media.Media, error) {
	switch event.ChangeType {
	case FileChangeMedia:
		// Direct lookup by file path
		return h.mediaRepo.GetByFilePath(ctx, lib.ID, relPath)

	case FileChangeNFO, FileChangeLocalImage:
		// For NFO and images, find the parent media file
		return h.findParentMedia(ctx, lib, relPath)
	}

	return nil, nil
}

// findParentMedia finds the media item for an NFO or image file.
// NFO/images are typically in the same directory as the media file.
func (h *Handler) findParentMedia(ctx context.Context, lib *library.Library, relPath string) (*media.Media, error) {
	dir := filepath.Dir(relPath)

	// Get all media in this library and find one in the same directory
	// This is a simplistic approach - a better implementation would use an index
	allMedia, err := h.mediaRepo.ListByLibrary(ctx, lib.ID)
	if err != nil {
		return nil, err
	}

	// Find media file in the same directory
	for _, m := range allMedia {
		mediaDir := filepath.Dir(m.FilePath)
		if mediaDir == dir {
			return m, nil
		}
	}

	// For TV episodes, the NFO might be in a parent directory (show.nfo, season.nfo)
	// Walk up the directory tree
	for dir != "." && dir != "/" && dir != "" {
		for _, m := range allMedia {
			if strings.HasPrefix(m.FilePath, dir+"/") {
				return m, nil
			}
		}
		dir = filepath.Dir(dir)
	}

	return nil, nil
}

// getRelativePath converts an absolute path to a library-relative path.
func (h *Handler) getRelativePath(absPath, libraryPath string) string {
	if !strings.HasPrefix(absPath, libraryPath) {
		return ""
	}

	rel := strings.TrimPrefix(absPath, libraryPath)
	rel = strings.TrimPrefix(rel, "/")
	return rel
}

// getMediaTypeFromLibrary determines the media type from the library type.
func (h *Handler) getMediaTypeFromLibrary(lib *library.Library) enrichment.MediaType {
	switch lib.Type {
	case library.LibraryTypeMovies:
		return enrichment.MediaTypeMovie
	case library.LibraryTypeTV:
		return enrichment.MediaTypeTVShow
	case library.LibraryTypeMusic:
		return enrichment.MediaTypeMusic
	default:
		return enrichment.MediaTypeMovie
	}
}

// getPriority returns the enrichment priority for a library.
func (h *Handler) getPriority(lib *library.Library) int {
	config := lib.GetEffectiveConfig()
	return config.Priority
}

// RefreshLibraryCache reloads the library cache from the database.
func (h *Handler) RefreshLibraryCache(ctx context.Context) error {
	libs, err := h.libraryRepo.List(ctx)
	if err != nil {
		return err
	}

	h.libraryCacheMu.Lock()
	defer h.libraryCacheMu.Unlock()

	h.libraryCache = make(map[int64]*library.Library, len(libs))
	for _, lib := range libs {
		h.libraryCache[lib.ID] = lib
	}

	return nil
}

// GetCachedLibrary returns a cached library by ID.
func (h *Handler) GetCachedLibrary(id int64) *library.Library {
	h.libraryCacheMu.RLock()
	defer h.libraryCacheMu.RUnlock()
	return h.libraryCache[id]
}
