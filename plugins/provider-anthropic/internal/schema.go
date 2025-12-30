package internal

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// SettingsSchema returns the JSON Schema for Anthropic plugin settings.
func SettingsSchema() *sdk.Schema {
	return sdk.NewSchema("Anthropic Settings").
		Meta(sdk.PluginMeta{
			DisplayName: "Anthropic",
			Description: "Claude AI for intelligent chat responses",
			Tip:         "Requires API key. Best-in-class reasoning capabilities.",
			IsLocal:     false,
			Icon:        "brain",
		}).
		Property("api_key", sdk.String().
			Title("API Key").
			Description("Your Anthropic API key").
			Format("password").
			Required()).
		Property("chat_model", sdk.String().
			Title("Chat Model").
			Description("Model to use for chat completions").
			Default("claude-sonnet-4-5-20250929").
			EnumStrings("claude-sonnet-4-5-20250929", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022", "claude-3-opus-20240229")).
		// Chat capability section - Anthropic only provides chat
		Section(sdk.NewSection("chat").
			Properties("api_key", "chat_model").
			Actions("test-connection").
			Capabilities("chat")).
		Action(sdk.TestAction("test-connection", "/health"))
}
