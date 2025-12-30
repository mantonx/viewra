// Package main implements the AI Local plugin for ViewRA.
// This plugin provides:
// - Master AI configuration (enable/disable, provider selection)
// - Local AI inference via Ollama
package main

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/ai-local/internal"
)

func main() {
	hclogger, logger := sdk.NewLogger("ai-local")
	plugin := internal.NewAILocalPlugin(logger)
	sdk.ServeProvider(plugin, hclogger)
}
