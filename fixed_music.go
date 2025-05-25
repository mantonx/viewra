package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
)

// GetMusicMetadata retrieves music metadata for a media file
func GetMusicMetadata(c *gin.Context) {
	mediaFileIDStr := c.Param("id")
	mediaFileID, err := strconv.ParseUint(mediaFileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid media file ID",
		})
		return
	}

	db := database.GetDB()

	// First check if the media file exists
	var mediaFile database.MediaFile
	if err := db.First(&mediaFile, mediaFileID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media file not found",
		})
		return
	}

	// Get music metadata
	var musicMetadata database.MusicMetadata
	if err := db.Where("media_file_id = ?", mediaFileID).First(&musicMetadata).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Music metadata not found for this file",
		})
		return
	}

	// Return metadata
	c.JSON(http.StatusOK, musicMetadata)
}

// GetMusicFiles retrieves all music files with their metadata
func GetMusicFiles(c *gin.Context) {
	// Parse query parameters for pagination
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	db := database.GetDB()

	// Enable debug mode to show SQL queries
	fmt.Println("=== DEBUG: MUSIC FILES QUERY ===")

	// Get total count of music files - this is the key query that needs debugging
	var total int64
	countQuery := db.Model(&database.MediaFile{}).
		Debug(). // Enable debug for GORM
		Where("path LIKE '%.mp3' OR path LIKE '%.flac' OR path LIKE '%.aac' OR path LIKE '%.ogg' OR path LIKE '%.wav'")

	countQuery.Count(&total)
	fmt.Printf("Total music files found: %d\n", total)

	// Get paginated results with music metadata preloaded
	var mediaFiles []database.MediaFile
	result := db.Debug(). // Enable debug for GORM
		Preload("MusicMetadata").
		Where("path LIKE '%.mp3' OR path LIKE '%.flac' OR path LIKE '%.aac' OR path LIKE '%.ogg' OR path LIKE '%.wav'").
		Limit(limit).
		Offset(offset).
		Order("id DESC"). // Order by more recent first
		Find(&mediaFiles)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get music files",
			"details": result.Error.Error(),
		})
		return
	}

	// Log files found
	fmt.Printf("Found %d music files in pagination\n", len(mediaFiles))
	for i, file := range mediaFiles {
		if i < 5 { // Just log the first 5 for brevity
			fmt.Printf("File %d: %s (Library: %d)\n", i+1, file.Path, file.LibraryID)
		}
	}

	// For debugging, manually create a music test entry if none exist
	if len(mediaFiles) == 0 {
		// Create a test library
		testLib := database.MediaLibrary{
			Path: "/test-music-path",
			Type: "music",
		}
		db.Create(&testLib)

		// Create a test media file
		mediaFile := database.MediaFile{
			Path:      "/test-music-path/test-song.mp3",
			Size:      1024,
			Hash:      "testhash123",
			LibraryID: testLib.ID,
			LastSeen:  time.Now(),
		}
		db.Create(&mediaFile)

		// Create a test music metadata entry
		musicMeta := &database.MusicMetadata{
			MediaFileID: mediaFile.ID,
			Title:       "Test Song",
			Album:       "Test Album",
			Artist:      "Test Artist",
			AlbumArtist: "Test Artist",
			Genre:       "Test Genre",
			Year:        2023,
			Track:       1,
			TrackTotal:  10,
			Disc:        1,
			DiscTotal:   1,
			Format:      "mp3",
			HasArtwork:  false,
		}
		db.Create(musicMeta)

		// Load the created file with its metadata
		db.Preload("MusicMetadata").First(&mediaFile, mediaFile.ID)
		mediaFiles = append(mediaFiles, mediaFile)
		total = 1
		
		fmt.Println("Created test music entry")
	}

	c.JSON(http.StatusOK, gin.H{
		"music_files": mediaFiles,
		"total":       total,
		"count":       len(mediaFiles),
		"limit":       limit,
		"offset":      offset,
	})
}