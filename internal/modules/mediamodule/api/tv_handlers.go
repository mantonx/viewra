// Package api - TV show handlers
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// GetTVShows handles GET /api/tv/shows
// It returns all TV shows in the library with their seasons and episodes preloaded.
//
// Query parameters:
//   - TODO: Add pagination support
//   - TODO: Add filtering by genre, year, etc.
//
// Response: Array of TVShow objects with nested seasons and episodes
func (h *Handler) GetTVShows(c *gin.Context) {
	var shows []database.TVShow
	db := c.MustGet("db").(*gorm.DB)
	
	// Preload seasons and episodes for complete show information
	query := db.Preload("Seasons.Episodes")
	if err := query.Find(&shows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, shows)
}

// GetTVShow handles GET /api/tv/shows/:id
// It returns detailed information about a specific TV show.
//
// Path parameters:
//   - id: The TV show ID
//
// Response: TVShow object with nested seasons and episodes or 404 if not found
func (h *Handler) GetTVShow(c *gin.Context) {
	id := c.Param("id")
	
	var show database.TVShow
	db := c.MustGet("db").(*gorm.DB)
	
	// Load show with all related data
	if err := db.Preload("Seasons.Episodes").Where("id = ?", id).First(&show).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "TV show not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, show)
}

// GetSeasons handles GET /api/tv/shows/:id/seasons
// It returns all seasons for a specific TV show.
//
// Path parameters:
//   - id: The TV show ID
//
// Response: Array of Season objects with episodes
func (h *Handler) GetSeasons(c *gin.Context) {
	showID := c.Param("id")
	
	var seasons []database.Season
	db := c.MustGet("db").(*gorm.DB)
	
	// Get all seasons for the show with episodes preloaded
	if err := db.Preload("Episodes").Where("tv_show_id = ?", showID).Find(&seasons).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, seasons)
}

// GetEpisodes handles GET /api/tv/shows/:id/seasons/:seasonId/episodes
// It returns all episodes for a specific season.
//
// Path parameters:
//   - id: The TV show ID
//   - seasonId: The season ID
//
// Query parameters:
//   - episode_number: Filter by specific episode number
//
// Response: Array of Episode objects with associated media files
func (h *Handler) GetEpisodes(c *gin.Context) {
	seasonID := c.Param("seasonId")
	
	var episodes []database.Episode
	db := c.MustGet("db").(*gorm.DB)
	
	// Build query for episodes
	query := db.Where("season_id = ?", seasonID)
	
	// Add optional episode number filter
	if episodeNumber := c.Query("episode_number"); episodeNumber != "" {
		query = query.Where("episode_number = ?", episodeNumber)
	}
	
	if err := query.Find(&episodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Attach media files to episodes
	// This provides the file information needed for playback
	episodesWithMedia := h.attachMediaFilesToEpisodes(db, episodes)
	
	c.JSON(http.StatusOK, episodesWithMedia)
}

// GetEpisode handles GET /api/tv/episodes/:episodeId
// It returns detailed information about a specific episode.
//
// Path parameters:
//   - episodeId: The episode ID
//
// Response: Episode object with season, show, and media file information
func (h *Handler) GetEpisode(c *gin.Context) {
	episodeID := c.Param("episodeId")
	
	var episode database.Episode
	db := c.MustGet("db").(*gorm.DB)
	
	// Load episode with its season and show information
	if err := db.Preload("Season.TVShow").Where("id = ?", episodeID).First(&episode).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Episode not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Attach media file information
	episodeWithMedia := h.attachMediaFileToEpisode(db, episode)
	
	c.JSON(http.StatusOK, episodeWithMedia)
}

// attachMediaFilesToEpisodes attaches media file information to episodes.
// This creates a response structure that includes playback information.
func (h *Handler) attachMediaFilesToEpisodes(db *gorm.DB, episodes []database.Episode) []gin.H {
	result := make([]gin.H, len(episodes))
	
	for i, episode := range episodes {
		// Convert episode to response format
		episodeData := gin.H{
			"id":             episode.ID,
			"title":          episode.Title,
			"episode_number": episode.EpisodeNumber,
			"air_date":       episode.AirDate,
			"description":    episode.Description,
			"duration":       episode.Duration,
			"still_image":    episode.StillImage,
			"season_id":      episode.SeasonID,
			"created_at":     episode.CreatedAt,
			"updated_at":     episode.UpdatedAt,
		}
		
		// Find associated media file
		var mediaFile database.MediaFile
		if err := db.Where("media_id = ? AND media_type = ?", episode.ID, "episode").First(&mediaFile).Error; err == nil {
			episodeData["media_file"] = mediaFile
		} else {
			episodeData["media_file"] = nil
		}
		
		result[i] = episodeData
	}
	
	return result
}

// attachMediaFileToEpisode attaches media file information to a single episode.
func (h *Handler) attachMediaFileToEpisode(db *gorm.DB, episode database.Episode) gin.H {
	// Convert episode to response format
	episodeData := gin.H{
		"id":             episode.ID,
		"title":          episode.Title,
		"episode_number": episode.EpisodeNumber,
		"air_date":       episode.AirDate,
		"description":    episode.Description,
		"duration":       episode.Duration,
		"still_image":    episode.StillImage,
		"season_id":      episode.SeasonID,
		"season":         episode.Season,
		"created_at":     episode.CreatedAt,
		"updated_at":     episode.UpdatedAt,
	}
	
	// Find associated media file
	var mediaFile database.MediaFile
	if err := db.Where("media_id = ? AND media_type = ?", episode.ID, "episode").First(&mediaFile).Error; err == nil {
		episodeData["media_file"] = mediaFile
	} else {
		episodeData["media_file"] = nil
	}
	
	return episodeData
}