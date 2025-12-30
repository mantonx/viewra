package plugins

import "github.com/mantonx/viewra/internal/infrastructure/plugins/manifest"

// Type aliases for backward compatibility.
// New code should import the manifest package directly.

// Manifest represents the plugin.yml metadata file.
// Deprecated: Import from github.com/mantonx/viewra/internal/infrastructure/plugins/manifest instead.
type Manifest = manifest.Manifest

// ManifestDependency declares a capability requirement for a plugin.
// Deprecated: Import from github.com/mantonx/viewra/internal/infrastructure/plugins/manifest instead.
type ManifestDependency = manifest.Dependency

// ManifestCapabilities describes enricher capabilities in the manifest.
// Deprecated: Import from github.com/mantonx/viewra/internal/infrastructure/plugins/manifest instead.
type ManifestCapabilities = manifest.Capabilities

// LoadManifest reads and parses a plugin.yml file from the given directory.
// Deprecated: Use manifest.Load instead.
func LoadManifest(pluginDir string) (*Manifest, error) {
	return manifest.Load(pluginDir)
}
