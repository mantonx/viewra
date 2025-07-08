// Package api - Movie handlers
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// GetMovies handles GET /api/movies
// It returns all movies in the library with their associated media files.
//
// Query parameters:
//   - genre: Filter by genre
//   - year: Filter by release year
//   - min_rating: Filter by minimum rating
//   - limit: Number of results per page
//   - offset: Number of results to skip
//   - sort_by: Field to sort by (title, year, rating, date_added)
//   - sort_order: Sort direction (asc, desc)
//
// Response: Array of Movie objects with media file information
func (h *Handler) GetMovies(c *gin.Context) {
	var movies []database.Movie
	db := c.MustGet("db").(*gorm.DB)
	
	// Build query with filters
	query := db.Model(&database.Movie{})
	
	// Genre filter - search in JSON genres field
	if genre := c.Query("genre"); genre != "" {
		query = query.Where("genres LIKE ?", "%"+genre+"%")
	}
	
	// Year filter - extract from release_date
	if year := c.Query("year"); year != "" {
		query = query.Where("YEAR(release_date) = ?", year)
	}
	
	// Rating filter
	if minRating := c.Query("min_rating"); minRating != "" {
		query = query.Where("rating >= ?", minRating)
	}
	
	// Sorting
	sortBy := c.DefaultQuery("sort_by", "date_added")
	sortOrder := c.DefaultQuery("sort_order", "desc")
	query = query.Order(sortBy + " " + sortOrder)
	
	// Pagination
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			query = query.Limit(limit)
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			query = query.Offset(offset)
		}
	}
	
	if err := query.Find(&movies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Attach media files to movies
	moviesWithMedia := h.attachMediaFilesToMovies(db, movies)
	
	c.JSON(http.StatusOK, moviesWithMedia)
}

// GetMovie handles GET /api/movies/:id
// It returns detailed information about a specific movie.
//
// Path parameters:
//   - id: The movie ID
//
// Response: Movie object with media file information or 404 if not found
func (h *Handler) GetMovie(c *gin.Context) {
	id := c.Param("id")
	
	var movie database.Movie
	db := c.MustGet("db").(*gorm.DB)
	
	// Load movie
	if err := db.Where("id = ?", id).First(&movie).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Movie not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Attach media file information
	movieWithMedia := h.attachMediaFileToMovie(db, movie)
	
	c.JSON(http.StatusOK, movieWithMedia)
}

// SearchMovies handles GET /api/movies/search
// It provides search functionality for movies.
//
// Query parameters:
//   - q: Search query (searches in title, director, cast)
//   - limit: Maximum results to return (default: 20)
//
// Response:
//   {
//     "results": [...],
//     "count": 10
//   }
func (h *Handler) SearchMovies(c *gin.Context) {
	search := c.Query("q")
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	
	if search == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query required"})
		return
	}
	
	var movies []database.Movie
	db := c.MustGet("db").(*gorm.DB)
	
	// Search in title and overview fields
	searchPattern := "%" + search + "%"
	query := db.Where("title LIKE ? OR overview LIKE ?", 
		searchPattern, searchPattern).
		Limit(limit)
	
	if err := query.Find(&movies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Attach media files
	moviesWithMedia := h.attachMediaFilesToMovies(db, movies)
	
	c.JSON(http.StatusOK, gin.H{
		"results": moviesWithMedia,
		"count":   len(moviesWithMedia),
	})
}

// GetSimilarMovies handles GET /api/movies/:id/similar
// It returns movies similar to the specified movie based on genre and other metadata.
//
// Path parameters:
//   - id: The movie ID
//
// Query parameters:
//   - limit: Maximum number of similar movies to return (default: 10)
//
// Response: Array of similar Movie objects
func (h *Handler) GetSimilarMovies(c *gin.Context) {
	id := c.Param("id")
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	
	var movie database.Movie
	db := c.MustGet("db").(*gorm.DB)
	
	// Get the movie to find similar ones
	if err := db.Where("id = ?", id).First(&movie).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Movie not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Find similar movies based on genre
	// TODO: Enhance this with more sophisticated similarity matching
	var similarMovies []database.Movie
	query := db.Where("id != ? AND genres LIKE ?", id, "%"+movie.Genres+"%").
		Limit(limit)
	
	if err := query.Find(&similarMovies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Attach media files
	moviesWithMedia := h.attachMediaFilesToMovies(db, similarMovies)
	
	c.JSON(http.StatusOK, moviesWithMedia)
}

// attachMediaFilesToMovies attaches media file information to movies.
// This creates a response structure that includes playback information.
func (h *Handler) attachMediaFilesToMovies(db *gorm.DB, movies []database.Movie) []gin.H {
	result := make([]gin.H, len(movies))
	
	for i, movie := range movies {
		// Convert movie to response format
		movieData := gin.H{
			"id":             movie.ID,
			"title":          movie.Title,
			"original_title": movie.OriginalTitle,
			"overview":       movie.Overview,
			"tagline":        movie.Tagline,
			"release_date":   movie.ReleaseDate,
			"runtime":        movie.Runtime,
			"rating":         movie.Rating,
			"tmdb_rating":    movie.TmdbRating,
			"vote_count":     movie.VoteCount,
			"popularity":     movie.Popularity,
			"status":         movie.Status,
			"adult":          movie.Adult,
			"video":          movie.Video,
			"budget":         movie.Budget,
			"revenue":        movie.Revenue,
			"poster":         movie.Poster,
			"backdrop":       movie.Backdrop,
			"genres":         movie.Genres,
			"imdb_id":        movie.ImdbID,
			"tmdb_id":        movie.TmdbID,
			"created_at":     movie.CreatedAt,
			"updated_at":     movie.UpdatedAt,
		}
		
		// Find associated media file
		var mediaFile database.MediaFile
		if err := db.Where("media_id = ? AND media_type = ?", movie.ID, "movie").First(&mediaFile).Error; err == nil {
			movieData["media_file"] = mediaFile
		} else {
			movieData["media_file"] = nil
		}
		
		result[i] = movieData
	}
	
	return result
}

// attachMediaFileToMovie attaches media file information to a single movie.
func (h *Handler) attachMediaFileToMovie(db *gorm.DB, movie database.Movie) gin.H {
	// Convert movie to response format
	movieData := gin.H{
		"id":             movie.ID,
		"title":          movie.Title,
		"original_title": movie.OriginalTitle,
		"overview":       movie.Overview,
		"tagline":        movie.Tagline,
		"release_date":   movie.ReleaseDate,
		"runtime":        movie.Runtime,
		"rating":         movie.Rating,
		"tmdb_rating":    movie.TmdbRating,
		"vote_count":     movie.VoteCount,
		"popularity":     movie.Popularity,
		"status":         movie.Status,
		"adult":          movie.Adult,
		"video":          movie.Video,
		"budget":         movie.Budget,
		"revenue":        movie.Revenue,
		"poster":         movie.Poster,
		"backdrop":       movie.Backdrop,
		"genres":         movie.Genres,
		"imdb_id":        movie.ImdbID,
		"tmdb_id":        movie.TmdbID,
		"created_at":     movie.CreatedAt,
		"updated_at":     movie.UpdatedAt,
	}
	
	// Find associated media file
	var mediaFile database.MediaFile
	if err := db.Where("media_id = ? AND media_type = ?", movie.ID, "movie").First(&mediaFile).Error; err == nil {
		movieData["media_file"] = mediaFile
	} else {
		movieData["media_file"] = nil
	}
	
	return movieData
}