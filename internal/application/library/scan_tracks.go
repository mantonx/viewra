package library

import (
	"context"
	"path/filepath"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// persistMediaTracks saves audio and subtitle track metadata for a media item.
// It deletes existing tracks and inserts new ones from the scan result,
// then discovers and persists external subtitle files.
func (uc *ScanLibraryUseCase) persistMediaTracks(ctx context.Context, mediaID int64, result *scanner.ScanResult) {
	// Delete existing tracks (we'll re-add them)
	if err := uc.mediaRepos.Media.DeleteAudioTracksByMediaID(ctx, mediaID); err != nil {
		uc.logger.Warn("Failed to delete existing audio tracks",
			"media_id", mediaID,
			"error", err)
	}

	if err := uc.mediaRepos.Media.DeleteSubtitleTracksByMediaID(ctx, mediaID); err != nil {
		uc.logger.Warn("Failed to delete existing subtitle tracks",
			"media_id", mediaID,
			"error", err)
	}

	// Insert audio tracks from embedded streams
	for _, t := range result.AudioTracks {
		audioTrack := &media.AudioTrack{
			MediaID:       mediaID,
			StreamIndex:   t.StreamIndex,
			Codec:         t.Codec,
			CodecProfile:  t.CodecProfile,
			Channels:      t.Channels,
			ChannelLayout: t.ChannelLayout,
			SampleRate:    t.SampleRate,
			BitRate:       t.BitRate,
			Language:      t.Language,
			Title:         t.Title,
			IsDefault:     t.IsDefault,
			IsCommentary:  t.IsCommentary,
			IsDescriptive: t.IsDescriptive,
		}
		if err := uc.mediaRepos.Media.InsertAudioTrack(ctx, audioTrack); err != nil {
			uc.logger.Warn("Failed to insert audio track",
				"media_id", mediaID,
				"stream_index", t.StreamIndex,
				"error", err)
		}
	}

	// Insert embedded subtitle tracks
	for _, t := range result.SubtitleTracks {
		streamIndex := t.StreamIndex
		subtitleTrack := &media.SubtitleTrack{
			MediaID:      mediaID,
			StreamIndex:  &streamIndex,
			SourceType:   media.SubtitleSourceEmbedded,
			Codec:        t.Codec,
			Language:     t.Language,
			Title:        t.Title,
			IsDefault:    t.IsDefault,
			IsForced:     t.IsForced,
			IsSDH:        t.IsSDH,
			IsCommentary: t.IsCommentary,
			IsBitmap:     t.IsBitmap,
		}
		if err := uc.mediaRepos.Media.InsertSubtitleTrack(ctx, subtitleTrack); err != nil {
			uc.logger.Warn("Failed to insert subtitle track",
				"media_id", mediaID,
				"stream_index", t.StreamIndex,
				"error", err)
		}
	}

	// Discover and persist external subtitle files
	uc.discoverExternalSubtitles(ctx, mediaID, result.FilePath)
}

// discoverExternalSubtitles finds and persists external subtitle files for a media item.
func (uc *ScanLibraryUseCase) discoverExternalSubtitles(ctx context.Context, mediaID int64, videoPath string) {
	// Get the library to resolve the full path
	mediaItem, err := uc.mediaRepos.Media.GetByID(ctx, mediaID)
	if err != nil {
		uc.logger.Warn("Failed to get media for subtitle discovery",
			"media_id", mediaID,
			"error", err)
		return
	}

	// Get library path to build full video path
	library, err := uc.mediaRepos.Library.GetByID(ctx, mediaItem.LibraryID)
	if err != nil {
		uc.logger.Warn("Failed to get library for subtitle discovery",
			"library_id", mediaItem.LibraryID,
			"error", err)
		return
	}

	fullVideoPath := filepath.Join(library.Path, mediaItem.FilePath)

	// Discover external subtitles
	externalSubs := filesystem.DiscoverAllExternalSubtitles(fullVideoPath)
	if len(externalSubs) == 0 {
		return
	}

	// Persist each external subtitle
	for _, sub := range externalSubs {
		// Convert absolute path to relative path from library root
		relPath, err := filepath.Rel(library.Path, sub.FilePath)
		if err != nil {
			uc.logger.Warn("Failed to compute relative path for external subtitle",
				"subtitle_path", sub.FilePath,
				"library_path", library.Path,
				"error", err)
			continue
		}

		subtitleTrack := &media.SubtitleTrack{
			MediaID:      mediaID,
			SourceType:   media.SubtitleSourceExternal,
			Codec:        sub.Codec,
			Language:     sub.Language,
			Title:        sub.Title,
			FilePath:     &relPath,
			IsDefault:    false, // External subtitles are not default by default
			IsForced:     sub.IsForced,
			IsSDH:        sub.IsSDH,
			IsCommentary: sub.Title == "Commentary",
			IsBitmap:     false, // External subtitles are typically text-based
		}

		if err := uc.mediaRepos.Media.InsertSubtitleTrack(ctx, subtitleTrack); err != nil {
			uc.logger.Warn("Failed to insert external subtitle track",
				"media_id", mediaID,
				"subtitle_path", relPath,
				"error", err)
		}
	}
}
