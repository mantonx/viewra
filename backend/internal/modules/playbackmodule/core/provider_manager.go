package core

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/pkg/plugins"
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

	pm.logger.Info("registered transcoding provider",
		"id", info.ID,
		"name", info.Name,
		"priority", info.Priority)

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

	// Get capable providers
	candidates := pm.getCapableProviders(req)
	if len(candidates) == 0 {
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

	for _, provider := range pm.providers {
		// Check if provider supports the requested format
		formats := provider.GetSupportedFormats()
		for _, format := range formats {
			if format.Format == req.Container {
				capable = append(capable, provider)
				break
			}
		}
	}

	return capable
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
