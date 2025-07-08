// Package filters provides media filtering logic
package filters

import (
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	mediatypes "github.com/mantonx/viewra/internal/modules/mediamodule/types"
	"github.com/mantonx/viewra/internal/types"
	"gorm.io/gorm"
)

// MediaFilter handles the business logic for filtering media files
type MediaFilter struct{}

// NewMediaFilter creates a new media filter
func NewMediaFilter() *MediaFilter {
	return &MediaFilter{}
}

// ApplyFilter applies all filter criteria to a database query
func (f *MediaFilter) ApplyFilter(query *gorm.DB, filter types.MediaFilter) *gorm.DB {
	query = query.Model(&database.MediaFile{})

	// Apply basic filters
	query = f.applyBasicFilters(query, filter)
	
	// Apply format filters
	query = f.applyFormatFilters(query, filter)
	
	// Apply playback method filter
	query = f.applyPlaybackMethodFilter(query, filter)
	
	// Apply sorting
	query = f.applySorting(query, filter)
	
	// Apply pagination
	query = f.applyPagination(query, filter)
	
	return query
}

// applyBasicFilters applies library, type, and search filters
func (f *MediaFilter) applyBasicFilters(query *gorm.DB, filter types.MediaFilter) *gorm.DB {
	if filter.LibraryID != nil {
		query = query.Where("library_id = ?", *filter.LibraryID)
	}
	
	if filter.MediaType != "" {
		query = query.Where("media_type = ?", filter.MediaType)
	}
	
	if filter.Search != "" {
		query = query.Where("path LIKE ?", "%"+filter.Search+"%")
	}
	
	return query
}

// applyFormatFilters applies codec and container filters
func (f *MediaFilter) applyFormatFilters(query *gorm.DB, filter types.MediaFilter) *gorm.DB {
	if filter.VideoCodec != "" {
		query = query.Where("LOWER(video_codec) LIKE ?", "%"+filter.VideoCodec+"%")
	}
	
	if filter.AudioCodec != "" {
		query = query.Where("LOWER(audio_codec) LIKE ?", "%"+filter.AudioCodec+"%")
	}
	
	if filter.Container != "" {
		query = query.Where("LOWER(container) = ?", filter.Container)
	}
	
	if filter.Resolution != "" {
		query = f.applyResolutionFilter(query, filter.Resolution)
	}
	
	return query
}

// applyResolutionFilter applies resolution-based filtering
func (f *MediaFilter) applyResolutionFilter(query *gorm.DB, resolution string) *gorm.DB {
	switch resolution {
	case "4k":
		return query.Where("(resolution LIKE ? OR resolution LIKE ? OR video_height >= ?)", 
			"%2160%", "%4K%", 2160)
	case "1080p":
		return query.Where("(resolution LIKE ? OR (video_height >= ? AND video_height < ?))", 
			"%1080%", 1080, 1440)
	case "720p":
		return query.Where("(resolution LIKE ? OR (video_height >= ? AND video_height < ?))", 
			"%720%", 720, 1080)
	case "480p":
		return query.Where("(resolution LIKE ? OR resolution LIKE ? OR (video_height >= ? AND video_height < ?))", 
			"%480%", "%SDTV%", 480, 720)
	default:
		return query.Where("resolution LIKE ?", "%"+resolution+"%")
	}
}

// applyPlaybackMethodFilter filters based on browser compatibility
func (f *MediaFilter) applyPlaybackMethodFilter(query *gorm.DB, filter types.MediaFilter) *gorm.DB {
	if filter.PlaybackMethod == "" {
		return query
	}
	
	switch filter.PlaybackMethod {
	case "direct":
		// Direct play: H264 video, AAC/MP3 audio, MP4/WebM/MOV containers
		return query.Where(
			"(LOWER(container) IN (?, ?, ?) AND LOWER(video_codec) = ? AND LOWER(audio_codec) IN (?, ?))",
			"mp4", "webm", "mov", "h264", "aac", "mp3",
		)
		
	case "remux":
		// Remux: Good codecs but wrong container (e.g., H264 in MKV)
		return query.Where(
			"(LOWER(container) NOT IN (?, ?, ?) AND LOWER(video_codec) = ? AND LOWER(audio_codec) IN (?, ?, ?, ?))",
			"mp4", "webm", "mov", "h264", "aac", "mp3", "ac3", "eac3",
		)
		
	case "transcode":
		// Transcode: Incompatible codecs (HEVC, VP9, etc.)
		return query.Where(
			"(LOWER(video_codec) NOT IN (?) OR LOWER(audio_codec) NOT IN (?, ?, ?, ?))",
			"h264", "aac", "mp3", "ac3", "eac3",
		)
		
	default:
		return query
	}
}

// applySorting applies sorting to the query
func (f *MediaFilter) applySorting(query *gorm.DB, filter types.MediaFilter) *gorm.DB {
	if filter.SortBy != "" {
		order := "ASC"
		if filter.SortDesc {
			order = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", filter.SortBy, order))
	}
	return query
}

// applyPagination applies limit and offset
func (f *MediaFilter) applyPagination(query *gorm.DB, filter types.MediaFilter) *gorm.DB {
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	return query
}

// GetPlaybackMethod determines the playback method for a media file
func (f *MediaFilter) GetPlaybackMethod(file *database.MediaFile) mediatypes.PlaybackMethod {
	container := file.Container
	videoCodec := file.VideoCodec
	audioCodec := file.AudioCodec
	
	// Check for direct play compatibility
	if f.isDirectPlayCompatible(container, videoCodec, audioCodec) {
		return mediatypes.PlaybackMethodDirect
	}
	
	// Check if only remuxing is needed
	if f.isRemuxCompatible(container, videoCodec, audioCodec) {
		return mediatypes.PlaybackMethodRemux
	}
	
	// Otherwise transcode is required
	return mediatypes.PlaybackMethodTranscode
}

// isDirectPlayCompatible checks if a file can be played directly in browsers
func (f *MediaFilter) isDirectPlayCompatible(container, videoCodec, audioCodec string) bool {
	// Supported containers for direct play
	supportedContainers := map[string]bool{
		"mp4":  true,
		"webm": true,
		"mov":  true,
	}
	
	// Check container
	if !supportedContainers[container] {
		return false
	}
	
	// Check video codec
	if videoCodec != "h264" {
		return false
	}
	
	// Check audio codec
	supportedAudioCodecs := map[string]bool{
		"aac": true,
		"mp3": true,
	}
	
	return supportedAudioCodecs[audioCodec]
}

// isRemuxCompatible checks if a file only needs container change
func (f *MediaFilter) isRemuxCompatible(container, videoCodec, audioCodec string) bool {
	// If container is already supported, no remux needed
	supportedContainers := map[string]bool{
		"mp4":  true,
		"webm": true,
		"mov":  true,
	}
	
	if supportedContainers[container] {
		return false
	}
	
	// Check if codecs are compatible
	if videoCodec != "h264" {
		return false
	}
	
	// Extended audio codec support for remuxing
	supportedAudioCodecs := map[string]bool{
		"aac":  true,
		"mp3":  true,
		"ac3":  true,
		"eac3": true,
	}
	
	return supportedAudioCodecs[audioCodec]
}