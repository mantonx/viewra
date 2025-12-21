package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/middleware"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/location"
	"github.com/mantonx/viewra/internal/infrastructure/weather"
)

// LocationSettingsHandler handles location preference HTTP requests.
type LocationSettingsHandler struct {
	locationRepo   *location.Repository
	weatherService *weather.Service
}

// NewLocationSettingsHandler creates a new location settings handler.
func NewLocationSettingsHandler(
	locationRepo *location.Repository,
	weatherService *weather.Service,
) *LocationSettingsHandler {
	return &LocationSettingsHandler{
		locationRepo:   locationRepo,
		weatherService: weatherService,
	}
}

// LocationPreferencesResponse represents a user's location preferences.
type LocationPreferencesResponse struct {
	Enabled      bool     `json:"enabled"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	Timezone     string   `json:"timezone"`
	LocationName string   `json:"location_name,omitempty"`
}

// UpdateLocationPreferencesRequest is the request body for updating location preferences.
type UpdateLocationPreferencesRequest struct {
	Enabled      *bool    `json:"enabled"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
	Timezone     *string  `json:"timezone"`
	LocationName *string  `json:"location_name"`
}

// WeatherContextResponse represents the current weather context.
type WeatherContextResponse struct {
	Available     bool    `json:"available"`
	Temperature   float64 `json:"temperature,omitempty"`
	Humidity      int     `json:"humidity,omitempty"`
	IsDay         bool    `json:"is_day,omitempty"`
	Condition     string  `json:"condition,omitempty"`
	TimeOfDay     string  `json:"time_of_day,omitempty"`
	Season        string  `json:"season,omitempty"`
	Precipitation float64 `json:"precipitation,omitempty"`
	CloudCover    int     `json:"cloud_cover,omitempty"`
}

// GetLocationPreferences handles GET /api/settings/location
// @Summary Get location preferences
// @Description Returns the current user's location preferences
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} LocationPreferencesResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/settings/location [get]
func (h *LocationSettingsHandler) GetLocationPreferences(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	prefs, err := h.locationRepo.Get(c.Request.Context(), userID)
	if err != nil {
		handleError(c, err)
		return
	}

	resp := LocationPreferencesResponse{
		Timezone: "auto",
	}

	if prefs != nil {
		resp.Enabled = prefs.Enabled
		resp.Timezone = prefs.Timezone
		resp.LocationName = prefs.LocationName
		if prefs.Latitude != 0 {
			resp.Latitude = &prefs.Latitude
		}
		if prefs.Longitude != 0 {
			resp.Longitude = &prefs.Longitude
		}
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateLocationPreferences handles PUT /api/settings/location
// @Summary Update location preferences
// @Description Updates the current user's location preferences
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body UpdateLocationPreferencesRequest true "Location preferences"
// @Success 200 {object} LocationPreferencesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/settings/location [put]
func (h *LocationSettingsHandler) UpdateLocationPreferences(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	var req UpdateLocationPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body: " + err.Error()})
		return
	}

	// Get existing preferences or create defaults
	prefs, err := h.locationRepo.Get(c.Request.Context(), userID)
	if err != nil {
		handleError(c, err)
		return
	}

	if prefs == nil {
		prefs = &location.UserLocationPreference{
			UserID:   userID,
			Timezone: "auto",
		}
	}

	// Apply updates
	if req.Enabled != nil {
		prefs.Enabled = *req.Enabled
	}
	if req.Latitude != nil {
		prefs.Latitude = *req.Latitude
	}
	if req.Longitude != nil {
		prefs.Longitude = *req.Longitude
	}
	if req.Timezone != nil {
		prefs.Timezone = *req.Timezone
	}
	if req.LocationName != nil {
		prefs.LocationName = *req.LocationName
	}

	// Clear location name if coordinates are cleared
	if prefs.Latitude == 0 && prefs.Longitude == 0 {
		prefs.LocationName = ""
	}

	// Save preferences
	if err := h.locationRepo.Upsert(c.Request.Context(), prefs); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save: " + err.Error()})
		return
	}

	resp := LocationPreferencesResponse{
		Enabled:      prefs.Enabled,
		Timezone:     prefs.Timezone,
		LocationName: prefs.LocationName,
	}
	if prefs.Latitude != 0 {
		resp.Latitude = &prefs.Latitude
	}
	if prefs.Longitude != 0 {
		resp.Longitude = &prefs.Longitude
	}

	c.JSON(http.StatusOK, resp)
}

// GetWeatherContext handles GET /api/context/weather
// @Summary Get current weather context
// @Description Returns weather context for the current user's location
// @Tags context
// @Security BearerAuth
// @Produce json
// @Success 200 {object} WeatherContextResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/context/weather [get]
func (h *LocationSettingsHandler) GetWeatherContext(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	prefs, err := h.locationRepo.Get(c.Request.Context(), userID)
	if err != nil {
		handleError(c, err)
		return
	}

	if prefs == nil || !prefs.Enabled || (prefs.Latitude == 0 && prefs.Longitude == 0) {
		c.JSON(http.StatusOK, WeatherContextResponse{Available: false})
		return
	}

	weatherCtx, err := h.weatherService.GetCurrentWeather(c.Request.Context(), prefs.Latitude, prefs.Longitude, prefs.Timezone)
	if err != nil {
		// Return unavailable rather than error
		c.JSON(http.StatusOK, WeatherContextResponse{Available: false})
		return
	}

	c.JSON(http.StatusOK, WeatherContextResponse{
		Available:     weatherCtx.Available,
		Temperature:   weatherCtx.Temperature,
		Humidity:      weatherCtx.Humidity,
		IsDay:         weatherCtx.IsDay,
		Condition:     weatherCtx.Condition,
		TimeOfDay:     weatherCtx.TimeOfDay,
		Season:        weatherCtx.Season,
		Precipitation: weatherCtx.Precipitation,
		CloudCover:    weatherCtx.CloudCover,
	})
}
