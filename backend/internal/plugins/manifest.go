// Package plugins provides helper functions for plugin manifest handling.
package plugins

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ReadPluginManifestFile reads a plugin manifest file (YAML only) and returns the parsed manifest.
func ReadPluginManifestFile(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var manifest PluginManifest
	
	// Parse YAML
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse YAML manifest: %w", err)
	}
	
	// Validate required fields
	if manifest.ID == "" || manifest.Name == "" || manifest.Version == "" {
		return nil, fmt.Errorf("invalid manifest: missing required fields")
	}
	
	return &manifest, nil
}
