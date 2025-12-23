// Package main implements the Ollama provider plugin for ViewRA.
// This plugin provides local AI inference capabilities via Ollama.
package main

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/provider-ollama/internal"
)

func main() {
	hclogger, logger := sdk.NewLogger("provider-ollama")
	provider := internal.NewOllamaProvider(logger)
	sdk.ServeProvider(provider, hclogger)
}
