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
			Default("text-embedding-3-small").
			EnumStrings("text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002")).
		Property("chat_model", sdk.String().
			Title("Chat Model").
			Description("Model to use for chat completions").
			Default("gpt-4o-mini").
			EnumStrings("gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4", "gpt-3.5-turbo")).
		// Embedding capability section - shown when OpenAI is selected as embedding provider
		Section(sdk.NewSection("embedding").
			Properties("api_key", "base_url", "embedding_model").
			Actions("test-connection").
			Capabilities("embedding")).
		// Chat capability section - shown when OpenAI is selected as chat provider
		Section(sdk.NewSection("chat").
			Properties("api_key", "base_url", "chat_model").
			Actions("test-connection").
			Capabilities("chat")).
		Action(sdk.TestAction("test-connection", "/health"))
}
