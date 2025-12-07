package handlers

import (
	"crypto/rand"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
)

// AdaptiveQualityHandler handles adaptive quality recommendation requests.
type AdaptiveQualityHandler struct {
	recommender *profile.QualityRecommender
	logger      *slog.Logger
}

// NewAdaptiveQualityHandler creates a new adaptive quality handler.
func NewAdaptiveQualityHandler(logger *slog.Logger) *AdaptiveQualityHandler {
	return &AdaptiveQualityHandler{
		recommender: profile.NewQualityRecommender(logger),
		logger:      logger,
	}
}

// RecommendQualityRequest represents client capability data for quality recommendation.
type RecommendQualityRequest struct {
	// Network
	NetworkSpeedMbps float64 `json:"networkSpeedMbps" binding:"required,min=0"`
	ConnectionType   string  `json:"connectionType"`
	IsMetered        bool    `json:"isMetered"`

	// Device
	DeviceType   string  `json:"deviceType" binding:"required"`
	ScreenWidth  int     `json:"screenWidth" binding:"required,min=1"`
	ScreenHeight int     `json:"screenHeight" binding:"required,min=1"`
	PixelRatio   float64 `json:"pixelRatio"`

	// Performance
	CPUCores     int     `json:"cpuCores"`
	MemoryGB     float64 `json:"memoryGB"`
	LowPowerMode bool    `json:"lowPowerMode"`
	BatteryLevel float64 `json:"batteryLevel"`
	IsCharging   bool    `json:"isCharging"`

	// Media capabilities
	SupportedCodecs      []string `json:"supportedCodecs"`
	HardwareAcceleration bool     `json:"hardwareAcceleration"`
	MaxDecodingProfile   string   `json:"maxDecodingProfile"`
}

// QualityRecommendationResponse represents a quality recommendation.
type QualityRecommendationResponse struct {
	ProfileID          string   `json:"profileId"`
	DisplayName        string   `json:"displayName"`
	Width              int      `json:"width"`
	Height             int      `json:"height"`
	VideoBitrate       int      `json:"videoBitrate"`
	AudioBitrate       int      `json:"audioBitrate"`
	DataUsageMBPerHour int      `json:"dataUsageMBPerHour"`
	Score              float64  `json:"score"`
	Reason             string   `json:"reason"`
	PreferredCodec     string   `json:"preferredCodec"`
	FallbackCodecs     []string `json:"fallbackCodecs"`
}

// AdaptiveLadderResponse represents an ABR ladder for adaptive streaming.
type AdaptiveLadderResponse struct {
	Profiles []QualityProfile `json:"profiles"`
}

// QualityProfile represents a simplified quality profile for the ladder.
type QualityProfile struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	VideoBitrate     int    `json:"videoBitrate"`
	MinNetworkMbps   float64 `json:"minNetworkMbps"`
	PreferredCodec   string `json:"preferredCodec"`
}

// RecommendQuality handles POST /api/adaptive/recommend
// @Summary Recommend optimal quality profile
// @Description Analyzes client capabilities and recommends the best quality profile
// @Tags adaptive
// @Accept json
// @Produce json
// @Param request body RecommendQualityRequest true "Client capabilities"
// @Success 200 {object} QualityRecommendationResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/adaptive/recommend [post]
func (h *AdaptiveQualityHandler) RecommendQuality(c *gin.Context) {
	var req RecommendQualityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid quality recommendation request", "error", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request",
			Message: err.Error(),
		})
		return
	}

	// Convert to internal capabilities struct
	caps := profile.ClientCapabilities{
		NetworkSpeedMbps:     req.NetworkSpeedMbps,
		ConnectionType:       req.ConnectionType,
		IsMetered:            req.IsMetered,
		DeviceType:           req.DeviceType,
		ScreenWidth:          req.ScreenWidth,
		ScreenHeight:         req.ScreenHeight,
		PixelRatio:           req.PixelRatio,
		CPUCores:             req.CPUCores,
		MemoryGB:             req.MemoryGB,
		LowPowerMode:         req.LowPowerMode,
		BatteryLevel:         req.BatteryLevel,
		IsCharging:           req.IsCharging,
		SupportedCodecs:      req.SupportedCodecs,
		HardwareAcceleration: req.HardwareAcceleration,
		MaxDecodingProfile:   req.MaxDecodingProfile,
	}

	// Get recommendation
	recommendation := h.recommender.RecommendQuality(caps)

	// Build response
	response := QualityRecommendationResponse{
		ProfileID:          recommendation.Profile.ID,
		DisplayName:        recommendation.Profile.DisplayName,
		Width:              recommendation.Profile.Width,
		Height:             recommendation.Profile.Height,
		VideoBitrate:       recommendation.Profile.VideoBitrate,
		AudioBitrate:       recommendation.Profile.AudioBitrate,
		DataUsageMBPerHour: recommendation.Profile.DataUsageMBPerHour,
		Score:              recommendation.Score,
		Reason:             recommendation.Reason,
		PreferredCodec:     recommendation.Profile.PreferredCodec,
		FallbackCodecs:     recommendation.Profile.FallbackCodecs,
	}

	c.JSON(http.StatusOK, response)
}

// GetAdaptiveLadder handles POST /api/adaptive/ladder
// @Summary Get adaptive bitrate ladder
// @Description Returns a set of quality profiles for adaptive streaming (ABR)
// @Tags adaptive
// @Accept json
// @Produce json
// @Param request body RecommendQualityRequest true "Client capabilities"
// @Success 200 {object} AdaptiveLadderResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/adaptive/ladder [post]
func (h *AdaptiveQualityHandler) GetAdaptiveLadder(c *gin.Context) {
	var req RecommendQualityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid adaptive ladder request", "error", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid request",
			Message: err.Error(),
		})
		return
	}

	// Convert to internal capabilities struct
	caps := profile.ClientCapabilities{
		NetworkSpeedMbps:     req.NetworkSpeedMbps,
		ConnectionType:       req.ConnectionType,
		IsMetered:            req.IsMetered,
		DeviceType:           req.DeviceType,
		ScreenWidth:          req.ScreenWidth,
		ScreenHeight:         req.ScreenHeight,
		PixelRatio:           req.PixelRatio,
		CPUCores:             req.CPUCores,
		MemoryGB:             req.MemoryGB,
		LowPowerMode:         req.LowPowerMode,
		BatteryLevel:         req.BatteryLevel,
		IsCharging:           req.IsCharging,
		SupportedCodecs:      req.SupportedCodecs,
		HardwareAcceleration: req.HardwareAcceleration,
		MaxDecodingProfile:   req.MaxDecodingProfile,
	}

	// Get adaptive ladder
	ladder := h.recommender.GetAdaptiveLadder(caps)

	// Build response
	profiles := make([]QualityProfile, len(ladder))
	for i, profile := range ladder {
		profiles[i] = QualityProfile{
			ID:             profile.ID,
			DisplayName:    profile.DisplayName,
			Width:          profile.Width,
			Height:         profile.Height,
			VideoBitrate:   profile.VideoBitrate,
			MinNetworkMbps: profile.MinNetworkMbps,
			PreferredCodec: profile.PreferredCodec,
		}
	}

	response := AdaptiveLadderResponse{
		Profiles: profiles,
	}

	c.JSON(http.StatusOK, response)
}

// SpeedTestChunk handles GET /api/speedtest/chunk
// @Summary Download speed test chunk
// @Description Returns a chunk of random data for client-side speed testing
// @Tags adaptive
// @Produce application/octet-stream
// @Param size query int false "Chunk size in bytes" default(1048576)
// @Success 200 {file} binary
// @Router /api/speedtest/chunk [get]
func (h *AdaptiveQualityHandler) SpeedTestChunk(c *gin.Context) {
	// Default to 1MB chunk
	chunkSize := 1024 * 1024

	// Allow custom size (max 10MB for safety)
	if size := c.Query("size"); size != "" {
		if parsedSize, err := parseInt(size); err == nil && parsedSize > 0 && parsedSize <= 10*1024*1024 {
			chunkSize = parsedSize
		}
	}

	// Generate random data
	data := make([]byte, chunkSize)
	if _, err := rand.Read(data); err != nil {
		h.logger.Error("failed to generate speed test chunk", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal server error",
			Message: "failed to generate test data",
		})
		return
	}

	// Set headers for speed testing
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// Add server timing header for latency measurement
	c.Header("Server-Timing", "processing;dur=0")

	c.Data(http.StatusOK, "application/octet-stream", data)
}
