package library

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/domain/scanner/parsers"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/music"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/nfo"
)

// ScanLibraryUseCase handles the business logic for scanning a library
type ScanLibraryUseCase struct {
	libraryRepo          library.Repository
	mediaRepo            media.Repository
	movieRepo            media.MovieRepository
	tvRepo               media.TVRepository
	musicRepo            media.MusicRepository
	scanJobRepo          scanner.ScanJobRepository
	extractMovieImages   ExtractMovieImagesExecutor
	extractEpisodeImages ExtractTVEpisodeImagesExecutor
	extractShowImages    ExtractTVShowImagesExecutor
	extractSeasonImages  ExtractTVSeasonImagesExecutor
	extractMusicImages   ExtractMusicAlbumImagesExecutor
	extractArtistImages  ExtractMusicArtistImagesExecutor
	imageRepo            images.Repository
	imageCleanup         ImageCleanupExecutor
	scanTimeout          time.Duration

	// Artist deduplication tracking (per scan session)
	// Using sync.Map for lock-free concurrent access (fixes race condition)
	processedArtists sync.Map // string -> bool
}

// ExtractMovieImagesExecutor interface for movie image extraction
type ExtractMovieImagesExecutor interface {
	Execute(ctx context.Context, movieFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error
}

// ExtractTVEpisodeImagesExecutor interface for TV episode image extraction
type ExtractTVEpisodeImagesExecutor interface {
	Execute(ctx context.Context, episodeFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error
}

// ExtractTVShowImagesExecutor interface for TV show image extraction
type ExtractTVShowImagesExecutor interface {
	Execute(ctx context.Context, showDir string, mediaType images.MediaType, entityID int) error
}

// ExtractTVSeasonImagesExecutor interface for TV season image extraction
type ExtractTVSeasonImagesExecutor interface {
	Execute(ctx context.Context, showDir string, seasonNumber int, mediaType images.MediaType, entityID int) error
}

// ExtractMusicAlbumImagesExecutor interface for music album image extraction
type ExtractMusicAlbumImagesExecutor interface {
	Execute(ctx context.Context, albumDir string, mediaType images.MediaType, entityID int) error
}

// ExtractMusicArtistImagesExecutor interface for music artist image extraction
type ExtractMusicArtistImagesExecutor interface {
	Execute(ctx context.Context, artistDir string, mediaType images.MediaType, entityID int) error
}

// NewScanLibraryUseCase creates a new instance of ScanLibraryUseCase
func NewScanLibraryUseCase(
	libraryRepo library.Repository,
	mediaRepo media.Repository,
	movieRepo media.MovieRepository,
	tvRepo media.TVRepository,
	musicRepo media.MusicRepository,
	scanJobRepo scanner.ScanJobRepository,
	extractMovieImages ExtractMovieImagesExecutor,
	extractEpisodeImages ExtractTVEpisodeImagesExecutor,
	extractShowImages ExtractTVShowImagesExecutor,
	extractSeasonImages ExtractTVSeasonImagesExecutor,
	extractMusicImages ExtractMusicAlbumImagesExecutor,
	extractArtistImages ExtractMusicArtistImagesExecutor,
	imageRepo images.Repository,
	imageCleanup ImageCleanupExecutor,
	scanTimeout time.Duration,
) *ScanLibraryUseCase {
	return &ScanLibraryUseCase{
		libraryRepo:          libraryRepo,
		mediaRepo:            mediaRepo,
		movieRepo:            movieRepo,
		tvRepo:               tvRepo,
		musicRepo:            musicRepo,
		scanJobRepo:          scanJobRepo,
		extractMovieImages:   extractMovieImages,
		extractEpisodeImages: extractEpisodeImages,
		extractShowImages:    extractShowImages,
		extractSeasonImages:  extractSeasonImages,
		extractMusicImages:   extractMusicImages,
		extractArtistImages:  extractArtistImages,
		imageRepo:            imageRepo,
		imageCleanup:         imageCleanup,
		scanTimeout:          scanTimeout,
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

	// Start background scan with timeout
	// Use a new context with timeout (not derived from request context which may cancel early)
	scanCtx, cancel := context.WithTimeout(context.Background(), uc.scanTimeout)

	// Start the scan goroutine with proper cleanup and panic recovery
	go func() {
		defer cancel() // Always cancel to prevent context leak

		// Recover from panics to prevent crashing the entire application
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("PANIC: scan goroutine panicked: %v\nStack trace:\n%s\n", r, string(debug.Stack()))

				// Mark job as failed
				failedJob := &scanner.ScanJob{
					ID:           job.ID,
					Status:       scanner.ScanStatusFailed,
					ErrorMessage: fmt.Sprintf("scan panicked: %v", r),
					CompletedAt:  &[]time.Time{time.Now()}[0],
				}
				if err := uc.scanJobRepo.Complete(context.Background(), failedJob); err != nil {
					fmt.Printf("error: failed to mark panicked scan job as failed: %v\n", err)
				}
			}
		}()

		uc.runScan(scanCtx, job.ID, lib)
	}()

	// Monitor parent context cancellation (if user cancels the request, stop the scan)
	go func() {
		<-ctx.Done()
		cancel() // Parent context cancelled, trigger scan cancellation
	}()

	return ToStartScanResponse(job), nil
}

// runScan executes the actual scan in the background
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
	// Initialize artist deduplication tracking for this scan session
	// Clear any previous entries (sync.Map doesn't have a Clear method, so we replace it)
	uc.processedArtists = sync.Map{}

	// Create coordinator
	coordinator := filesystem.NewCoordinator(filesystem.DefaultCoordinatorConfig())

	// Create result channel
	resultChan := make(chan scanner.ScanResult, 100)

	// Start result processor in separate goroutine
	// Use unbuffered channel and drain in separate goroutine to avoid deadlock
	foundFilePaths := make(chan string)
	foundFiles := make(map[string]bool)
	foundFilesMu := sync.Mutex{}

	// Goroutine to collect found file paths
	foundFilesCollectorDone := make(chan struct{})
	go func() {
		defer close(foundFilesCollectorDone)
		for filePath := range foundFilePaths {
			foundFilesMu.Lock()
			foundFiles[filePath] = true
			foundFilesMu.Unlock()
		}
	}()

	processDone := make(chan struct{})
	go func() {
		defer close(processDone)
		uc.processResults(ctx, jobID, lib.ID, lib.Type, resultChan, foundFilePaths)
	}()

	// Run the scan
	scanErr := coordinator.Scan(ctx, lib.Path, resultChan)
	close(resultChan)

	// Wait for result processing to complete
	<-processDone

	// foundFilePaths channel is now closed by processResults
	// Wait for found files collector to finish collecting all paths
	<-foundFilesCollectorDone

	// Now it's safe to read foundFiles without a lock since collector is done

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

	// CRITICAL: Only cleanup stale media on COMPLETELY successful scan
	// Prevents data corruption if scan partially fails (e.g., permission error on one directory)
	// We only cleanup if:
	// 1. No scan errors occurred
	// 2. No processing errors occurred (ErrorCount == 0)
	// 3. All found files were successfully processed (FilesProcessed == FilesFound)
	if scanErr == nil && progress.ErrorCount == 0 && progress.FilesProcessed == progress.FilesFound {
		if uc.imageRepo != nil && uc.imageCleanup != nil {
			uc.cleanupStaleMedia(ctx, lib.ID, foundFiles)
		}
	} else {
		fmt.Printf("info: skipping stale media cleanup due to scan errors (scanErr=%v, errorCount=%d, processed=%d, found=%d)\n",
			scanErr, progress.ErrorCount, progress.FilesProcessed, progress.FilesFound)
	}
}

// isExtra determines if a file is an extra (trailer, deleted scene, featurette, etc.)
// based on common filename patterns
func isExtra(filepath string) bool {
	lower := strings.ToLower(filepath)
	extraPatterns := []string{
		"-trailer.",
		"_trailer.",
		".trailer.",
		"-deleted",
		"_deleted",
		".deleted",
		"-featurette",
		"_featurette",
		".featurette",
		"-extra.",
		"_extra.",
		".extra.",
		"-bonus.",
		"_bonus.",
		".bonus.",
	}

	for _, pattern := range extraPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// processResults processes scan results and creates/updates media entries
func (uc *ScanLibraryUseCase) processResults(ctx context.Context, jobID int64, libraryID int64, libraryType library.LibraryType, resultChan <-chan scanner.ScanResult, foundFilePaths chan<- string) {
	updateTicker := time.NewTicker(2 * time.Second)
	defer updateTicker.Stop()
	defer close(foundFilePaths)

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

		// Track this file as found (use select to prevent deadlock if context is cancelled)
		select {
		case foundFilePaths <- result.FilePath:
			// Successfully sent
		case <-ctx.Done():
			// Context cancelled, stop processing
			return
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
	// Coordinator already parsed the filename - just use the results
	movie := &media.Movie{
		Media: media.Media{
			LibraryID:       libraryID,
			Title:           result.Title,
			FilePath:        result.FilePath,
			FileSize:        result.FileSize,
			Duration:        int(result.Duration),
			IsExtra:         isExtra(result.FilePath),
			Width:           result.Width,
			Height:          result.Height,
			VideoCodec:      result.VideoCodec,
			AudioCodec:      result.AudioCodec,
			Bitrate:         result.Bitrate,
			FrameRate:       result.FrameRate,
			ContainerFormat: result.ContainerFormat,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}

	// Set year from scan result
	if result.Year != nil {
		movie.Year = *result.Year
	}

	// Try to enhance metadata from NFO file
	nfoPath, err := nfo.FindMovieNFO(result.FilePath)
	if err == nil && nfoPath != "" {
		nfoMetadata, err := nfo.ParseMovieNFO(nfoPath)
		if err == nil && nfoMetadata != nil {
			// Populate movie metadata from NFO
			if nfoMetadata.Title != "" {
				movie.Media.Title = nfoMetadata.Title
			}
			if nfoMetadata.Year > 0 {
				movie.Year = nfoMetadata.Year
			}
			movie.OriginalTitle = nfoMetadata.OriginalTitle
			movie.SortTitle = nfoMetadata.SortTitle
			movie.ReleaseDate = nfoMetadata.ReleaseDate
			movie.RuntimeMinutes = nfoMetadata.RuntimeMinutes
			movie.IMDbID = nfoMetadata.IMDbID
			movie.TMDbID = nfoMetadata.TMDbID
			movie.Director = nfoMetadata.Director
			movie.Cast = nfoMetadata.Cast
			movie.Genre = nfoMetadata.Genre
			movie.Plot = nfoMetadata.Plot
			movie.Tagline = nfoMetadata.Tagline
			movie.ContentRating = nfoMetadata.ContentRating
			movie.MaturityRating = nfoMetadata.MaturityRating
			movie.ContentAdvisories = nfoMetadata.ContentAdvisories
			movie.Budget = nfoMetadata.Budget
			movie.Revenue = nfoMetadata.Revenue
			movie.OriginalLanguage = nfoMetadata.OriginalLanguage
			movie.CountryOfOrigin = nfoMetadata.CountryOfOrigin
			movie.AwardsSummary = nfoMetadata.AwardsSummary
		}
	}

	// Ensure SortTitle is always set with normalized value
	if movie.SortTitle == "" {
		movie.SortTitle = domainCommon.NormalizeSortTitle(movie.Media.Title)
	}

	// Check if movie already exists
	existing, err := uc.mediaRepo.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		movie.Media.ID = existing.ID
		movie.Media.Type = "movie"
		if err := uc.mediaRepo.Update(ctx, &movie.Media); err != nil {
			fmt.Printf("failed to update media %s: %v\n", result.FilePath, err)
		}
		if err := uc.movieRepo.UpdateMovie(ctx, movie); err != nil {
			fmt.Printf("failed to update movie metadata %s: %v\n", result.FilePath, err)
		}
		// Extract and catalog images (even for existing movies to populate cache)
		uc.extractImagesForMovie(ctx, movie, result.FilePath)
		return
	}

	// Create new entry - let movie repository handle both media and movie records
	movie.Media.Type = "movie"
	if err := uc.movieRepo.CreateMovie(ctx, movie); err != nil {
		fmt.Printf("failed to create movie %s: %v\n", result.FilePath, err)
		return
	}

	// Extract and catalog images for the movie
	uc.extractImagesForMovie(ctx, movie, result.FilePath)
}

// processTVEpisode creates or updates a TV episode entry
func (uc *ScanLibraryUseCase) processTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult) {
	// Coordinator already parsed season/episode/title, but we need show name which isn't in ScanResult
	parser := parsers.NewDefaultParser()
	tvInfo, err := parser.ParseTVEpisode(result.FilePath)
	if err != nil || tvInfo == nil {
		fmt.Printf("failed to parse TV episode filename %s: %v\n", result.FilePath, err)
		return // Can't create episode without show name
	}

	// Use coordinator's parsed data where available, parser for show name
	showTitle := tvInfo.ShowName
	season := 0
	episodeNumber := 0
	episodeTitle := result.Title

	// Prefer coordinator's parsed season/episode numbers
	if result.SeasonNumber != nil {
		season = *result.SeasonNumber
	} else {
		season = tvInfo.Season
	}

	if result.EpisodeNumber != nil {
		episodeNumber = *result.EpisodeNumber
	} else {
		episodeNumber = tvInfo.Episode
	}

	// Use parser's episode title if result title is empty
	if result.Title == "" && tvInfo != nil && tvInfo.EpisodeTitle != "" {
		episodeTitle = tvInfo.EpisodeTitle
	}

	episode := &media.TVEpisode{
		Media: media.Media{
			LibraryID:       libraryID,
			Title:           episodeTitle,
			FilePath:        result.FilePath,
			FileSize:        result.FileSize,
			Duration:        int(result.Duration),
			IsExtra:         isExtra(result.FilePath),
			Width:           result.Width,
			Height:          result.Height,
			VideoCodec:      result.VideoCodec,
			AudioCodec:      result.AudioCodec,
			Bitrate:         result.Bitrate,
			FrameRate:       result.FrameRate,
			ContainerFormat: result.ContainerFormat,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		ShowTitle:    showTitle,
		Season:       season,
		Episode:      episodeNumber,
		EpisodeTitle: episodeTitle,
	}

	// Try to enhance metadata from NFO file
	nfoPath, err := nfo.FindEpisodeNFO(result.FilePath)
	if err == nil && nfoPath != "" {
		nfoMetadata, err := nfo.ParseEpisodeNFO(nfoPath)
		if err == nil && nfoMetadata != nil {
			// Populate episode metadata from NFO
			if nfoMetadata.Title != "" {
				episode.EpisodeTitle = nfoMetadata.Title
				episode.Media.Title = nfoMetadata.Title
			}
			if nfoMetadata.ShowTitle != "" {
				episode.ShowTitle = nfoMetadata.ShowTitle
			}
			if nfoMetadata.Season > 0 {
				episode.Season = nfoMetadata.Season
			}
			if nfoMetadata.Episode > 0 {
				episode.Episode = nfoMetadata.Episode
			}
			if nfoMetadata.Plot != "" {
				episode.Description = nfoMetadata.Plot
			}
			if !nfoMetadata.AirDate.IsZero() {
				episode.AirDate = nfoMetadata.AirDate.Format("2006-01-02")
			}
			if nfoMetadata.RuntimeMinutes > 0 {
				episode.Media.Duration = nfoMetadata.RuntimeMinutes * 60
			}
			episode.IMDbID = nfoMetadata.IMDbID
			episode.TVDbID = nfoMetadata.TVDbID
		}
	}

	// Check if episode already exists
	existing, err := uc.mediaRepo.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		episode.Media.ID = existing.ID
		episode.Media.Type = "tv_episode"
		if err := uc.mediaRepo.Update(ctx, &episode.Media); err != nil {
			fmt.Printf("failed to update media %s: %v\n", result.FilePath, err)
		}
		if err := uc.tvRepo.UpdateTVEpisode(ctx, episode); err != nil {
			fmt.Printf("failed to update TV episode metadata %s: %v\n", result.FilePath, err)
		}
		// Extract and catalog images (even for existing episodes to populate cache)
		uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
		return
	}

	// Create new entry - let TV repository handle both media and episode records
	episode.Media.Type = "tv_episode"
	if err := uc.tvRepo.CreateTVEpisode(ctx, episode); err != nil {
		fmt.Printf("failed to create TV episode %s: %v\n", result.FilePath, err)
		return
	}

	// Extract and catalog images for the episode, show, and season
	uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
}

// processMusicTrack creates or updates a music track entry
func (uc *ScanLibraryUseCase) processMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult) {
	// Use coordinator's parsed metadata
	title := result.Title
	artist := result.Artist
	album := result.Album
	trackNumber := 0
	year := 0
	albumArtist := ""
	discNumber := 0
	genre := ""

	if result.TrackNumber != nil {
		trackNumber = *result.TrackNumber
	}
	if result.Year != nil {
		year = *result.Year
	}

	// Try to extract additional metadata if coordinator's result is incomplete
	// This handles fields like album artist, disc number, and genre that aren't in ScanResult
	if artist == "" || album == "" || genre == "" {
		extractor := music.NewExtractor()
		musicInfo, err := extractor.ExtractMetadata(result.FilePath)
		if err == nil && musicInfo != nil {
			// Fill in missing fields from ID3 tags
			if title == "" {
				title = musicInfo.Title
			}
			if artist == "" {
				artist = musicInfo.Artist
			}
			if album == "" {
				album = musicInfo.Album
			}
			if albumArtist == "" {
				albumArtist = musicInfo.AlbumArtist
			}
			if genre == "" {
				genre = musicInfo.Genre
			}
			if trackNumber == 0 {
				trackNumber = musicInfo.TrackNumber
			}
			if discNumber == 0 {
				discNumber = musicInfo.DiscNumber
			}
			if year == 0 {
				year = musicInfo.Year
			}
		}
	}

	track := &media.MusicTrack{
		Media: media.Media{
			LibraryID:       libraryID,
			Title:           title,
			FilePath:        result.FilePath,
			FileSize:        result.FileSize,
			Duration:        int(result.Duration),
			IsExtra:         isExtra(result.FilePath),
			AudioCodec:      result.AudioCodec,
			Bitrate:         result.Bitrate,
			ContainerFormat: result.ContainerFormat,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		Artist:      artist,
		Album:       album,
		AlbumArtist: albumArtist,
		TrackNumber: trackNumber,
		DiscNumber:  discNumber,
		Genre:       genre,
		Year:        year,
	}

	// Check if track already exists
	existing, err := uc.mediaRepo.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		track.Media.ID = existing.ID
		track.Media.Type = "music_track"
		if err := uc.mediaRepo.Update(ctx, &track.Media); err != nil {
			fmt.Printf("failed to update media %s: %v\n", result.FilePath, err)
		}
		if err := uc.musicRepo.UpdateMusicTrack(ctx, track); err != nil {
			fmt.Printf("failed to update music track metadata %s: %v\n", result.FilePath, err)
		}
		// Extract album and artist images (even for existing tracks to populate cache)
		uc.extractImagesForTrack(ctx, track, result.FilePath)
		return
	}

	// Create new entry - let music repository handle both media and track records
	track.Media.Type = "music_track"
	if err := uc.musicRepo.CreateMusicTrack(ctx, track); err != nil {
		fmt.Printf("failed to create music track %s: %v\n", result.FilePath, err)
		return
	}

	// Extract and catalog images for the album and artist
	uc.extractImagesForTrack(ctx, track, result.FilePath)
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

// cleanupStaleMedia removes media database records and associated images for files that no longer exist on disk
//
// IMPORTANT: This only deletes database records and image cache files, NOT actual media files.
// The media files were already removed by the user from disk - we're just cleaning up our catalog.
func (uc *ScanLibraryUseCase) cleanupStaleMedia(ctx context.Context, libraryID int64, foundFiles map[string]bool) {
	// Get all media items for this library
	allMedia, err := uc.mediaRepo.ListByLibrary(ctx, libraryID)
	if err != nil {
		fmt.Printf("warning: failed to list media for stale cleanup: %v\n", err)
		return
	}

	// Count how many files would be marked as stale
	staleCount := 0
	for _, m := range allMedia {
		if !foundFiles[m.FilePath] {
			staleCount++
		}
	}

	// SAFETY: Don't delete if >10% of library is "stale"
	// This likely indicates scan failure (permission error, network issue, etc.), not actual deletions
	// Better to leave stale records than accidentally delete valid media entries
	if len(allMedia) > 0 {
		stalePercent := float64(staleCount) / float64(len(allMedia)) * 100
		if stalePercent > 10.0 {
			fmt.Printf("error: refusing to cleanup - too many files marked stale (stale=%d, total=%d, percentage=%.1f%%). This likely indicates a scan failure, not actual file deletions.\n",
				staleCount, len(allMedia), stalePercent)
			return
		}
	}

	if staleCount == 0 {
		fmt.Printf("info: no stale media to cleanup\n")
		return
	}

	fmt.Printf("info: cleaning up %d stale media records (%.1f%% of library)\n",
		staleCount, float64(staleCount)/float64(len(allMedia))*100)

	// Track hashes for cleanup
	var hashesToClean []string

	// Find media that's in the database but not on disk
	for _, m := range allMedia {
		if !foundFiles[m.FilePath] {
			// This media file no longer exists on disk - delete it from database

			// Collect image hashes for this media before deletion
			mediaHashes := CollectImageHashesForMedia(ctx, uc.imageRepo, m.ID)
			hashesToClean = append(hashesToClean, mediaHashes...)

			// Delete the media database record (cascades to images, transcode jobs, etc.)
			if err := uc.mediaRepo.Delete(ctx, m.ID); err != nil {
				fmt.Printf("warning: failed to delete stale media %d (%s): %v\n", m.ID, m.FilePath, err)
			} else {
				fmt.Printf("info: removed stale media from library: %s\n", m.FilePath)
			}
		}
	}

	// Clean up image cache files for all the removed media
	if uc.imageCleanup != nil && len(hashesToClean) > 0 {
		if err := uc.imageCleanup.CleanCacheForHashes(ctx, hashesToClean); err != nil {
			fmt.Printf("warning: failed to clean image cache during library scan: %v\n", err)
		}
	}
}

// extractTVShowAndSeasonImages extracts images for a TV show and season
// This is a helper to avoid code duplication between create and update paths
func (uc *ScanLibraryUseCase) extractTVShowAndSeasonImages(ctx context.Context, showTitle string, libraryID int64, showDir string, seasonNumber int) {
	// Get show ID by title (show was created/ensured by CreateTVEpisode)
	show, err := uc.tvRepo.GetTVShowByTitle(ctx, libraryID, showTitle)
	if err != nil {
		fmt.Printf("failed to get TV show for image extraction: %v\n", err)
		return
	}

	// Extract show images
	if uc.extractShowImages != nil {
		if err := uc.extractShowImages.Execute(ctx, showDir, images.MediaTypeTVShow, int(show.ID)); err != nil {
			fmt.Printf("failed to extract images for show %s: %v\n", showTitle, err)
		}
	}

	// Get season ID
	season, err := uc.tvRepo.GetTVSeasonByShowAndNumber(ctx, show.ID, int64(seasonNumber))
	if err != nil {
		fmt.Printf("failed to get TV season for image extraction: %v\n", err)
		return
	}

	// Extract season images
	if uc.extractSeasonImages != nil {
		if err := uc.extractSeasonImages.Execute(ctx, showDir, seasonNumber, images.MediaTypeTVSeason, int(season.ID)); err != nil {
			fmt.Printf("failed to extract images for season %d: %v\n", seasonNumber, err)
		}
	}
}

// tryMarkArtistProcessed atomically checks if an artist has been processed and marks it as processed
// Returns true if this is the first time the artist is being processed (caller should extract images)
// Returns false if the artist was already processed (caller should skip extraction)
// This uses LoadOrStore for atomic check-and-set to prevent race conditions
func (uc *ScanLibraryUseCase) tryMarkArtistProcessed(artistName string) bool {
	_, alreadyProcessed := uc.processedArtists.LoadOrStore(artistName, true)
	return !alreadyProcessed // Return true if this is the first time
}

// extractImagesForMovie extracts and catalogs images for a movie
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForMovie(ctx context.Context, movie *media.Movie, filePath string) {
	if uc.extractMovieImages == nil {
		return
	}

	mediaID := int(movie.Media.ID)
	if err := uc.extractMovieImages.Execute(ctx, filePath, images.MediaTypeMovie, mediaID, &mediaID); err != nil {
		fmt.Printf("failed to extract images for movie %s: %v\n", filePath, err)
	}
}

// extractImagesForEpisode extracts images for a TV episode, its show, and season
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForEpisode(ctx context.Context, episode *media.TVEpisode, filePath string, libraryID int64) {
	// Extract episode images
	if uc.extractEpisodeImages != nil {
		mediaID := int(episode.Media.ID)
		if err := uc.extractEpisodeImages.Execute(ctx, filePath, images.MediaTypeTVEpisode, mediaID, &mediaID); err != nil {
			fmt.Printf("failed to extract images for episode %s: %v\n", filePath, err)
		}
	}

	// Extract show and season images
	showDir := filepath.Dir(filepath.Dir(filePath))
	uc.extractTVShowAndSeasonImages(ctx, episode.ShowTitle, libraryID, showDir, episode.Season)
}

// extractImagesForTrack extracts images for a music track (album and artist)
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForTrack(ctx context.Context, track *media.MusicTrack, filePath string) {
	// Extract album images
	if uc.extractMusicImages != nil && track.Album != "" {
		albumDir := filepath.Dir(filePath)
		entityID := int(track.Media.ID)
		if err := uc.extractMusicImages.Execute(ctx, albumDir, images.MediaTypeMusicAlbum, entityID); err != nil {
			fmt.Printf("failed to extract images for album %s: %v\n", track.Album, err)
		}
	}

	// Extract artist images (once per artist)
	if uc.extractArtistImages != nil && track.Artist != "" {
		artistDir := filepath.Dir(filepath.Dir(filePath)) // Parent of album dir
		entityID := int(track.Media.ID)

		// Atomically check and mark artist as processed (prevents race condition)
		if uc.tryMarkArtistProcessed(track.Artist) {
			if err := uc.extractArtistImages.Execute(ctx, artistDir, images.MediaTypeMusicArtist, entityID); err != nil {
				fmt.Printf("failed to extract artist images for %s: %v\n", track.Artist, err)
			}
		}
	}
}
