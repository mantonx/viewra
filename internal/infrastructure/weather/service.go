package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	openMeteoBaseURL = "https://api.open-meteo.com/v1/forecast"
	cacheTTL         = 30 * time.Minute
)

// Service provides weather context using the Open-Meteo API.
type Service struct {
	client *http.Client
	logger *slog.Logger

	// Cache
	mu    sync.RWMutex
	cache map[string]*cachedWeather
}

type cachedWeather struct {
	context   *WeatherContext
	fetchedAt time.Time
}

// NewService creates a new weather service.
func NewService(logger *slog.Logger) *Service {
	return &Service{
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
		cache:  make(map[string]*cachedWeather),
	}
}

// GetCurrentWeather fetches current weather for a location.
// Returns a context with Available=false if location is not provided.
func (s *Service) GetCurrentWeather(ctx context.Context, lat, lon float64, timezone string) (*WeatherContext, error) {
	if lat == 0 && lon == 0 {
		return &WeatherContext{Available: false}, nil
	}

	// Check cache
	cacheKey := fmt.Sprintf("%.4f,%.4f", lat, lon)
	s.mu.RLock()
	if cached, ok := s.cache[cacheKey]; ok {
		if time.Since(cached.fetchedAt) < cacheTTL {
			s.mu.RUnlock()
			return cached.context, nil
		}
	}
	s.mu.RUnlock()

	// Fetch from API
	if timezone == "" || timezone == "auto" {
		timezone = "auto"
	}

	url := fmt.Sprintf(
		"%s?latitude=%.4f&longitude=%.4f&current=temperature_2m,relative_humidity_2m,is_day,precipitation,cloud_cover,weather_code&timezone=%s",
		openMeteoBaseURL, lat, lon, timezone,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch weather: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo returned status %d", resp.StatusCode)
	}

	var apiResp openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	now := time.Now()
	weatherCtx := &WeatherContext{
		Available:     true,
		Temperature:   apiResp.Current.Temperature2m,
		Humidity:      apiResp.Current.RelativeHumidity2m,
		IsDay:         apiResp.Current.IsDay == 1,
		Precipitation: apiResp.Current.Precipitation,
		CloudCover:    apiResp.Current.CloudCover,
		WeatherCode:   apiResp.Current.WeatherCode,
		Condition:     weatherCodeToCondition(apiResp.Current.WeatherCode),
		TimeOfDay:     getTimeOfDay(apiResp.Current.IsDay == 1, now),
		Season:        getSeason(lat, now),
		FetchedAt:     now,
	}

	// Update cache
	s.mu.Lock()
	s.cache[cacheKey] = &cachedWeather{
		context:   weatherCtx,
		fetchedAt: now,
	}
	s.mu.Unlock()

	s.logger.Debug("fetched weather",
		"lat", lat,
		"lon", lon,
		"condition", weatherCtx.Condition,
		"temperature", weatherCtx.Temperature,
	)

	return weatherCtx, nil
}

// weatherCodeToCondition maps WMO weather codes to simple condition strings.
// See: https://open-meteo.com/en/docs (Weather interpretation codes)
func weatherCodeToCondition(code int) string {
	switch {
	case code == 0:
		return "sunny"
	case code >= 1 && code <= 3:
		return "cloudy"
	case code >= 45 && code <= 48:
		return "foggy"
	case code >= 51 && code <= 57:
		return "rainy" // Drizzle
	case code >= 61 && code <= 67:
		return "rainy" // Rain
	case code >= 71 && code <= 77:
		return "snowy"
	case code >= 80 && code <= 82:
		return "rainy" // Rain showers
	case code >= 85 && code <= 86:
		return "snowy" // Snow showers
	case code >= 95 && code <= 99:
		return "stormy"
	default:
		return "clear"
	}
}

// getTimeOfDay returns the current time of day category.
func getTimeOfDay(isDay bool, t time.Time) string {
	if !isDay {
		return "night"
	}

	hour := t.Hour()
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 21:
		return "evening"
	default:
		return "night"
	}
}

// getSeason returns the current season based on latitude and date.
func getSeason(lat float64, t time.Time) string {
	month := t.Month()

	// Southern hemisphere has reversed seasons
	southern := lat < 0

	switch {
	case month >= 3 && month <= 5:
		if southern {
			return "fall"
		}
		return "spring"
	case month >= 6 && month <= 8:
		if southern {
			return "winter"
		}
		return "summer"
	case month >= 9 && month <= 11:
		if southern {
			return "spring"
		}
		return "fall"
	default: // December, January, February
		if southern {
			return "summer"
		}
		return "winter"
	}
}

// ClearCache clears the weather cache.
func (s *Service) ClearCache() {
	s.mu.Lock()
	s.cache = make(map[string]*cachedWeather)
	s.mu.Unlock()
}
