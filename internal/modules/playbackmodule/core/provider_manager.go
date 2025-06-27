package core

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/go-hclog"
	plugins "github.com/mantonx/viewra/sdk"
)

// ProviderManager manages multiple transcoding providers
type ProviderManager struct {
	mu           sync.RWMutex
	providers    map[string]plugins.TranscodingProvider
	priorities   map[string]int
	sessionStore *SessionStore
	logger       hclog.Logger
}

// NewProviderManager creates a new provider manager
func NewProviderManager(sessionStore *SessionStore, logger hclog.Logger) *ProviderManager {
	return &ProviderManager{
		providers:    make(map[string]plugins.TranscodingProvider),
		priorities:   make(map[string]int),
		sessionStore: sessionStore,
		logger:       logger.Named("provider-manager"),
	}
}

// RegisterProvider registers a transcoding provider
func (pm *ProviderManager) RegisterProvider(provider plugins.TranscodingProvider) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	info := provider.GetInfo()
	if info.ID == "" {
		return fmt.Errorf("provider ID cannot be empty")
	}

	pm.providers[info.ID] = provider
	pm.priorities[info.ID] = info.Priority

	pm.logger.Info("TRACE: Provider registered",
		"provider_manager_instance", fmt.Sprintf("%p", pm),
		"provider_id", info.ID,
		"provider_name", info.Name,
		"total_providers_after", len(pm.providers))

	return nil
}

// UnregisterProvider removes a transcoding provider
func (pm *ProviderManager) UnregisterProvider(providerID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.providers[providerID]; !exists {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	delete(pm.providers, providerID)
	delete(pm.priorities, providerID)

	pm.logger.Info("unregistered transcoding provider", "id", providerID)
	return nil
}

// GetProvider returns a specific provider
func (pm *ProviderManager) GetProvider(providerID string) (plugins.TranscodingProvider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	provider, exists := pm.providers[providerID]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	return provider, nil
}

// ListProviders returns all registered providers
func (pm *ProviderManager) ListProviders() []plugins.ProviderInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var infos []plugins.ProviderInfo
	for _, provider := range pm.providers {
		infos = append(infos, provider.GetInfo())
	}

	// Sort by priority (higher first)
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Priority > infos[j].Priority
	})

	return infos
}

// SelectProvider selects the best provider for a transcoding request
func (pm *ProviderManager) SelectProvider(ctx context.Context, req *plugins.TranscodeRequest) (plugins.TranscodingProvider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pm.logger.Info("TRACE: SelectProvider called",
		"provider_manager_instance", fmt.Sprintf("%p", pm),
		"total_providers", len(pm.providers),
		"requested_container", req.Container,
		"container_empty", req.Container == "",
		"request_input_path", req.InputPath)

	// Get capable providers
	candidates := pm.getCapableProviders(req)

	pm.logger.Debug("DEBUG: SelectProvider after getCapableProviders",
		"candidates_count", len(candidates),
		"requested_container", req.Container)

	if len(candidates) == 0 {
		pm.logger.Debug("DEBUG: No capable providers found",
			"total_providers", len(pm.providers),
			"requested_container", req.Container,
			"container_empty", req.Container == "")

		// Enhanced debugging for the root cause
		if req.Container == "" {
			pm.logger.Error("CRITICAL: Container field is empty in TranscodeRequest - this indicates a bug in request construction",
				"input_path", req.InputPath,
				"session_id", req.SessionID)
		}

		// If no providers are found, this might be a timing issue with plugin discovery
		// This is a safeguard to ensure the system is self-healing
		if len(pm.providers) == 0 {
			pm.logger.Warn("No providers registered at all - this suggests a plugin discovery issue")
		} else {
			// Log details about available providers when container is not empty but no match found
			if req.Container != "" {
				pm.logger.Warn("Providers are available but none support the requested format",
					"requested_format", req.Container,
					"available_providers", len(pm.providers))

				// Log what formats each provider supports
				for providerID, provider := range pm.providers {
					formats := provider.GetSupportedFormats()
					formatStrings := make([]string, len(formats))
					for i, format := range formats {
						formatStrings[i] = format.Format
					}
					pm.logger.Debug("Provider supported formats",
						"provider_id", providerID,
						"supported_formats", formatStrings)
				}
			}
		}

		return nil, fmt.Errorf("no capable providers found for format: %s", req.Container)
	}

	// Select optimal provider based on various factors
	provider := pm.selectOptimalProvider(candidates, req)
	if provider == nil {
		return nil, fmt.Errorf("failed to select provider")
	}

	pm.logger.Debug("selected provider for request",
		"provider", provider.GetInfo().ID,
		"container", req.Container,
		"quality", req.Quality)

	return provider, nil
}

// getCapableProviders returns providers that can handle the request
func (pm *ProviderManager) getCapableProviders(req *plugins.TranscodeRequest) []plugins.TranscodingProvider {
	var capable []plugins.TranscodingProvider

	pm.logger.Debug("DEBUG: getCapableProviders called",
		"provider_count", len(pm.providers),
		"requested_container", req.Container)

	for providerID, provider := range pm.providers {
		info := provider.GetInfo()
		pm.logger.Debug("DEBUG: checking provider",
			"provider_id", providerID,
			"provider_name", info.Name)

		// Check if provider supports the requested format
		formats := provider.GetSupportedFormats()
		pm.logger.Debug("DEBUG: provider formats",
			"provider_id", providerID,
			"format_count", len(formats))

		for _, format := range formats {
			pm.logger.Debug("DEBUG: checking format",
				"provider_id", providerID,
				"format", format.Format,
				"requested", req.Container)

			if format.Format == req.Container {
				pm.logger.Debug("DEBUG: provider supports format",
					"provider_id", providerID,
					"format", format.Format)
				capable = append(capable, provider)
				break
			}
		}
	}

	pm.logger.Debug("DEBUG: getCapableProviders result",
		"capable_count", len(capable),
		"requested_container", req.Container)

	return capable
}

// GetProviders returns the number of registered providers (for robustness checks)
func (pm *ProviderManager) GetProviders() map[string]plugins.TranscodingProvider {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Return a copy to prevent external modification
	providers := make(map[string]plugins.TranscodingProvider)
	for id, provider := range pm.providers {
		providers[id] = provider
	}
	return providers
}

// selectOptimalProvider selects the best provider from candidates
func (pm *ProviderManager) selectOptimalProvider(candidates []plugins.TranscodingProvider, req *plugins.TranscodeRequest) plugins.TranscodingProvider {
	if len(candidates) == 0 {
		return nil
	}

	// For now, simple selection based on:
	// 1. Hardware acceleration preference
	// 2. Current load (active sessions)
	// 3. Provider priority

	type scoredProvider struct {
		provider plugins.TranscodingProvider
		score    int
	}

	var scored []scoredProvider

	for _, provider := range candidates {
		score := 0
		info := provider.GetInfo()

		// Base score from priority
		score += info.Priority * 100

		// Bonus for hardware acceleration if requested
		if req.PreferHardware {
			accelerators := provider.GetHardwareAccelerators()
			for _, accel := range accelerators {
				if accel.Available && accel.Type == string(req.HardwareType) {
					score += 500 // Large bonus for matching hardware
					break
				}
			}
		}

		// Penalty for current load
		stats, err := pm.sessionStore.GetProviderStats(info.ID)
		if err == nil {
			// Reduce score based on active sessions
			score -= int(stats.ActiveSessions) * 10
		}

		scored = append(scored, scoredProvider{
			provider: provider,
			score:    score,
		})
	}

	// Sort by score (highest first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	return scored[0].provider
}

// GetProviderResources returns resource usage for all providers
func (pm *ProviderManager) GetProviderResources() map[string]ProviderResources {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	resources := make(map[string]ProviderResources)

	for id := range pm.providers {
		stats, err := pm.sessionStore.GetProviderStats(id)
		if err != nil {
			continue
		}

		resources[id] = ProviderResources{
			ActiveSessions: int(stats.ActiveSessions),
			// Other metrics would come from provider-specific monitoring
		}
	}

	return resources
}

// ProviderResources contains resource usage information
type ProviderResources struct {
	CPUUsage       float64
	MemoryUsage    int64
	GPUUsage       float64
	ActiveSessions int
	QueueLength    int
}
