package library

import (
	"context"
	"fmt"
	"time"

	"github.com/viewra/viewra/internal/domain/library"
	"github.com/viewra/viewra/internal/domain/media"
	"github.com/viewra/viewra/internal/domain/scanner"
	"github.com/viewra/viewra/internal/infrastructure/filesystem"
)

// ScanLibraryUseCase handles the business logic for scanning a library
type ScanLibraryUseCase struct {
	libraryRepo library.Repository
	mediaRepo   media.Repository
	movieRepo   media.MovieRepository
	tvRepo      media.TVRepository
	musicRepo   media.MusicRepository
	scanJobRepo scanner.ScanJobRepository
}

// NewScanLibraryUseCase creates a new instance of ScanLibraryUseCase
func NewScanLibraryUseCase(
	libraryRepo library.Repository,
	mediaRepo media.Repository,
	movieRepo media.MovieRepository,
	tvRepo media.TVRepository,
	musicRepo media.MusicRepository,
	scanJobRepo scanner.ScanJobRepository,
) *ScanLibraryUseCase {
	return &ScanLibraryUseCase{
		libraryRepo: libraryRepo,
		mediaRepo:   mediaRepo,
		movieRepo:   movieRepo,
		tvRepo:      tvRepo,
		musicRepo:   musicRepo,
		scanJobRepo: scanJobRepo,
	}
}

// StartScan initiates a new scan for a library
func (uc *ScanLibraryUseCase) StartScan(ctx context.Context, libraryID int64) (StartScanResponse, error) {
	// Verify library exists
	lib, err := uc.libraryRepo.GetByID(ctx, libraryID)
	if err != nil {
		return StartScanResponse{}, fmt.Errorf("failed to get library: %w", err)
	}

	// Check for existing running scan
	running, err := uc.scanJobRepo.ListRunning(ctx)
	if err != nil {
		return StartScanResponse{}, fmt.Errorf("failed to check running scans: %w", err)
	}
	for _, job := range running {
		if job.LibraryID == libraryID {
			return StartScanResponse{}, scanner.ErrAlreadyRunning
		}
	}

	// Create scan job
	job := &scanner.ScanJob{
		LibraryID:      libraryID,
		Status:         scanner.ScanStatusRunning,
		Progress:       0,
		FilesFound:     0,
		FilesProcessed: 0,
		BytesProcessed: 0,
		ErrorCount:     0,
		StartedAt:      time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := uc.scanJobRepo.Create(ctx, job); err != nil {
		return StartScanResponse{}, fmt.Errorf("failed to create scan job: %w", err)
	}

	// Start background scan
	go uc.runScan(context.Background(), job.ID, lib)

	return ToStartScanResponse(job), nil
}

// runScan executes the actual scan in the background
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
	// Create coordinator
	coordinator := filesystem.NewCoordinator(filesystem.DefaultCoordinatorConfig())

	// Create result channel
	resultChan := make(chan scanner.ScanResult, 100)

	// Start result processor in separate goroutine
	processDone := make(chan struct{})
	go func() {
		defer close(processDone)
		uc.processResults(ctx, jobID, lib.ID, lib.Type, resultChan)
	}()

	// Run the scan
	scanErr := coordinator.Scan(ctx, lib.Path, resultChan)
	close(resultChan)

	// Wait for result processing to complete
	<-processDone

	// Get final progress
	progress := coordinator.GetProgress()

	// Update job with final status
	job := &scanner.ScanJob{
		ID:             jobID,
		FilesFound:     progress.FilesFound,
		FilesProcessed: progress.FilesProcessed,
		BytesProcessed: progress.BytesProcessed,
		ErrorCount:     progress.ErrorCount,
		Progress:       progress.GetPercentage(),
		CompletedAt:    &progress.LastUpdate,
	}

	if scanErr != nil {
		job.Status = scanner.ScanStatusFailed
		job.ErrorMessage = scanErr.Error()
	} else {
		job.Status = scanner.ScanStatusCompleted
	}

	// Mark job as complete
	if err := uc.scanJobRepo.Complete(ctx, job); err != nil {
		// Log error but don't fail the scan
		fmt.Printf("failed to complete scan job: %v\n", err)
	}
}

// processResults processes scan results and creates/updates media entries
func (uc *ScanLibraryUseCase) processResults(ctx context.Context, jobID int64, libraryID int64, libraryType library.LibraryType, resultChan <-chan scanner.ScanResult) {
	updateTicker := time.NewTicker(2 * time.Second)
	defer updateTicker.Stop()

	progress := &scanner.Progress{
		StartTime:  time.Now(),
		LastUpdate: time.Now(),
	}

	for result := range resultChan {
		// Update progress counters
		progress.FilesProcessed++
		progress.BytesProcessed += result.BytesProcessed
		progress.LastUpdate = time.Now()

		if result.Error != nil {
			progress.ErrorCount++
			continue
		}

		// Create media entry based on library type
		switch libraryType {
		case library.LibraryTypeMovies:
			uc.processMovie(ctx, libraryID, &result)
		case library.LibraryTypeTV:
			uc.processTVEpisode(ctx, libraryID, &result)
		case library.LibraryTypeMusic:
			uc.processMusicTrack(ctx, libraryID, &result)
		}

		// Periodically update job progress
		select {
		case <-updateTicker.C:
			if err := uc.scanJobRepo.UpdateProgress(ctx, jobID, progress); err != nil {
				fmt.Printf("failed to update scan progress: %v\n", err)
			}
		default:
		}
	}

	// Final progress update
	progress.LastUpdate = time.Now()
	if err := uc.scanJobRepo.UpdateProgress(ctx, jobID, progress); err != nil {
		fmt.Printf("failed to update final scan progress: %v\n", err)
	}
}

// processMovie creates or updates a movie entry
func (uc *ScanLibraryUseCase) processMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult) {
	movie := &media.Movie{
		Media: media.Media{
			LibraryID: libraryID,
			Title:     result.Title,
			FilePath:  result.FilePath,
			Duration:  int(result.Duration),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	if result.Year != nil {
		movie.Year = *result.Year
	}

	// Check if movie already exists
	existing, err := uc.mediaRepo.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		movie.Media.ID = existing.ID
		if err := uc.movieRepo.UpdateMovie(ctx, movie); err != nil {
			fmt.Printf("failed to update movie %s: %v\n", result.FilePath, err)
		}
		return
	}

	// Create new entry
	if err := uc.movieRepo.CreateMovie(ctx, movie); err != nil {
		fmt.Printf("failed to create movie %s: %v\n", result.FilePath, err)
	}
}

// processTVEpisode creates or updates a TV episode entry
func (uc *ScanLibraryUseCase) processTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult) {
	episode := &media.TVEpisode{
		Media: media.Media{
			LibraryID: libraryID,
			Title:     result.Title,
			FilePath:  result.FilePath,
			Duration:  int(result.Duration),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	if result.SeasonNumber != nil {
		episode.Season = *result.SeasonNumber
	}
	if result.EpisodeNumber != nil {
		episode.Episode = *result.EpisodeNumber
	}

	// Check if episode already exists
	existing, err := uc.mediaRepo.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		episode.Media.ID = existing.ID
		if err := uc.tvRepo.UpdateTVEpisode(ctx, episode); err != nil {
			fmt.Printf("failed to update TV episode %s: %v\n", result.FilePath, err)
		}
		return
	}

	// Create new entry
	if err := uc.tvRepo.CreateTVEpisode(ctx, episode); err != nil {
		fmt.Printf("failed to create TV episode %s: %v\n", result.FilePath, err)
	}
}

// processMusicTrack creates or updates a music track entry
func (uc *ScanLibraryUseCase) processMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult) {
	track := &media.MusicTrack{
		Media: media.Media{
			LibraryID: libraryID,
			Title:     result.Title,
			FilePath:  result.FilePath,
			Duration:  int(result.Duration),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Artist: result.Artist,
		Album:  result.Album,
	}

	if result.TrackNumber != nil {
		track.TrackNumber = *result.TrackNumber
	}
	if result.Year != nil {
		track.Year = *result.Year
	}

	// Check if track already exists
	existing, err := uc.mediaRepo.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		track.Media.ID = existing.ID
		if err := uc.musicRepo.UpdateMusicTrack(ctx, track); err != nil {
			fmt.Printf("failed to update music track %s: %v\n", result.FilePath, err)
		}
		return
	}

	// Create new entry
	if err := uc.musicRepo.CreateMusicTrack(ctx, track); err != nil {
		fmt.Printf("failed to create music track %s: %v\n", result.FilePath, err)
	}
}

// GetProgress retrieves the current progress of a scan job
func (uc *ScanLibraryUseCase) GetProgress(ctx context.Context, jobID int64) (ScanProgressResponse, error) {
	job, err := uc.scanJobRepo.GetByID(ctx, jobID)
	if err != nil {
		return ScanProgressResponse{}, fmt.Errorf("failed to get scan job: %w", err)
	}

	return ToScanProgressResponse(job), nil
}

// GetLatestScan retrieves the most recent scan for a library
func (uc *ScanLibraryUseCase) GetLatestScan(ctx context.Context, libraryID int64) (ScanProgressResponse, error) {
	job, err := uc.scanJobRepo.GetLatestByLibrary(ctx, libraryID)
	if err != nil {
		return ScanProgressResponse{}, fmt.Errorf("failed to get latest scan: %w", err)
	}

	return ToScanProgressResponse(job), nil
}

// GetScanHistory retrieves scan history for a library
func (uc *ScanLibraryUseCase) GetScanHistory(ctx context.Context, libraryID int64, limit int32) (ScanHistoryResponse, error) {
	jobs, err := uc.scanJobRepo.ListByLibrary(ctx, libraryID, limit)
	if err != nil {
		return ScanHistoryResponse{}, fmt.Errorf("failed to get scan history: %w", err)
	}

	return ToScanHistoryResponse(jobs), nil
}
