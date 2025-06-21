package bootstrap

// This package handles bootstrapping of core plugins.
// It's separate from both the pluginmodule registry (which handles the mechanics)
// and the individual plugins (which register themselves).

import (
	// Import all core plugins to trigger their factory registration
	_ "github.com/mantonx/viewra/internal/plugins/enrichment"
	_ "github.com/mantonx/viewra/internal/plugins/ffmpeg"
	_ "github.com/mantonx/viewra/internal/plugins/moviestructure"
	_ "github.com/mantonx/viewra/internal/plugins/tvstructure"
)

// LoadCorePlugins ensures all core plugins are loaded by importing this package.
// This function exists to provide an explicit way to trigger plugin loading.
func LoadCorePlugins() {
	// The real work happens in the init() functions of the imported packages above.
	// This function serves as documentation and explicit intent.
}
