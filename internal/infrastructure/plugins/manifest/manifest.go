// Package manifest provides parsing and validation for plugin.yml manifest files.
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a plugin.yml file from the given directory.
func Load(pluginDir string) (*Manifest, error) {
	manifestPath := filepath.Join(pluginDir, "plugin.yml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin.yml: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse plugin.yml: %w", err)
	}

	// Validate required fields (trim whitespace before checking)
	if strings.TrimSpace(m.ID) == "" {
		return nil, fmt.Errorf("plugin.yml: id is required")
	}
	if strings.TrimSpace(m.Name) == "" {
		return nil, fmt.Errorf("plugin.yml: name is required")
	}
	if strings.TrimSpace(m.Version) == "" {
		return nil, fmt.Errorf("plugin.yml: version is required")
	}
	if strings.TrimSpace(m.DisplayCategory) == "" {
		return nil, fmt.Errorf("plugin.yml: display_category is required")
	}
	if !IsValidDisplayCategory(m.DisplayCategory) {
		return nil, fmt.Errorf("plugin.yml: invalid display_category %q (valid: %v)", m.DisplayCategory, displayCategoryIDs())
	}
	if len(m.Capabilities) == 0 {
		return nil, fmt.Errorf("plugin.yml: capabilities is required")
	}
	for _, cap := range m.Capabilities {
		if !IsValidCapability(cap) {
			return nil, fmt.Errorf("plugin.yml: invalid capability %q (valid: %v, or provider:*)", cap, ValidCapabilities)
		}
	}
	for _, req := range m.Requires {
		if !IsValidCapability(req) {
			return nil, fmt.Errorf("plugin.yml: invalid required capability %q (valid: %v, or provider:*)", req, ValidCapabilities)
		}
	}

	return &m, nil
}
