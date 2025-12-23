package internal

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// SettingsSchema returns the JSON Schema for OpenAI plugin settings.
func SettingsSchema() *sdk.Schema {
	return sdk.NewSchema("OpenAI Settings").
		Meta(sdk.PluginMeta{
			DisplayName: "OpenAI",
			Description: "OpenAI API for embeddings and chat",
			Tip:         "Requires API key. Usage is billed per token.",
			IsLocal:     false,
			Icon:        "cloud",
		}).
		Property("api_key", sdk.String().
			Title("API Key").
			Description("Your OpenAI API key").
			Format("password").
			Required()).
		Property("base_url", sdk.String().
			Title("Base URL").
			Description("Custom API base URL (optional, for OpenAI-compatible providers)").
			Default("")).
		Property("embedding_model", sdk.String().
			Title("Embedding Model").
			Description("Model to use for generating embeddings").
			Default("text-embedding-3-small")).
		Property("chat_model", sdk.String().
			Title("Chat Model").
			Description("Model to use for chat completions").
			Default("gpt-4o-mini")).
		Action(sdk.TestAction("test-connection", "/health"))
}
