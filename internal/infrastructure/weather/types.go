// Package weather provides weather context for search features.
package weather

import "time"

// WeatherContext contains all weather and time context for query enrichment.
type WeatherContext struct {
	Available bool `json:"available"` // False if user hasn't enabled location

	// Current conditions
	Temperature   float64 `json:"temperature"`   // Celsius
	Humidity      int     `json:"humidity"`      // Percentage
	IsDay         bool    `json:"is_day"`        // True if daylight
	Precipitation float64 `json:"precipitation"` // mm
	CloudCover    int     `json:"cloud_cover"`   // Percentage
	WeatherCode   int     `json:"weather_code"`  // WMO weather code

	// Derived context
	Condition string `json:"condition"`   // sunny, cloudy, rainy, snowy, stormy, foggy
	TimeOfDay string `json:"time_of_day"` // morning, afternoon, evening, night
	Season    string `json:"season"`      // spring, summer, fall, winter

	// Cache info
	FetchedAt time.Time `json:"fetched_at"`
}

// UserLocationPreference stores a user's location settings.
type UserLocationPreference struct {
	UserID    string    `json:"user_id"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Timezone  string    `json:"timezone"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// openMeteoResponse represents the API response from Open-Meteo.
type openMeteoResponse struct {
	Current struct {
		Time               string  `json:"time"`
		Temperature2m      float64 `json:"temperature_2m"`
		RelativeHumidity2m int     `json:"relative_humidity_2m"`
		IsDay              int     `json:"is_day"`
		Precipitation      float64 `json:"precipitation"`
		CloudCover         int     `json:"cloud_cover"`
		WeatherCode        int     `json:"weather_code"`
	} `json:"current"`
	Timezone string `json:"timezone"`
}
