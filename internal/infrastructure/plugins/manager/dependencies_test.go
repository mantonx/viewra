package manager

import (
	"log/slog"
	"os"
	"testing"

	"github.com/mantonx/viewra/internal/infrastructure/plugins/manifest"
)

func TestManager_ResolveLoadOrder(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	m, err := NewManager(ManagerConfig{
		PluginDir:   tmpDir,
		HostVersion: "1.0.0",
	}, logger)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	tests := []struct {
		name                string
		pluginInfos         map[string]*pluginLoadInfo
		capabilityProviders map[string][]string
		wantOrder           []string // expected order (subset check)
		wantSkipped         []string // plugins that should be skipped
	}{
		{
			name: "no dependencies - all load",
			pluginInfos: map[string]*pluginLoadInfo{
				"plugin-a": {manifest: &manifest.Manifest{ID: "plugin-a", DisplayCategory: "other", Capabilities: []string{"metadata"}}},
				"plugin-b": {manifest: &manifest.Manifest{ID: "plugin-b", DisplayCategory: "other", Capabilities: []string{"metadata"}}},
				"plugin-c": {manifest: &manifest.Manifest{ID: "plugin-c", DisplayCategory: "other", Capabilities: []string{"metadata"}}},
			},
			capabilityProviders: map[string][]string{},
			wantOrder:           []string{"plugin-a", "plugin-b", "plugin-c"},
			wantSkipped:         nil,
		},
		{
			name: "simple dependency chain",
			pluginInfos: map[string]*pluginLoadInfo{
				"provider": {
					manifest: &manifest.Manifest{
						ID:              "provider",
						DisplayCategory: "providers",
						Capabilities:    []string{"embedding"},
					},
				},
				"consumer": {
					manifest: &manifest.Manifest{
						ID:              "consumer",
						DisplayCategory: "search",
						Capabilities:    []string{"search"},
						Requires:        []string{"embedding"},
					},
				},
			},
			capabilityProviders: map[string][]string{
				"embedding": {"provider"},
			},
			wantOrder:   []string{"provider", "consumer"}, // provider must be first
			wantSkipped: nil,
		},
		{
			name: "missing required dependency - skip",
			pluginInfos: map[string]*pluginLoadInfo{
				"consumer": {
					manifest: &manifest.Manifest{
						ID:              "consumer",
						DisplayCategory: "search",
						Capabilities:    []string{"search"},
						Requires:        []string{"embedding"},
					},
				},
			},
			capabilityProviders: map[string][]string{},
			wantOrder:           []string{},
			wantSkipped:         []string{"consumer"},
		},
		{
			name: "complex dependency graph",
			pluginInfos: map[string]*pluginLoadInfo{
				"base": {
					manifest: &manifest.Manifest{
						ID:              "base",
						DisplayCategory: "providers",
						Capabilities:    []string{"provider", "embedding"},
					},
				},
				"middle": {
					manifest: &manifest.Manifest{
						ID:              "middle",
						DisplayCategory: "search",
						Capabilities:    []string{"vector_search"},
						Requires:        []string{"embedding"},
					},
				},
				"top": {
					manifest: &manifest.Manifest{
						ID:              "top",
						DisplayCategory: "recommendations",
						Capabilities:    []string{"recommendations"},
						Requires:        []string{"vector_search"},
					},
				},
			},
			capabilityProviders: map[string][]string{
				"embedding":     {"base"},
				"vector_search": {"middle"},
			},
			wantOrder:   []string{"base", "middle", "top"}, // strict ordering
			wantSkipped: nil,
		},
		{
			name: "circular dependency detection",
			pluginInfos: map[string]*pluginLoadInfo{
				"plugin-a": {
					manifest: &manifest.Manifest{
						ID:              "plugin-a",
						DisplayCategory: "other",
						Capabilities:    []string{"metadata"},
						Requires:        []string{"artwork"},
					},
				},
				"plugin-b": {
					manifest: &manifest.Manifest{
						ID:              "plugin-b",
						DisplayCategory: "other",
						Capabilities:    []string{"artwork"},
						Requires:        []string{"metadata"},
					},
				},
			},
			capabilityProviders: map[string][]string{
				"metadata": {"plugin-a"},
				"artwork":  {"plugin-b"},
			},
			wantOrder:   []string{},
			wantSkipped: []string{"plugin-a", "plugin-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, skipped := m.resolveLoadOrder(tt.pluginInfos, tt.capabilityProviders)

			// Check that skipped plugins match
			for _, pluginID := range tt.wantSkipped {
				if _, ok := skipped[pluginID]; !ok {
					t.Errorf("expected plugin %s to be skipped, but it wasn't", pluginID)
				}
			}

			// For ordered tests, verify sequence
			if len(tt.wantOrder) > 0 {
				// For simple tests, verify all expected plugins are in the order
				for _, expected := range tt.wantOrder {
					found := false
					for _, got := range order {
						if got == expected {
							found = true
							break
						}
					}
					if !found && len(tt.wantSkipped) == 0 {
						t.Errorf("expected plugin %s in order, but it wasn't found. Got: %v", expected, order)
					}
				}

				// For dependency tests, verify that dependencies come before dependents
				if tt.name == "simple dependency chain" || tt.name == "complex dependency graph" {
					for i, pluginID := range order {
						info, ok := tt.pluginInfos[pluginID]
						if !ok {
							continue
						}
						for _, capability := range info.manifest.Requires {
							// Find the provider of this capability
							providers := tt.capabilityProviders[capability]
							for _, providerID := range providers {
								// Find provider's position
								providerPos := -1
								for j, p := range order {
									if p == providerID {
										providerPos = j
										break
									}
								}
								if providerPos == -1 {
									continue // provider not in order
								}
								if providerPos >= i {
									t.Errorf("dependency %s (pos %d) should come before %s (pos %d)",
										providerID, providerPos, pluginID, i)
								}
							}
						}
					}
				}
			}

			// Check count of ordered plugins
			expectedCount := len(tt.pluginInfos) - len(tt.wantSkipped)
			if len(order) != expectedCount {
				t.Errorf("expected %d plugins in order, got %d: %v", expectedCount, len(order), order)
			}
		})
	}
}
