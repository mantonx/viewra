package internal

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// SettingsSchema returns the JSON Schema for Voyage AI plugin settings.
func SettingsSchema() *sdk.Schema {
	return sdk.NewSchema("Voyage AI Settings").
		Meta(sdk.PluginMeta{
			DisplayName: "Voyage AI",
			Description: "High-quality embeddings for semantic search",
			Tip:         "Requires API key. Specialized for embedding generation.",
			IsLocal:     false,
			Icon:        "compass",
		}).
		Property("api_key", sdk.String().
			Title("API Key").
			Description("Your Voyage AI API key").
			Format("password").
			Required()).
		Property("embedding_model", sdk.String().
			Title("Embedding Model").
			Description("Model to use for generating embeddings").
			Default("voyage-3-lite").
			EnumStrings("voyage-3-lite", "voyage-3", "voyage-3-large", "voyage-code-3")).
		// Embedding capability section - Voyage AI only provides embeddings
		Section(sdk.NewSection("embedding").
			Properties("api_key", "embedding_model").
			Actions("test-connection").
			Capabilities("embedding")).
		Action(sdk.TestAction("test-connection", "/health"))
}
