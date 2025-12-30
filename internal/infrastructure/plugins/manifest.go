package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Manifest represents the plugin.yml metadata file.
// This is read by the host before starting the plugin binary.
type Manifest struct {
	// Identity
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`

	// Authorship
	Author   string `yaml:"author"`
	License  string `yaml:"license"`
	Homepage string `yaml:"homepage"`

	// Compatibility
	MinHostVersion string `yaml:"min_host_version"`

	// Plugin type
	Categories []string `yaml:"categories"`

	// Enricher-specific capabilities
	Capabilities *ManifestCapabilities `yaml:"capabilities,omitempty"`

	// Service capabilities this plugin provides (for capability aliasing)
	// e.g., ["semantic_search", "similar_items"] creates /api/search and /api/similar aliases
	ServiceCapabilities []string `yaml:"service_capabilities,omitempty"`

	// Dependencies declares what capabilities this plugin requires from other plugins.
	// Used for dependency resolution and plugin load ordering.
	Dependencies []ManifestDependency `yaml:"dependencies,omitempty"`

	// Provides declares what capabilities this plugin offers to others.
	// e.g., ["provider:ollama", "provider"] for a provider plugin
	// Other plugins can depend on these capabilities.
	Provides []string `yaml:"provides,omitempty"`

	// Requires is a simpler alternative to Dependencies for declaring
	// required capabilities. Each entry is a capability name that must
	// be provided by another enabled plugin.
	// e.g., ["embedding"] for a search plugin that needs an embedding provider
	Requires []string `yaml:"requires,omitempty"`

	// Type describes the plugin type: "enricher", "provider", etc.
	Type string `yaml:"type,omitempty"`

	// MediaTypes lists the media types this enricher supports.
	// Alternative to Capabilities.MediaTypes for simpler manifests.
	MediaTypes []string `yaml:"media_types,omitempty"`

	// Required permissions
	Permissions []string `yaml:"permissions"`
}

// ManifestDependency declares a capability requirement for a plugin.
type ManifestDependency struct {
	// Capability is the required capability, e.g., "provider", "ai:search"
	Capability string `yaml:"capability"`

	// Required indicates if this dependency must be satisfied for the plugin to load.
	// If true and no plugin provides the capability, this plugin won't load.
	Required bool `yaml:"required"`
}

// ManifestCapabilities describes enricher capabilities in the manifest.
type ManifestCapabilities struct {
	MediaTypes []string `yaml:"media_types"`
	Provides   []string `yaml:"provides"`
	IsLocal    bool     `yaml:"is_local"`
	RateLimit  int      `yaml:"rate_limit"`
}

// LoadManifest reads and parses a plugin.yml file from the given directory.
func LoadManifest(pluginDir string) (*Manifest, error) {
	manifestPath := filepath.Join(pluginDir, "plugin.yml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin.yml: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse plugin.yml: %w", err)
	}

	// Validate required fields
	if manifest.ID == "" {
		return nil, fmt.Errorf("plugin.yml: id is required")
	}
	if manifest.Name == "" {
		return nil, fmt.Errorf("plugin.yml: name is required")
	}
	if manifest.Version == "" {
		return nil, fmt.Errorf("plugin.yml: version is required")
	}

	return &manifest, nil
}
