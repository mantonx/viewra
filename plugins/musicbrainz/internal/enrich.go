package internal

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// enrichTrack fetches metadata for a music track from MusicBrainz.
func (p *MusicBrainzPlugin) enrichTrack(ctx context.Context, client *Client, req *pluginv1.EnrichRequest, minConfidence float32) (*pluginv1.EnrichResponse, error) {
	var recordingID string
	var releaseID string

	// Extract music metadata from request
	artist := ""
	album := ""
	trackTitle := req.Title
	if req.Music != nil {
		if req.Music.Artist != "" {
			artist = req.Music.Artist
		}
		if req.Music.Album != "" {
			album = req.Music.Album
		}
		if req.Music.TrackTitle != "" {
			trackTitle = req.Music.TrackTitle
		}
	}

	// Strategy 1: Use existing MusicBrainz ID if available
	if mbid, ok := req.ExistingIds["musicbrainz_track"]; ok && mbid != "" {
		recordingID = mbid
		p.logger.Debug("using existing MusicBrainz recording ID", "id", recordingID)
	}
	if mbid, ok := req.ExistingIds["musicbrainz_album"]; ok && mbid != "" {
		releaseID = mbid
	}

	// Strategy 2: Search by artist, track, and album
	if recordingID == "" && trackTitle != "" {
		p.logger.Debug("searching for recording", "artist", artist, "track", trackTitle, "album", album)
		searchResp, err := client.SearchRecordings(ctx, artist, trackTitle, album)
		if err != nil {
			p.recordError()
			return nil, fmt.Errorf("recording search failed: %w", err)
		}

		if len(searchResp.Recordings) > 0 {
			best := searchResp.Recordings[0]
			confidence := float32(best.Score) / 100.0

			if confidence >= minConfidence {
				recordingID = best.ID
				p.logger.Debug("found recording via search",
					"id", recordingID,
					"score", best.Score,
					"title", best.Title,
				)

				// Extract release ID from the best result if we don't have one
				if releaseID == "" && len(best.Releases) > 0 {
					releaseID = best.Releases[0].ID
				}
			} else {
				p.logger.Debug("best match below confidence threshold",
					"score", best.Score,
					"threshold", minConfidence*100,
				)
			}
		}
	}

	if recordingID == "" {
		return &pluginv1.EnrichResponse{
			Matched:    false,
			Skipped:    true,
			SkipReason: "no recording match found",
		}, nil
	}

	// Fetch full recording details
	recording, err := client.GetRecording(ctx, recordingID)
	if err != nil {
		p.recordError()
		return nil, fmt.Errorf("failed to get recording details: %w", err)
	}

	return p.buildTrackResponse(ctx, client, recording, releaseID)
}

// enrichAlbum fetches metadata for a music album from MusicBrainz.
func (p *MusicBrainzPlugin) enrichAlbum(ctx context.Context, client *Client, req *pluginv1.EnrichRequest, minConfidence float32) (*pluginv1.EnrichResponse, error) {
	var releaseID string

	// Extract album metadata
	artist := ""
	albumTitle := req.Title
	year := int(req.Year)
	if req.Music != nil {
		if req.Music.Artist != "" {
			artist = req.Music.Artist
		}
		if req.Music.Album != "" {
			albumTitle = req.Music.Album
		}
	}

	// Strategy 1: Use existing MusicBrainz ID
	if mbid, ok := req.ExistingIds["musicbrainz_album"]; ok && mbid != "" {
		releaseID = mbid
		p.logger.Debug("using existing MusicBrainz release ID", "id", releaseID)
	}

	// Strategy 2: Search by artist and album
	if releaseID == "" && albumTitle != "" {
		p.logger.Debug("searching for release", "artist", artist, "album", albumTitle, "year", year)
		searchResp, err := client.SearchReleases(ctx, artist, albumTitle, year)
		if err != nil {
			p.recordError()
			return nil, fmt.Errorf("release search failed: %w", err)
		}

		if len(searchResp.Releases) > 0 {
			best := searchResp.Releases[0]
			confidence := float32(best.Score) / 100.0

			if confidence >= minConfidence {
				releaseID = best.ID
				p.logger.Debug("found release via search",
					"id", releaseID,
					"score", best.Score,
					"title", best.Title,
				)
			}
		}
	}

	if releaseID == "" {
		return &pluginv1.EnrichResponse{
			Matched:    false,
			Skipped:    true,
			SkipReason: "no album match found",
		}, nil
	}

	// Fetch full release details
	release, err := client.GetRelease(ctx, releaseID)
	if err != nil {
		p.recordError()
		return nil, fmt.Errorf("failed to get release details: %w", err)
	}

	return p.buildAlbumResponse(ctx, client, release)
}

// enrichArtist fetches metadata for a music artist from MusicBrainz.
func (p *MusicBrainzPlugin) enrichArtist(ctx context.Context, client *Client, req *pluginv1.EnrichRequest, minConfidence float32) (*pluginv1.EnrichResponse, error) {
	var artistID string

	// Extract artist name
	artistName := req.Title
	if req.Music != nil && req.Music.Artist != "" {
		artistName = req.Music.Artist
	}

	// Strategy 1: Use existing MusicBrainz ID
	if mbid, ok := req.ExistingIds["musicbrainz_artist"]; ok && mbid != "" {
		artistID = mbid
		p.logger.Debug("using existing MusicBrainz artist ID", "id", artistID)
	}

	// Strategy 2: Search by name
	if artistID == "" && artistName != "" {
		p.logger.Debug("searching for artist", "name", artistName)
		searchResp, err := client.SearchArtists(ctx, artistName)
		if err != nil {
			p.recordError()
			return nil, fmt.Errorf("artist search failed: %w", err)
		}

		if len(searchResp.Artists) > 0 {
			best := searchResp.Artists[0]
			confidence := float32(best.Score) / 100.0

			if confidence >= minConfidence {
				artistID = best.ID
				p.logger.Debug("found artist via search",
					"id", artistID,
					"score", best.Score,
					"name", best.Name,
				)
			}
		}
	}

	if artistID == "" {
		return &pluginv1.EnrichResponse{
			Matched:    false,
			Skipped:    true,
			SkipReason: "no artist match found",
		}, nil
	}

	// Fetch full artist details
	artist, err := client.GetArtist(ctx, artistID)
	if err != nil {
		p.recordError()
		return nil, fmt.Errorf("failed to get artist details: %w", err)
	}

	return p.buildArtistResponse(artist), nil
}

// buildTrackResponse builds an EnrichResponse for a recording.
func (p *MusicBrainzPlugin) buildTrackResponse(ctx context.Context, client *Client, recording *Recording, releaseID string) (*pluginv1.EnrichResponse, error) {
	resp := &pluginv1.EnrichResponse{
		Matched:       true,
		DiscoveredIds: make(map[string]string),
		Confidence:    0.85,
	}

	// External IDs
	resp.DiscoveredIds["musicbrainz_track"] = recording.ID
	if len(recording.ISRCs) > 0 {
		resp.DiscoveredIds["isrc"] = recording.ISRCs[0]
	}

	// Build metadata
	metadata := &pluginv1.EnrichedMetadata{
		Title: strPtr(recording.Title),
	}

	// Duration
	if recording.Length > 0 {
		runtimeMinutes := int32(recording.Length / 60000) // Convert ms to minutes
		metadata.RuntimeMinutes = &runtimeMinutes
	}

	// Artist from artist credit
	if len(recording.ArtistCredit) > 0 {
		artistName := formatArtistCredit(recording.ArtistCredit)
		metadata.Artist = strPtr(artistName)

		// Store the primary artist's MusicBrainz ID
		if recording.ArtistCredit[0].Artist != nil {
			resp.DiscoveredIds["musicbrainz_artist"] = recording.ArtistCredit[0].Artist.ID
		}
	}

	// Genre from tags
	if len(recording.Tags) > 0 {
		genre := getPrimaryGenre(recording.Tags)
		if genre != "" {
			metadata.Genres = []string{genre}
		}
	}

	// Get release info if we have a release ID
	if releaseID != "" {
		resp.DiscoveredIds["musicbrainz_album"] = releaseID

		// Try to get additional info from releases in recording
		for _, rel := range recording.Releases {
			if rel.ID == releaseID {
				metadata.Album = strPtr(rel.Title)
				if rel.Date != "" {
					if year := extractYear(rel.Date); year > 0 {
						y := int32(year)
						metadata.Year = &y
					}
					metadata.ReleaseDate = strPtr(rel.Date)
				}
				break
			}
		}

		// Fetch cover art
		coverArt, err := client.GetCoverArt(ctx, releaseID)
		if err != nil {
			p.logger.Warn("failed to fetch cover art", "release_id", releaseID, "error", err)
		} else if coverArt != nil {
			resp.Images = p.extractCoverArtImages(client, coverArt)
		}
	}

	resp.Metadata = metadata
	return resp, nil
}

// buildAlbumResponse builds an EnrichResponse for a release.
func (p *MusicBrainzPlugin) buildAlbumResponse(ctx context.Context, client *Client, release *Release) (*pluginv1.EnrichResponse, error) {
	resp := &pluginv1.EnrichResponse{
		Matched:       true,
		DiscoveredIds: make(map[string]string),
		Confidence:    0.85,
	}

	// External IDs
	resp.DiscoveredIds["musicbrainz_album"] = release.ID
	if release.Barcode != "" {
		resp.DiscoveredIds["barcode"] = release.Barcode
	}
	if release.ReleaseGroup != nil {
		resp.DiscoveredIds["musicbrainz_release_group"] = release.ReleaseGroup.ID
	}

	// Build album metadata
	albumMeta := &pluginv1.AlbumMetadata{
		Title:              strPtr(release.Title),
		MusicbrainzAlbumId: strPtr(release.ID),
	}

	// Artist
	if len(release.ArtistCredit) > 0 {
		artistName := formatArtistCredit(release.ArtistCredit)
		albumMeta.AlbumArtist = strPtr(artistName)
		albumMeta.Artist = strPtr(artistName)

		if release.ArtistCredit[0].Artist != nil {
			resp.DiscoveredIds["musicbrainz_artist"] = release.ArtistCredit[0].Artist.ID
		}
	}

	// Date and year
	if release.Date != "" {
		albumMeta.ReleaseDate = strPtr(release.Date)
		if year := extractYear(release.Date); year > 0 {
			y := int32(year)
			albumMeta.Year = &y
		}
	}

	// Track/disc counts
	var totalTracks, totalDiscs int32
	for _, medium := range release.Media {
		totalTracks += int32(medium.TrackCount)
		totalDiscs++
	}
	if totalTracks > 0 {
		albumMeta.TotalTracks = &totalTracks
	}
	if totalDiscs > 0 {
		albumMeta.TotalDiscs = &totalDiscs
	}

	// Label
	if len(release.LabelInfo) > 0 && release.LabelInfo[0].Label != nil {
		albumMeta.RecordLabel = strPtr(release.LabelInfo[0].Label.Name)
	}

	// Release type from release group
	if release.ReleaseGroup != nil {
		albumMeta.ReleaseType = strPtr(strings.ToLower(release.ReleaseGroup.PrimaryType))
		if len(release.ReleaseGroup.SecondaryTypes) > 0 {
			for _, st := range release.ReleaseGroup.SecondaryTypes {
				if strings.EqualFold(st, "Compilation") {
					t := true
					albumMeta.Compilation = &t
					break
				}
			}
		}
	}

	// Genre from tags
	if len(release.Tags) > 0 {
		genre := getPrimaryGenre(release.Tags)
		if genre != "" {
			albumMeta.Genre = strPtr(genre)
		}
	}

	resp.Metadata = &pluginv1.EnrichedMetadata{
		AlbumMetadata: albumMeta,
	}

	// Fetch cover art
	coverArt, err := client.GetCoverArt(ctx, release.ID)
	if err != nil {
		p.logger.Warn("failed to fetch cover art", "release_id", release.ID, "error", err)
	} else if coverArt != nil {
		resp.Images = p.extractCoverArtImages(client, coverArt)
	}

	return resp, nil
}

// buildArtistResponse builds an EnrichResponse for an artist.
func (p *MusicBrainzPlugin) buildArtistResponse(artist *Artist) *pluginv1.EnrichResponse {
	resp := &pluginv1.EnrichResponse{
		Matched:       true,
		DiscoveredIds: make(map[string]string),
		Confidence:    0.85,
	}

	// External ID
	resp.DiscoveredIds["musicbrainz_artist"] = artist.ID

	// Build artist metadata
	artistMeta := &pluginv1.ArtistMetadata{
		Name:               strPtr(artist.Name),
		SortName:           strPtr(artist.SortName),
		MusicbrainzArtistId: strPtr(artist.ID),
		Country:            strPtr(artist.Country),
	}

	// Formed year from life span
	if artist.LifeSpan != nil && artist.LifeSpan.Begin != "" {
		if year := extractYear(artist.LifeSpan.Begin); year > 0 {
			y := int32(year)
			artistMeta.FormedYear = &y
		}
	}

	// Genre from tags
	if len(artist.Tags) > 0 {
		genre := getPrimaryGenre(artist.Tags)
		if genre != "" {
			artistMeta.Genre = strPtr(genre)
		}
	}

	resp.Metadata = &pluginv1.EnrichedMetadata{
		ArtistMetadata: artistMeta,
	}

	return resp
}

// extractCoverArtImages extracts images from Cover Art Archive response.
func (p *MusicBrainzPlugin) extractCoverArtImages(client *Client, coverArt *CoverArtResponse) []*pluginv1.EnrichedImage {
	var images []*pluginv1.EnrichedImage

	for _, img := range coverArt.Images {
		if !img.Approved {
			continue
		}

		imageURL := client.GetCoverArtURL(&img)
		if imageURL == "" {
			continue
		}

		// Determine image type
		imageType := "poster" // Default for front cover
		if img.Back {
			imageType = "fanart"
		}
		for _, t := range img.Types {
			switch strings.ToLower(t) {
			case "front":
				imageType = "poster"
			case "back":
				imageType = "fanart"
			case "medium":
				imageType = "discart"
			case "booklet":
				imageType = "thumb"
			}
		}

		images = append(images, &pluginv1.EnrichedImage{
			Type:     imageType,
			Path:     imageURL,
			IsRemote: true,
		})

		// Limit to a reasonable number of images
		if len(images) >= 5 {
			break
		}
	}

	return images
}

// formatArtistCredit formats artist credits into a single string.
func formatArtistCredit(credits []ArtistCredit) string {
	var parts []string
	for _, credit := range credits {
		name := credit.Name
		if name == "" && credit.Artist != nil {
			name = credit.Artist.Name
		}
		if name != "" {
			parts = append(parts, name+credit.JoinPhrase)
		}
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

// getPrimaryGenre returns the most popular tag as a genre.
func getPrimaryGenre(tags []Tag) string {
	if len(tags) == 0 {
		return ""
	}

	// Tags are typically sorted by vote count, but let's be safe
	best := tags[0]
	for _, t := range tags[1:] {
		if t.Count > best.Count {
			best = t
		}
	}

	// Capitalize first letter
	if best.Name != "" {
		return strings.ToUpper(best.Name[:1]) + best.Name[1:]
	}
	return ""
}

// extractYear extracts a 4-digit year from a date string.
func extractYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	year, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return year
}

// Helper functions for optional proto fields

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	s = strings.TrimSpace(s)
	return &s
}
