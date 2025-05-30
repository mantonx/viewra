package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/mediaassetmodule"
)

// MusicFileExtensions defines supported music file formats (audio only)
var MusicFileExtensions = map[string]bool{
	".mp3":  true,
	".flac": true,
	".m4a":  true,
	".mp4":  true,
	".aac":  true,
	".ogg":  true,
	".wav":  true,
	".wma":  true,
	".ape":  true,
	".mpc":  true,
	".wv":   true,
	".opus": true,
	".aiff": true,
}

// MediaLibraryExtensions defines supported media file formats for music libraries (audio + video)
var MediaLibraryExtensions = map[string]bool{
	// Audio formats
	".mp3":  true,
	".flac": true,
	".m4a":  true,
	".aac":  true,
	".ogg":  true,
	".wav":  true,
	".wma":  true,
	".ape":  true,
	".mpc":  true,
	".wv":   true,
	".opus": true,
	".aiff": true,
	
	// Video formats (for music videos, concerts, etc.)
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".m4v":  true,
	".3gp":  true,
	".ogv":  true,
}

// IsMusicFile checks if a file is a supported music format (audio only)
func IsMusicFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return MusicFileExtensions[ext]
}

// IsMediaLibraryFile checks if a file is supported in a media library (audio + video)
func IsMediaLibraryFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return MediaLibraryExtensions[ext]
}

// ExtractMusicMetadata extracts metadata from a music file and saves artwork
func ExtractMusicMetadata(filePath string, mediaFile *database.MediaFile) (*database.MusicMetadata, error) {
	fmt.Printf("Extracting metadata from: %s\n", filePath)
	
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file %s: %v\n", filePath, err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Extract metadata using the tag library
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Extract technical information using FFprobe (if available)
	var technicalInfo *AudioTechnicalInfo
	if IsFFProbeAvailable() {
		technicalInfo, err = ExtractAudioTechnicalInfo(filePath)
		if err != nil {
			fmt.Printf("WARNING: FFprobe extraction failed for %s: %v\n", filePath, err)
			// Continue with fallback approach
		}
	}

	// Create MusicMetadata instance
	musicMeta := &database.MusicMetadata{
		MediaFileID: mediaFile.ID,
		Title:       metadata.Title(),
		Album:       metadata.Album(),
		Artist:      metadata.Artist(),
		AlbumArtist: metadata.AlbumArtist(),
		Genre:       metadata.Genre(),
	}

	// Use FFprobe data if available, otherwise fall back to file extension
	if technicalInfo != nil {
		musicMeta.Format = technicalInfo.Format
		musicMeta.Bitrate = technicalInfo.Bitrate
		musicMeta.SampleRate = technicalInfo.SampleRate
		musicMeta.Channels = technicalInfo.Channels
		if technicalInfo.Duration > 0 {
			musicMeta.Duration = time.Duration(technicalInfo.Duration * float64(time.Second))
		}
	} else {
		// Fallback to file extension
		musicMeta.Format = strings.ToLower(filepath.Ext(filePath)[1:])
		musicMeta.Bitrate = 0 // No bitrate available without FFprobe
		musicMeta.SampleRate = 0 // No sample rate available without FFprobe
		musicMeta.Channels = 0 // No channel info available without FFprobe
	}

	// Handle track and disc numbers which return multiple values
	trackNum, trackTotal := metadata.Track()
	musicMeta.Track = trackNum
	musicMeta.TrackTotal = trackTotal

	discNum, discTotal := metadata.Disc()
	musicMeta.Disc = discNum
	musicMeta.DiscTotal = discTotal

	// Handle year
	if metadata.Year() != 0 {
		musicMeta.Year = metadata.Year()
	}

	// Handle artwork
	picture := metadata.Picture()
	if picture != nil && len(picture.Data) > 0 {
		musicMeta.HasArtwork = true
		fmt.Printf("Found artwork in file %s with MIME type: %s\n", filePath, picture.MIMEType)
		
		// Store artwork data temporarily - will be saved after MediaFile gets its ID
		musicMeta.ArtworkData = picture.Data
		musicMeta.ArtworkExt = picture.Ext
		
		// If we have the MediaFile ID, save the artwork immediately
		if mediaFile.ID != 0 {
			if err := SaveArtworkForMediaFile(mediaFile.ID, picture.Data, picture.MIMEType); err != nil {
				fmt.Printf("WARNING: Failed to save artwork for file %s: %v\n", filePath, err)
			} else {
				fmt.Printf("Successfully saved artwork for file %s\n", filePath)
			}
		} else {
			fmt.Printf("Stored artwork data for file %s (will save after MediaFile gets ID)\n", filePath)
		}
	} else {
		fmt.Printf("No artwork found in file %s\n", filePath)
	}

	return musicMeta, nil
}

// SaveArtworkForMediaFile saves artwork for a media file using the new asset system with enhanced subtype detection
func SaveArtworkForMediaFile(mediaFileID uint, data []byte, mimeType string) error {
	if len(data) == 0 {
		return fmt.Errorf("artwork data cannot be empty")
	}
	
	if mimeType == "" {
		mimeType = "image/jpeg" // Default MIME type
	}
	
	// Determine more specific subtype based on artwork characteristics
	subtype := determineArtworkSubtype(data, mimeType)
	
	// Create asset request with enhanced subtype
	request := &mediaassetmodule.AssetRequest{
		MediaFileID: mediaFileID,
		Type:        mediaassetmodule.AssetTypeMusic,
		Category:    mediaassetmodule.CategoryAlbum,
		Subtype:     subtype,
		Data:        data,
		MimeType:    mimeType,
	}

	// Save using the new asset manager
	_, err := mediaassetmodule.SaveMediaAsset(request)
	if err != nil {
		return fmt.Errorf("failed to save artwork with new asset system: %w", err)
	}

	fmt.Printf("INFO: Successfully saved artwork for media file ID %d with subtype %s\n", mediaFileID, subtype)
	return nil
}

// determineArtworkSubtype analyzes artwork to determine the most appropriate subtype
func determineArtworkSubtype(data []byte, mimeType string) mediaassetmodule.AssetSubtype {
	// For now, use album_front as the primary subtype for embedded artwork
	// This is more specific than the generic "artwork" and represents the most common case
	// In the future, this could be enhanced with image analysis to detect:
	// - Image dimensions (square = likely front cover, rectangular = might be booklet)
	// - Image content analysis (text detection, layout analysis)
	// - File size (larger images might be higher quality front covers)
	
	// Basic size-based heuristics
	dataSize := len(data)
	
	if dataSize > 500000 { // > 500KB - likely high quality front cover
		return mediaassetmodule.SubtypeAlbumFront
	} else if dataSize > 100000 { // > 100KB - likely standard front cover
		return mediaassetmodule.SubtypeAlbumFront
	} else if dataSize > 50000 { // > 50KB - could be thumbnail or medium quality
		return mediaassetmodule.SubtypeAlbumThumb
	} else {
		// Small images - likely thumbnails
		return mediaassetmodule.SubtypeAlbumThumb
	}
}
