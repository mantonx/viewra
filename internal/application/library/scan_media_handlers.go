package library

import (
	"context"
	"fmt"
	"time"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/domain/scanner/parsers"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/music"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/nfo"
)

// processMovie creates or updates a movie entry
func (uc *ScanLibraryUseCase) processMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint) *int64 {
	// Handle nil checkpoint (legacy code path or tests)
	if checkpoint == nil {
		checkpoint = &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileHash: "",
			FileSize: 0,
		}
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

	// Check if movie already exists
	existing, err := uc.mediaRepos.Media.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		movie.Media.ID = existing.ID
		movie.Media.Type = "movie"
		if err := uc.mediaRepos.Media.Update(ctx, &movie.Media); err != nil {
			fmt.Printf("failed to update media %s: %v\n", result.FilePath, err)
		}
		if err := uc.mediaRepos.Movie.UpdateMovie(ctx, movie); err != nil {
			fmt.Printf("failed to update movie metadata %s: %v\n", result.FilePath, err)
		}
		// Extract and catalog images (even for existing movies to populate cache)
		uc.extractImagesForMovie(ctx, movie, result.FilePath)
		return &movie.Media.ID
	}

	// Create new entry - let movie repository handle both media and movie records
	movie.Media.Type = "movie"
	if err := uc.mediaRepos.Movie.CreateMovie(ctx, movie); err != nil {
		fmt.Printf("failed to create movie %s: %v\n", result.FilePath, err)
		return nil
	}

	// Extract and catalog images for the movie
	uc.extractImagesForMovie(ctx, movie, result.FilePath)
	return &movie.Media.ID
}

// processTVEpisode creates or updates a TV episode entry
func (uc *ScanLibraryUseCase) processTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint) *int64 {
	// Handle nil checkpoint (legacy code path or tests)
	if checkpoint == nil {
		checkpoint = &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileHash: "",
			FileSize: 0,
		}
	}

	// Coordinator already parsed season/episode/title, but we need show name which isn't in ScanResult
	parser := parsers.NewDefaultParser()
	tvInfo, err := parser.ParseTVEpisode(result.FilePath)
	if err != nil || tvInfo == nil {
		fmt.Printf("failed to parse TV episode filename %s: %v\n", result.FilePath, err)
		return nil // Can't create episode without show name
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

	// Check if episode already exists
	existing, err := uc.mediaRepos.Media.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		episode.Media.ID = existing.ID
		episode.Media.Type = "tv_episode"
		if err := uc.mediaRepos.Media.Update(ctx, &episode.Media); err != nil {
			fmt.Printf("failed to update media %s: %v\n", result.FilePath, err)
		}
		if err := uc.mediaRepos.TV.UpdateTVEpisode(ctx, episode); err != nil {
			fmt.Printf("failed to update TV episode metadata %s: %v\n", result.FilePath, err)
		}
		// Extract and catalog images (even for existing episodes to populate cache)
		uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
		return &episode.Media.ID
	}

	// Create new entry - let TV repository handle both media and episode records
	episode.Media.Type = "tv_episode"
	if err := uc.mediaRepos.TV.CreateTVEpisode(ctx, episode); err != nil {
		fmt.Printf("failed to create TV episode %s: %v\n", result.FilePath, err)
		return nil
	}

	// Extract and catalog images for the episode, show, and season
	uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
	return &episode.Media.ID
}

// processMusicTrack creates or updates a music track entry
func (uc *ScanLibraryUseCase) processMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint) *int64 {
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

	// Prepare extended metadata fields
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

	// Get extended metadata if we have the extractor result
	if extractor := music.NewExtractor(); extractor != nil {
		if musicInfo, err := extractor.ExtractMetadata(result.FilePath); err == nil && musicInfo != nil {
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

	// Check if track already exists
	existing, err := uc.mediaRepos.Media.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		track.Media.ID = existing.ID
		if err := uc.mediaRepos.Media.Update(ctx, &track.Media); err != nil {
			fmt.Printf("failed to update media %s: %v\n", result.FilePath, err)
		}
		if err := uc.mediaRepos.Music.UpdateMusicTrack(ctx, track); err != nil {
			fmt.Printf("failed to update music track metadata %s: %v\n", result.FilePath, err)
		}
		// Extract album and artist images (even for existing tracks to populate cache)
		uc.extractImagesForTrack(ctx, track, result.FilePath)
		return &track.Media.ID
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
		fmt.Printf("failed to create music track %s: %v\n", result.FilePath, err)
		return nil
	}

	// Extract and catalog images for the album and artist
	uc.extractImagesForTrack(ctx, track, result.FilePath)
	return &track.Media.ID
}
