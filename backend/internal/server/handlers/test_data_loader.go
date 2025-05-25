// Package handlers provides HTTP request handlers for the Viewra application.
// This file contains handlers for loading test data into the database.
package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/gin-gonic/gin"
	"github.com/yourusername/viewra/internal/database"
	"github.com/yourusername/viewra/internal/metadata"
)

// LoadTestMusicData loads test music data into the database
// This is a development-only endpoint to bypass scanner issues
func LoadTestMusicData(c *gin.Context) {
	// Check several possible locations for the test music directory
	possiblePaths := []string{
		"/home/fictional/Projects/viewra/backend/data/test-music",
		"./data/test-music",
		"../data/test-music",
		"/app/data/test-music",
	}
	
	var testMusicPath string
	var dirExists bool
	
	// Try each path
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("Found test music directory at: %s\n", path)
			testMusicPath = path
			dirExists = true
			break
		}
	}
	
	// If directory doesn't exist anywhere we checked, try creating it
	if !dirExists {
		// Try to create the test directory in a location we can write to
		testMusicPath = "./data/test-music"
		if err := os.MkdirAll(testMusicPath, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Could not find or create test music directory",
			})
			return
		}
		dirExists = true
	}
	
	// If we still don't have a valid directory, return an error
	if !dirExists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Could not find or create test music directory",
		})
		return
	}

	// Create a test library for music if it doesn't exist
	db := database.GetDB()
	var testLibrary database.MediaLibrary
	if err := db.Where("path = ? AND type = ?", testMusicPath, "music").First(&testLibrary).Error; err != nil {
		testLibrary = database.MediaLibrary{
			Path: testMusicPath,
			Type: "music",
		}
		if err := db.Create(&testLibrary).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to create test library: %v", err),
			})
			return
		}
	}

	// Process all MP3 files in the directory
	fmt.Printf("Reading files from directory: %s\n", testMusicPath)
	files, err := os.ReadDir(testMusicPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to read directory: %v", err),
		})
		return
	}

	fmt.Printf("Found %d files in directory\n", len(files))
	
	// List all files in the directory for debugging
	for _, file := range files {
		fmt.Printf("  - %s (isDir=%v)\n", file.Name(), file.IsDir())
	}

	// Track results
	processed := 0
	skipped := 0
	failed := 0

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := filepath.Ext(file.Name())
		if !metadata.IsMusicFile(file.Name()) {
			skipped++
			continue
		}

		fullPath := filepath.Join(testMusicPath, file.Name())
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			failed++
			continue
		}

		// Create or update the media file record
		var mediaFile database.MediaFile
		result := db.Where("path = ?", fullPath).First(&mediaFile)
		if result.Error != nil {
			// Create new record
			mediaFile = database.MediaFile{
				Path:      fullPath,
				Size:      fileInfo.Size(),
				LibraryID: testLibrary.ID,
				LastSeen:  time.Now(),
				Hash:      fmt.Sprintf("test_%d", time.Now().UnixNano()), // Dummy hash
			}
			if err := db.Create(&mediaFile).Error; err != nil {
				failed++
				continue
			}
		} else {
			// Update existing record
			mediaFile.LastSeen = time.Now()
			db.Save(&mediaFile)
		}

		// Extract metadata from the file
		f, err := os.Open(fullPath)
		if err != nil {
			failed++
			continue
		}

		// Extract metadata using the tag library
		metaData, err := tag.ReadFrom(f)
		f.Close()
		if err != nil {
			failed++
			continue
		}

		// Delete existing metadata if it exists
		db.Where("media_file_id = ?", mediaFile.ID).Delete(&database.MusicMetadata{})

		// Create MusicMetadata instance
		musicMeta := &database.MusicMetadata{
			MediaFileID: mediaFile.ID,
			Title:       metaData.Title(),
			Album:       metaData.Album(),
			Artist:      metaData.Artist(),
			AlbumArtist: metaData.AlbumArtist(),
			Genre:       metaData.Genre(),
			Format:      strings.ToLower(ext[1:]), // Remove the dot
		}

		// Handle track and disc numbers which return multiple values
		trackNum, trackTotal := metaData.Track()
		musicMeta.Track = trackNum
		musicMeta.TrackTotal = trackTotal

		discNum, discTotal := metaData.Disc()
		musicMeta.Disc = discNum
		musicMeta.DiscTotal = discTotal

		// Handle year
		if metaData.Year() != 0 {
			musicMeta.Year = metaData.Year()
		}

		// Handle artwork
		picture := metaData.Picture()
		if picture != nil && len(picture.Data) > 0 {
			musicMeta.HasArtwork = true

			// Save artwork to cache
			err := metadata.SaveArtwork(mediaFile.ID, picture.Data, picture.Ext)
			if err != nil {
				fmt.Printf("Warning: failed to save artwork for file %s: %v\n", fullPath, err)
			} else {
				fmt.Printf("Successfully saved artwork for file %s\n", fullPath)
			}
		}

		// Save the metadata
		if err := db.Create(musicMeta).Error; err != nil {
			failed++
			continue
		}

		processed++
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Test music data loaded successfully",
		"processed": processed,
		"skipped":   skipped,
		"failed":    failed,
		"library_id": testLibrary.ID,
	})
}
