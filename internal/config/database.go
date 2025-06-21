package config

import (
	"fmt"
)

// GetDatabaseURL returns the database URL for plugins and other components
func GetDatabaseURL() string {
	cfg := Get().Database

	// Generate URL if not explicitly set
	if cfg.URL != "" {
		return cfg.URL
	}

	switch cfg.Type {
	case "sqlite":
		return "sqlite://" + cfg.DatabasePath
	case "postgres":
		return buildPostgresURL(cfg)
	default:
		return "sqlite://" + cfg.DatabasePath
	}
}

// buildPostgresURL builds a PostgreSQL connection URL from config
func buildPostgresURL(cfg DatabaseFullConfig) string {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.Username == "" {
		cfg.Username = "viewra"
	}
	if cfg.Database == "" {
		cfg.Database = "viewra"
	}

	url := "postgres://"
	if cfg.Username != "" {
		url += cfg.Username
		if cfg.Password != "" {
			url += ":" + cfg.Password
		}
		url += "@"
	}
	url += cfg.Host
	if cfg.Port != 5432 {
		url += fmt.Sprintf(":%d", cfg.Port)
	}
	url += "/" + cfg.Database

	return url
}
