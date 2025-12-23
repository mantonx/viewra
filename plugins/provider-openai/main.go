// Package main implements the OpenAI provider plugin for ViewRA.
// This plugin provides AI inference capabilities via OpenAI's API.
package main

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/provider-openai/internal"
)

func main() {
	hclogger, logger := sdk.NewLogger("provider-openai")
	provider := internal.NewOpenAIProvider(logger)
	sdk.ServeProvider(provider, hclogger)
}
