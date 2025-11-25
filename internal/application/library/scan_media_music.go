package library

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/music"
)

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

		// Look up or create artist entity and set artist_id
		if artist != "" {
			existingArtist, err := uc.mediaRepos.Music.FindArtistByName(ctx, libraryID, artist)
			if err == nil && existingArtist != nil {
				track.ArtistID = existingArtist.ID
			} else {
				// Create new artist if it doesn't exist
				artistEntity := &media.Artist{
					LibraryID:           libraryID,
					Name:                artist,
					MusicBrainzArtistID: musicBrainzArtistID,
					Genre:               genre,
				}
				if createErr := uc.mediaRepos.Music.CreateArtist(ctx, artistEntity); createErr == nil {
					track.ArtistID = artistEntity.ID
				}
			}
		}

		// Look up or create album entity and set album_id
		if album != "" {
			effectiveAlbumArtist := albumArtist
			if effectiveAlbumArtist == "" {
				effectiveAlbumArtist = artist
			}
			existingAlbum, err := uc.mediaRepos.Music.FindAlbumByTitle(ctx, libraryID, album, effectiveAlbumArtist)
			if err == nil && existingAlbum != nil {
				track.AlbumID = existingAlbum.ID
			} else {
				// Create new album if it doesn't exist
				albumEntity := &media.Album{
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
					ArtistID:           track.ArtistID, // Link album to artist if we just created one
				}
				if createErr := uc.mediaRepos.Music.CreateAlbum(ctx, albumEntity); createErr == nil {
					track.AlbumID = albumEntity.ID
				}
			}
		}

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

			// Look up or create artist entity and set artist_id
			if artist != "" {
				existingArtist, findErr := uc.mediaRepos.Music.FindArtistByName(ctx, libraryID, artist)
				if findErr == nil && existingArtist != nil {
					track.ArtistID = existingArtist.ID
				} else {
					// Create new artist if it doesn't exist
					newArtist := &media.Artist{
						LibraryID:           libraryID,
						Name:                artist,
						MusicBrainzArtistID: musicBrainzArtistID,
						Genre:               genre,
					}
					if createErr := uc.mediaRepos.Music.CreateArtist(ctx, newArtist); createErr == nil {
						track.ArtistID = newArtist.ID
					}
				}
			}

			// Look up or create album entity and set album_id
			if album != "" {
				effectiveAlbumArtist := albumArtist
				if effectiveAlbumArtist == "" {
					effectiveAlbumArtist = artist
				}
				existingAlbum, findErr := uc.mediaRepos.Music.FindAlbumByTitle(ctx, libraryID, album, effectiveAlbumArtist)
				if findErr == nil && existingAlbum != nil {
					track.AlbumID = existingAlbum.ID
				} else {
					// Create new album if it doesn't exist
					newAlbum := &media.Album{
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
						ArtistID:           track.ArtistID, // Link album to artist if we just created one
					}
					if createErr := uc.mediaRepos.Music.CreateAlbum(ctx, newAlbum); createErr == nil {
						track.AlbumID = newAlbum.ID
					}
				}
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
