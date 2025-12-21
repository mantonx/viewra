package plugins

import (
	"context"
	"log/slog"
	"strconv"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/location"
	"github.com/mantonx/viewra/internal/infrastructure/weather"
)

// HostWeatherServer implements the HostWeather gRPC service.
type HostWeatherServer struct {
	pluginv1.UnimplementedHostWeatherServer
	locationRepo   *location.Repository
	weatherService *weather.Service
	logger         *slog.Logger
}

// NewHostWeatherServer creates a new HostWeatherServer.
func NewHostWeatherServer(locationRepo *location.Repository, weatherService *weather.Service, logger *slog.Logger) *HostWeatherServer {
	return &HostWeatherServer{
		locationRepo:   locationRepo,
		weatherService: weatherService,
		logger:         logger,
	}
}

// GetCurrentWeather returns current weather for a user's location.
func (s *HostWeatherServer) GetCurrentWeather(ctx context.Context, req *pluginv1.WeatherRequest) (*pluginv1.WeatherResponse, error) {
	// Parse user ID from string to int64
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		s.logger.Debug("invalid user_id format", "user_id", req.UserId, "error", err)
		return &pluginv1.WeatherResponse{Available: false}, nil
	}

	// Get user's location preferences using SQLC-based repository
	prefs, err := s.locationRepo.Get(ctx, userID)
	if err != nil {
		s.logger.Debug("failed to get location preferences", "user_id", userID, "error", err)
		return &pluginv1.WeatherResponse{Available: false}, nil
	}

	if prefs == nil || !prefs.Enabled || (prefs.Latitude == 0 && prefs.Longitude == 0) {
		return &pluginv1.WeatherResponse{Available: false}, nil
	}

	// Fetch weather
	weatherCtx, err := s.weatherService.GetCurrentWeather(ctx, prefs.Latitude, prefs.Longitude, prefs.Timezone)
	if err != nil {
		s.logger.Warn("failed to fetch weather", "error", err)
		return &pluginv1.WeatherResponse{Available: false}, nil
	}

	return &pluginv1.WeatherResponse{
		Available:     weatherCtx.Available,
		Temperature:   float32(weatherCtx.Temperature),
		Humidity:      int32(weatherCtx.Humidity),
		IsDay:         weatherCtx.IsDay,
		Precipitation: float32(weatherCtx.Precipitation),
		CloudCover:    int32(weatherCtx.CloudCover),
		WeatherCode:   int32(weatherCtx.WeatherCode),
		Condition:     weatherCtx.Condition,
		TimeOfDay:     weatherCtx.TimeOfDay,
		Season:        weatherCtx.Season,
	}, nil
}

// Verify interface implementation
var _ pluginv1.HostWeatherServer = (*HostWeatherServer)(nil)
