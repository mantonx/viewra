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

	// Required permissions
	Permissions []string `yaml:"permissions"`
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
