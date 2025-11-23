package library

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/domain/scanner/parsers"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/music"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/nfo"
)

// processMovie creates or updates a movie entry
func (uc *ScanLibraryUseCase) processMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, existingMediaCache *sync.Map) (*int64, error) {
	// Handle nil checkpoint (legacy code path or tests)
	if checkpoint == nil {
		checkpoint = &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileHash: "",
			FileSize: 0,
		}
	}

	// Skip audio files in Movie libraries - they can't be movies (e.g., soundtrack files)
	// Audio files should only be processed in Music libraries
	ext := strings.ToLower(strings.TrimPrefix(result.FilePath[strings.LastIndex(result.FilePath, "."):], "."))
	audioExts := map[string]bool{
		"mp3": true, "flac": true, "m4a": true, "aac": true, "ogg": true, "opus": true,
		"wav": true, "wma": true, "ape": true, "wv": true, "tta": true, "tak": true,
		"dsf": true, "dff": true, "alac": true, "aiff": true, "aif": true,
	}
	if audioExts[ext] {
		// Return nil media ID but no error - this file is intentionally skipped
		return nil, nil
	}

	// Coordinator already parsed the filename - just use the results
	movie := &media.Movie{
		Media: media.Media{
			LibraryID:       libraryID,
			Title:           result.Title,
			FilePath:        result.FilePath,
			FileSize:        checkpoint.FileSize,
			FileHash:        checkpoint.FileHash,
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

	// Check if movie already exists using in-memory cache (major performance optimization!)
	// This eliminates individual database SELECTs for every file
	if value, found := existingMediaCache.Load(result.FilePath); found {
		// Update existing entry
		movie.Media.ID = value.(int64)
		movie.Media.Type = "movie"
		if err := uc.mediaRepos.Media.Update(ctx, &movie.Media); err != nil {
			return nil, fmt.Errorf("failed to update base media record: %w", err)
		}
		if err := uc.mediaRepos.Movie.UpdateMovie(ctx, movie); err != nil {
			return nil, fmt.Errorf("failed to update movie metadata: %w", err)
		}
		// Extract and catalog images (even for existing movies to populate cache)
		uc.extractImagesForMovie(ctx, movie, result.FilePath)
		return &movie.Media.ID, nil
	}

	// Create new entry - let movie repository handle both media and movie records
	movie.Media.Type = "movie"
	if err := uc.mediaRepos.Movie.CreateMovie(ctx, movie); err != nil {
		// Handle race condition: Another worker may have created this movie between our check and insert
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "duplicate key") {
			// Check cache again (another worker may have just added it)
			if value, found := existingMediaCache.Load(result.FilePath); found {
				// Update the existing record
				movie.Media.ID = value.(int64)
			} else {
				// Cache miss - fetch from database (race condition: created after our initial cache load)
				existing, fetchErr := uc.mediaRepos.Media.GetByFilePath(ctx, libraryID, result.FilePath)
				if fetchErr != nil || existing == nil {
					return nil, fmt.Errorf("failed to fetch existing media after collision: %w", fetchErr)
				}
				movie.Media.ID = existing.ID
				// Add to cache for future lookups
				existingMediaCache.Store(result.FilePath, existing.ID)
			}

			// Update the existing record
			movie.Media.Type = "movie"
			if updateErr := uc.mediaRepos.Media.Update(ctx, &movie.Media); updateErr != nil {
				return nil, fmt.Errorf("failed to update base media record after collision: %w", updateErr)
			}
			if updateErr := uc.mediaRepos.Movie.UpdateMovie(ctx, movie); updateErr != nil {
				return nil, fmt.Errorf("failed to update movie metadata after collision: %w", updateErr)
			}
			uc.extractImagesForMovie(ctx, movie, result.FilePath)
			return &movie.Media.ID, nil
		}
		return nil, fmt.Errorf("failed to create base media record: %w", err)
	}

	// Add newly created media to cache so other workers don't try to create it again
	existingMediaCache.Store(result.FilePath, movie.Media.ID)

	// Extract and catalog images for the movie
	uc.extractImagesForMovie(ctx, movie, result.FilePath)
	return &movie.Media.ID, nil
}

// processTVEpisode creates or updates a TV episode entry
func (uc *ScanLibraryUseCase) processTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, existingMediaCache *sync.Map) (*int64, error) {
	// Handle nil checkpoint (legacy code path or tests)
	if checkpoint == nil {
		checkpoint = &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileHash: "",
			FileSize: 0,
		}
	}

	// Skip audio files in TV libraries - they can't be episodes (e.g., theme.mp3, soundtrack files)
	// Audio files should only be processed in Music libraries
	ext := strings.ToLower(strings.TrimPrefix(result.FilePath[strings.LastIndex(result.FilePath, "."):], "."))
	audioExts := map[string]bool{
		"mp3": true, "flac": true, "m4a": true, "aac": true, "ogg": true, "opus": true,
		"wav": true, "wma": true, "ape": true, "wv": true, "tta": true, "tak": true,
		"dsf": true, "dff": true, "alac": true, "aiff": true, "aif": true,
	}
	if audioExts[ext] {
		// Return nil media ID but no error - this file is intentionally skipped
		return nil, nil
	}

	// Use coordinator's parsed data directly (no duplicate parsing needed!)
	showTitle := result.ShowName
	season := 0
	episodeNumber := 0
	episodeTitle := result.Title

	// Use coordinator's parsed season/episode numbers
	if result.SeasonNumber != nil {
		season = *result.SeasonNumber
	}

	if result.EpisodeNumber != nil {
		episodeNumber = *result.EpisodeNumber
	}

	// Fallback: If coordinator didn't populate show name, parse as last resort
	// This should rarely happen, but handles edge cases
	if showTitle == "" {
		parser := parsers.NewDefaultParser()
		tvInfo, err := parser.ParseTVEpisode(result.FilePath)
		if err != nil || tvInfo == nil {
			return nil, fmt.Errorf("failed to parse TV episode filename: %w", err)
		}
		showTitle = tvInfo.ShowName
		if season == 0 {
			season = tvInfo.Season
		}
		if episodeNumber == 0 {
			episodeNumber = tvInfo.Episode
		}
		if episodeTitle == "" {
			episodeTitle = tvInfo.EpisodeTitle
		}
	}

	episode := &media.TVEpisode{
		Media: media.Media{
			LibraryID:       libraryID,
			Title:           episodeTitle,
			FilePath:        result.FilePath,
			FileSize:        checkpoint.FileSize,
			FileHash:        checkpoint.FileHash,
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

	// Check if episode already exists using in-memory cache (major performance optimization!)
	// This eliminates individual database SELECTs for every file
	if value, found := existingMediaCache.Load(result.FilePath); found {
		// Update existing entry
		episode.Media.ID = value.(int64)
		episode.Media.Type = "tv_episode"
		if err := uc.mediaRepos.Media.Update(ctx, &episode.Media); err != nil {
			return nil, fmt.Errorf("failed to update base media record: %w", err)
		}
		if err := uc.mediaRepos.TV.UpdateTVEpisode(ctx, episode); err != nil {
			return nil, fmt.Errorf("failed to update TV episode metadata: %w", err)
		}
		// Extract and catalog images (even for existing episodes to populate cache)
		uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
		return &episode.Media.ID, nil
	}

	// Create new entry - let TV repository handle both media and episode records
	episode.Media.Type = "tv_episode"
	if err := uc.mediaRepos.TV.CreateTVEpisode(ctx, episode); err != nil {
		// Handle race condition: Another worker may have created this episode between our check and insert
		// TV episodes have a UNIQUE constraint on (show_id, season_number, episode_number)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "duplicate key") {
			// Check cache again (another worker may have just added it)
			if value, found := existingMediaCache.Load(result.FilePath); found {
				// Update the existing record
				episode.Media.ID = value.(int64)
			} else {
				// Cache miss - fetch from database (race condition: created after our initial cache load)
				existing, fetchErr := uc.mediaRepos.Media.GetByFilePath(ctx, libraryID, result.FilePath)
				if fetchErr != nil || existing == nil {
					return nil, fmt.Errorf("failed to fetch existing media after collision: %w", fetchErr)
				}
				episode.Media.ID = existing.ID
				// Add to cache for future lookups
				existingMediaCache.Store(result.FilePath, existing.ID)
			}

			// Update the existing record
			episode.Media.Type = "tv_episode"
			if updateErr := uc.mediaRepos.Media.Update(ctx, &episode.Media); updateErr != nil {
				return nil, fmt.Errorf("failed to update base media record after collision: %w", updateErr)
			}
			if updateErr := uc.mediaRepos.TV.UpdateTVEpisode(ctx, episode); updateErr != nil {
				return nil, fmt.Errorf("failed to update TV episode metadata after collision: %w", updateErr)
			}
			uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
			return &episode.Media.ID, nil
		}
		return nil, fmt.Errorf("failed to create base media record: %w", err)
	}

	// Add newly created media to cache so other workers don't try to create it again
	existingMediaCache.Store(result.FilePath, episode.Media.ID)

	// Extract and catalog images for the episode, show, and season
	uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
	return &episode.Media.ID, nil
}

// processMusicTrack creates or updates a music track entry
func (uc *ScanLibraryUseCase) processMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, existingMediaCache *sync.Map) (*int64, error) {
	// Handle nil checkpoint (legacy code path or tests)
	if checkpoint == nil {
		checkpoint = &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileHash: "",
			FileSize: 0,
		}
	}

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

	// Extract ID3 metadata ONCE (was being done twice - major bottleneck!)
	// This handles fields like album artist, disc number, genre, and extended metadata
	// that aren't in the coordinator's ScanResult
	var (
		totalTracks         int
		totalDiscs          int
		releaseDate         string
		lyricist            string
		isrc                string
		releaseType         string
		compilation         bool
		originalTitle       string
		publisher           string
		musicBrainzTrackID  string
		musicBrainzAlbumID  string
		musicBrainzArtistID string
		composer            string
	)

	// Single metadata extraction for both basic and extended fields
	if artist == "" || album == "" || genre == "" || albumArtist == "" {
		extractor := music.NewExtractor()
		musicInfo, err := extractor.ExtractMetadata(result.FilePath)
		if err == nil && musicInfo != nil {
			// Fill in missing basic fields from ID3 tags
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

			// Extract extended metadata from the same result (no second read!)
			totalTracks = musicInfo.TotalTracks
			totalDiscs = musicInfo.TotalDiscs
			releaseDate = musicInfo.ReleaseDate
			lyricist = musicInfo.Lyricist
			isrc = musicInfo.ISRC
			releaseType = musicInfo.ReleaseType
			compilation = musicInfo.Compilation
			originalTitle = musicInfo.OriginalTitle
			publisher = musicInfo.Publisher
			musicBrainzTrackID = musicInfo.MusicBrainzTrackID
			musicBrainzAlbumID = musicInfo.MusicBrainzAlbumID
			musicBrainzArtistID = musicInfo.MusicBrainzArtistID
			composer = musicInfo.Composer
		}
	}

	track := &media.MusicTrack{
		Media: media.Media{
			LibraryID:       libraryID,
			Title:           title,
			FilePath:        result.FilePath,
			FileSize:        checkpoint.FileSize,
			FileHash:        checkpoint.FileHash,
			Duration:        int(result.Duration),
			IsExtra:         isExtra(result.FilePath),
			AudioCodec:      result.AudioCodec,
			Bitrate:         result.Bitrate,
			ContainerFormat: result.ContainerFormat,
			Type:            "music_track",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		// Basic metadata
		Artist:      artist,
		Album:       album,
		AlbumArtist: albumArtist,
		TrackNumber: trackNumber,
		DiscNumber:  discNumber,
		Genre:       genre,
		Year:        year,
		Composer:    composer,
		// Extended metadata
		TotalTracks:         totalTracks,
		TotalDiscs:          totalDiscs,
		ReleaseDate:         releaseDate,
		Lyricist:            lyricist,
		ISRC:                isrc,
		ReleaseType:         releaseType,
		Compilation:         compilation,
		OriginalTitle:       originalTitle,
		Publisher:           publisher,
		MusicBrainzTrackID:  musicBrainzTrackID,
		MusicBrainzAlbumID:  musicBrainzAlbumID,
		MusicBrainzArtistID: musicBrainzArtistID,
	}

	// Check if track already exists using in-memory cache (major performance optimization!)
	// This eliminates individual database SELECTs for every file
	if value, found := existingMediaCache.Load(result.FilePath); found {
		// Update existing entry
		track.Media.ID = value.(int64)
		if err := uc.mediaRepos.Media.Update(ctx, &track.Media); err != nil {
			return nil, fmt.Errorf("failed to update base media record: %w", err)
		}
		if err := uc.mediaRepos.Music.UpdateMusicTrack(ctx, track); err != nil {
			return nil, fmt.Errorf("failed to update music track metadata: %w", err)
		}
		// Extract album and artist images (even for existing tracks to populate cache)
		uc.extractImagesForTrack(ctx, track, result.FilePath)
		return &track.Media.ID, nil
	}

	// Prepare artist entity if artist info is available
	var artistEntity *media.Artist
	if artist != "" {
		artistEntity = &media.Artist{
			LibraryID:           libraryID,
			Name:                artist,
			MusicBrainzArtistID: musicBrainzArtistID,
			Genre:               genre,
		}
	}

	// Prepare album entity if album info is available
	var albumEntity *media.Album
	if album != "" {
		// Use album artist if available, otherwise fall back to track artist
		effectiveAlbumArtist := albumArtist
		if effectiveAlbumArtist == "" {
			effectiveAlbumArtist = artist
		}

		albumEntity = &media.Album{
			LibraryID:          libraryID,
			Title:              album,
			AlbumArtist:        effectiveAlbumArtist,
			Artist:             artist,
			Year:               year,
			ReleaseDate:        releaseDate,
			Genre:              genre,
			TotalTracks:        totalTracks,
			TotalDiscs:         totalDiscs,
			RecordLabel:        publisher,
			ReleaseType:        releaseType,
			Compilation:        compilation,
			MusicBrainzAlbumID: musicBrainzAlbumID,
		}
	}

	// Create track with artist and album entities in a single transaction
	// This ensures all-or-nothing semantics - no orphaned records on failure
	if err := uc.mediaRepos.Music.CreateMusicTrackWithEntities(ctx, track, artistEntity, albumEntity); err != nil {
		// Handle race condition: Another worker may have created this record between our check and insert
		// If we get a UNIQUE constraint error, retry with update logic
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "duplicate key") {
			// Check cache again (another worker may have just added it)
			if value, found := existingMediaCache.Load(result.FilePath); found {
				// Update the existing record
				track.Media.ID = value.(int64)
			} else {
				// Cache miss - fetch from database (race condition: created after our initial cache load)
				existing, fetchErr := uc.mediaRepos.Media.GetByFilePath(ctx, libraryID, result.FilePath)
				if fetchErr != nil || existing == nil {
					return nil, fmt.Errorf("failed to fetch existing media after collision: %w", fetchErr)
				}
				track.Media.ID = existing.ID
				// Add to cache for future lookups
				existingMediaCache.Store(result.FilePath, existing.ID)
			}

			// Update the existing record
			if updateErr := uc.mediaRepos.Media.Update(ctx, &track.Media); updateErr != nil {
				return nil, fmt.Errorf("failed to update base media record after collision: %w", updateErr)
			}
			if updateErr := uc.mediaRepos.Music.UpdateMusicTrack(ctx, track); updateErr != nil {
				return nil, fmt.Errorf("failed to update music track metadata after collision: %w", updateErr)
			}
			uc.extractImagesForTrack(ctx, track, result.FilePath)
			return &track.Media.ID, nil
		}
		return nil, fmt.Errorf("failed to create base media record: %w", err)
	}

	// Add newly created media to cache so other workers don't try to create it again
	existingMediaCache.Store(result.FilePath, track.Media.ID)

	// Extract and catalog images for the album and artist
	uc.extractImagesForTrack(ctx, track, result.FilePath)
	return &track.Media.ID, nil
}
