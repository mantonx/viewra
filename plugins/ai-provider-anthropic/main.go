// Package main implements the Anthropic provider plugin for ViewRA.
// This plugin provides AI inference capabilities via Anthropic's Claude models.
package main

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/ai-provider-anthropic/internal"
)

func main() {
	hclogger, logger := sdk.NewLogger("ai-provider-anthropic")
	provider := internal.NewAnthropicProvider(logger)
	sdk.ServeProvider(provider, hclogger)
}
