// Package main implements the AI Features plugin for ViewRA.
// This plugin provides:
// - Master AI configuration (enable/disable, provider selection)
// - Local AI inference via Ollama
package main

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/ai-features/internal"
)

func main() {
	hclogger, logger := sdk.NewLogger("ai-features")
	plugin := internal.NewAIFeaturesPlugin(logger)
	sdk.ServeProvider(plugin, hclogger)
}
