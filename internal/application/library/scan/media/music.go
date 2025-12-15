package media

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/music"
)

// ProcessMusicTrack creates or updates a music track entry.
func ProcessMusicTrack(
	ctx context.Context,
	deps *Deps,
	libraryID int64,
	result *scanner.ScanResult,
	checkpoint *scanner.ScanCheckpoint,
	existingMediaCache *sync.Map,
) (*int64, error) {
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
			IsExtra:         scanutil.IsExtra(result.FilePath),
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

	// Helper to resolve artist/album entities (used both in update and race condition recovery paths)
	resolveEntities := func(ctx context.Context) {
		// Look up or create artist entity
		if artist != "" {
			existingArtist, err := deps.MediaRepos.Music.FindArtistByName(ctx, libraryID, artist)
			if err == nil && existingArtist != nil {
				track.ArtistID = existingArtist.ID
			} else {
				artistEntity := &media.Artist{
					LibraryID:           libraryID,
					Name:                artist,
					MusicBrainzArtistID: musicBrainzArtistID,
					Genre:               genre,
				}
				if createErr := deps.MediaRepos.Music.CreateArtist(ctx, artistEntity); createErr == nil {
					track.ArtistID = artistEntity.ID
				}
			}
		}

		// Look up or create album entity
		if album != "" {
			effectiveAlbumArtist := albumArtist
			if effectiveAlbumArtist == "" {
				effectiveAlbumArtist = artist
			}
			existingAlbum, err := deps.MediaRepos.Music.FindAlbumByTitle(ctx, libraryID, album, effectiveAlbumArtist)
			if err == nil && existingAlbum != nil {
				track.AlbumID = existingAlbum.ID
			} else {
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
					ArtistID:           track.ArtistID,
				}
				if createErr := deps.MediaRepos.Music.CreateAlbum(ctx, albumEntity); createErr == nil {
					track.AlbumID = albumEntity.ID
				}
			}
		}
	}

	// Prepare artist/album entities for create path (transaction-based creation)
	var artistEntity *media.Artist
	if artist != "" {
		artistEntity = &media.Artist{
			LibraryID:           libraryID,
			Name:                artist,
			MusicBrainzArtistID: musicBrainzArtistID,
			Genre:               genre,
		}
	}

	var albumEntity *media.Album
	if album != "" {
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

	// Use shared cache-based upsert pattern with race condition handling
	return ProcessMediaWithCache(ctx, deps, libraryID, result.FilePath, existingMediaCache, UpsertCallbacks{
		GetMediaID: func() int64 { return track.Media.ID },
		SetMediaID: func(id int64) { track.Media.ID = id },
		Update: func(ctx context.Context) error {
			// Resolve artist/album entities for update path
			resolveEntities(ctx)
			if err := deps.MediaRepos.Media.Update(ctx, &track.Media); err != nil {
				return fmt.Errorf("failed to update base media record: %w", err)
			}
			if err := deps.MediaRepos.Music.UpdateMusicTrack(ctx, track); err != nil {
				return fmt.Errorf("failed to update music track metadata: %w", err)
			}
			return nil
		},
		Create: func(ctx context.Context) error {
			// Create track with artist and album entities in a single transaction
			if err := deps.MediaRepos.Music.CreateMusicTrackWithEntities(ctx, track, artistEntity, albumEntity); err != nil {
				return fmt.Errorf("failed to create music track: %w", err)
			}
			return nil
		},
		PostSave: func(ctx context.Context) {
			ExtractImagesForTrack(ctx, deps, track, result.FilePath)
			// Enqueue for enrichment if pipeline is configured
			enqueueForEnrichment(ctx, deps, track.Media.ID, enrichment.MediaTypeMusic)
		},
	})
}
