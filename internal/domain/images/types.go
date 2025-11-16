package images

import "fmt"

// ImageType represents the type/purpose of an image
type ImageType string

const (
	// Movie and TV image types
	ImageTypePoster      ImageType = "poster"      // Main poster (2:3 aspect ratio)
	ImageTypeFanart      ImageType = "fanart"      // Background/backdrop (16:9)
	ImageTypeBackdrop    ImageType = "backdrop"    // Alternative name for fanart
	ImageTypeBanner      ImageType = "banner"      // Wide banner
	ImageTypeClearLogo   ImageType = "clearlogo"   // Transparent logo PNG
	ImageTypeLandscape   ImageType = "landscape"   // Landscape poster (16:9)
	ImageTypeThumb       ImageType = "thumb"       // Episode thumbnail
	ImageTypeActor       ImageType = "actor"       // Actor headshot
	ImageTypeExtraFanart ImageType = "extrafanart" // Additional backdrops
	ImageTypeCharacterArt ImageType = "characterart" // Character artwork
	ImageTypeClearArt    ImageType = "clearart"    // Clear character art

	// Music-specific image types
	ImageTypeCover   ImageType = "cover"   // Album cover
	ImageTypeFolder  ImageType = "folder"  // Folder image (album or artist)
	ImageTypeDiscArt ImageType = "discart" // Disc artwork
	ImageTypeLogo    ImageType = "logo"    // Artist/band logo
)

// IsValid checks if the image type is valid
func (t ImageType) IsValid() bool {
	switch t {
	case ImageTypePoster, ImageTypeFanart, ImageTypeBackdrop, ImageTypeBanner,
		ImageTypeClearLogo, ImageTypeLandscape, ImageTypeThumb, ImageTypeActor,
		ImageTypeExtraFanart, ImageTypeCharacterArt, ImageTypeClearArt,
		ImageTypeCover, ImageTypeFolder, ImageTypeDiscArt, ImageTypeLogo:
		return true
	}
	return false
}

// String returns the string representation
func (t ImageType) String() string {
	return string(t)
}

// SourceType represents where the image came from
type SourceType string

const (
	SourceTypeLocal       SourceType = "local"       // Local file in media library
	SourceTypeTMDb        SourceType = "tmdb"        // Downloaded from TMDb
	SourceTypeMusicBrainz SourceType = "musicbrainz" // Downloaded from MusicBrainz
	SourceTypeTVDb        SourceType = "tvdb"        // Downloaded from TheTVDB
	SourceTypeFanartTV    SourceType = "fanart.tv"   // Downloaded from fanart.tv
	SourceTypeManual      SourceType = "manual"      // Manually uploaded by user
)

// IsValid checks if the source type is valid
func (s SourceType) IsValid() bool {
	switch s {
	case SourceTypeLocal, SourceTypeTMDb, SourceTypeMusicBrainz,
		SourceTypeTVDb, SourceTypeFanartTV, SourceTypeManual:
		return true
	}
	return false
}

// String returns the string representation
func (s SourceType) String() string {
	return string(s)
}

// MediaType represents the type of media entity
type MediaType string

const (
	MediaTypeMovie       MediaType = "movie"
	MediaTypeTVShow      MediaType = "tv_show"
	MediaTypeTVSeason    MediaType = "tv_season"
	MediaTypeTVEpisode   MediaType = "tv_episode"
	MediaTypeMusicArtist MediaType = "music_artist"
	MediaTypeMusicAlbum  MediaType = "music_album"
	MediaTypeMusicTrack  MediaType = "music_track"
)

// IsValid checks if the media type is valid
func (m MediaType) IsValid() bool {
	switch m {
	case MediaTypeMovie, MediaTypeTVShow, MediaTypeTVSeason, MediaTypeTVEpisode,
		MediaTypeMusicArtist, MediaTypeMusicAlbum, MediaTypeMusicTrack:
		return true
	}
	return false
}

// String returns the string representation
func (m MediaType) String() string {
	return string(m)
}

// Validate returns an error if the type is invalid
func (m MediaType) Validate() error {
	if !m.IsValid() {
		return fmt.Errorf("invalid media type: %s", m)
	}
	return nil
}
