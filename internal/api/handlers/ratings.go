package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/ratings"
	ratingsDomain "github.com/mantonx/viewra/internal/domain/ratings"
)

// RatingsHandler handles user ratings HTTP requests.
type RatingsHandler struct {
	service *ratings.Service
}

// NewRatingsHandler creates a new ratings handler.
func NewRatingsHandler(service *ratings.Service) *RatingsHandler {
	return &RatingsHandler{service: service}
}

// ListRatings lists all ratings for the current user.
//
// @Summary List user ratings
// @Description Gets all ratings for the current user with optional filters
// @Tags ratings
// @Produce json
// @Param entity_type query string false "Filter by entity type (movie, tv_show, tv_episode)"
// @Param rating query string false "Filter by rating type (up, down, favorite)"
// @Success 200 {object} ListRatingsResponse
// @Failure 500 {object} handlers.APIError
// @Router /api/ratings [get]
func (h *RatingsHandler) ListRatings(c *gin.Context) {
	req := &ratings.ListRatingsRequest{
		UserID:     getUserIDFromContext(c),
		EntityType: c.Query("entity_type"),
		Rating:     c.Query("rating"),
	}

	result, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list ratings")
		return
	}

	c.JSON(http.StatusOK, ListRatingsResponse{Ratings: result})
}

// CreateRating creates or updates a rating.
//
// @Summary Create or update rating
// @Description Creates or updates a rating for a media item
// @Tags ratings
// @Accept json
// @Produce json
// @Param request body ratings.CreateRatingRequest true "Rating request"
// @Success 200 {object} ratings.RatingDTO
// @Success 201 {object} ratings.RatingDTO
// @Failure 400 {object} handlers.APIError
// @Failure 500 {object} handlers.APIError
// @Router /api/ratings [post]
func (h *RatingsHandler) CreateRating(c *gin.Context) {
	var req ratings.CreateRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body")
		return
	}

	req.UserID = getUserIDFromContext(c)

	result, err := h.service.CreateOrUpdate(c.Request.Context(), &req)
	if err != nil {
		switch err {
		case ratingsDomain.ErrInvalidRating:
			respondError(c, http.StatusBadRequest, "INVALID_RATING", "Rating must be 'up', 'down', or 'favorite'")
		case ratingsDomain.ErrInvalidEntityType:
			respondError(c, http.StatusBadRequest, "INVALID_ENTITY_TYPE", "Entity type must be 'movie', 'tv_show', or 'tv_episode'")
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create rating")
		}
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetRating gets a specific rating.
//
// @Summary Get rating
// @Description Gets a specific rating for an entity
// @Tags ratings
// @Produce json
// @Param entity_type path string true "Entity type (movie, tv_show, tv_episode)"
// @Param entity_id path int true "Entity ID"
// @Success 200 {object} ratings.RatingDTO
// @Failure 404 {object} handlers.APIError
// @Failure 500 {object} handlers.APIError
// @Router /api/ratings/{entity_type}/{entity_id} [get]
func (h *RatingsHandler) GetRating(c *gin.Context) {
	entityType := c.Param("entity_type")
	entityIDStr := c.Param("entity_id")

	entityID, err := strconv.ParseInt(entityIDStr, 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ENTITY_ID", "Invalid entity ID")
		return
	}

	userID := getUserIDFromContext(c)

	result, err := h.service.Get(c.Request.Context(), userID, entityType, entityID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get rating")
		return
	}

	if result == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "Rating not found")
		return
	}

	c.JSON(http.StatusOK, result)
}

// DeleteRating deletes a rating.
//
// @Summary Delete rating
// @Description Deletes a rating for an entity
// @Tags ratings
// @Param entity_type path string true "Entity type (movie, tv_show, tv_episode)"
// @Param entity_id path int true "Entity ID"
// @Success 204
// @Failure 400 {object} handlers.APIError
// @Failure 500 {object} handlers.APIError
// @Router /api/ratings/{entity_type}/{entity_id} [delete]
func (h *RatingsHandler) DeleteRating(c *gin.Context) {
	entityType := c.Param("entity_type")
	entityIDStr := c.Param("entity_id")

	entityID, err := strconv.ParseInt(entityIDStr, 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ENTITY_ID", "Invalid entity ID")
		return
	}

	userID := getUserIDFromContext(c)

	if err := h.service.Delete(c.Request.Context(), userID, entityType, entityID); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete rating")
		return
	}

	c.Status(http.StatusNoContent)
}

// ListRatingsResponse is the response for listing ratings.
type ListRatingsResponse struct {
	Ratings []*ratings.RatingDTO `json:"ratings"`
}
