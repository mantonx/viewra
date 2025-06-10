package services

import (
	"github.com/mantonx/viewra/data/plugins/tmdb_enricher_v2/internal/config"
	"github.com/mantonx/viewra/pkg/plugins"
)

// ConfigurableService defines the interface for services that support runtime configuration updates
type ConfigurableService interface {
	UpdateConfiguration(config *config.Config)
}

// PerformanceAware defines the interface for services that can use performance monitoring
type PerformanceAware interface {
	SetPerformanceMonitor(monitor *plugins.BasePerformanceMonitor)
}

// ServiceBase provides common functionality for all services
type ServiceBase interface {
	ConfigurableService
	PerformanceAware
}

// Compile-time checks to ensure all services implement required interfaces
var (
	_ ConfigurableService = (*EnrichmentService)(nil)
	_ ConfigurableService = (*MatchingService)(nil)
	_ ConfigurableService = (*ArtworkService)(nil)
)
