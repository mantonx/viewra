package main

import (
	"log"

	plugins "github.com/mantonx/viewra/sdk"
)

// ExampleEnricher embeds BasePlugin and implements enrichment functionality
type ExampleEnricher struct {
	*plugins.BasePlugin
}

// NewExampleEnricher creates a new example enricher plugin
func NewExampleEnricher() *ExampleEnricher {
	base := plugins.NewBasePlugin(
		"example_enricher",
		"1.0.0",
		"enrichment",
		"Example enrichment plugin for demonstration",
	)
	
	return &ExampleEnricher{
		BasePlugin: base,
	}
}

// Initialize the plugin
func (e *ExampleEnricher) Initialize(ctx *plugins.PluginContext) error {
	log.Printf("Initializing example enricher plugin")
	return nil
}

// Start the plugin
func (e *ExampleEnricher) Start() error {
	log.Printf("Starting example enricher plugin")
	return nil
}

// Add any enrichment-specific functionality here
// This is where you would implement MetadataScraperService or other interfaces

func main() {
	enricher := NewExampleEnricher()
	
	// Use the SDK's StartPlugin helper
	plugins.StartPlugin(enricher)
}