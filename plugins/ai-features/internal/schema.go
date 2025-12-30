package internal

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// ModelOptions contains the available models for schema generation.
type ModelOptions struct {
	EmbeddingModels  []string
	ChatModels       []string
	DefaultEmbedding string
	DefaultChat      string
}

// SettingsSchema returns the JSON Schema for AI Local settings.
//
// This schema provides:
// - Master AI configuration (enabled toggle, provider selection)
// - Ollama-specific settings (server URL, models)
func SettingsSchema(opts ModelOptions) *sdk.Schema {
	// Build embedding model property
	embeddingModelProp := sdk.String().
		Title("Embedding Model").
		Description("Model to use for generating embeddings")
	if len(opts.EmbeddingModels) > 0 {
		embeddingModelProp.EnumStrings(opts.EmbeddingModels...)
		if opts.DefaultEmbedding != "" {
			embeddingModelProp.Default(opts.DefaultEmbedding)
		}
	} else {
		embeddingModelProp.Description("No embedding models installed. Go to the Models tab to pull one.")
	}

	// Build chat model property
	chatModelProp := sdk.String().
		Title("Chat Model").
		Description("Model to use for chat completions")
	if len(opts.ChatModels) > 0 {
		chatModelProp.EnumStrings(opts.ChatModels...)
		if opts.DefaultChat != "" {
			chatModelProp.Default(opts.DefaultChat)
		}
	} else {
		chatModelProp.Description("No chat models installed. Go to the Models tab to pull one.")
	}

	return sdk.NewSchema("AI Settings").
		Meta(sdk.PluginMeta{
			DisplayName:  "AI Features",
			ProviderName: "Ollama",
			Description:  "Configure AI-powered features like semantic search",
			Tip:          "Enable AI features and select which providers to use.",
			IsLocal:      true,
			Icon:         "brain",
		}).
		// --- Master AI Configuration ---
		// "enabled" is first and always visible (only in main settings view)
		Property("enabled", sdk.Boolean().
			Title("Enable AI Features").
			Description("Turn on AI-powered features like semantic search and recommendations").
			Default(false)).
		// Provider selection - only visible when enabled (only in main settings view)
		Property("embedding_provider", sdk.PluginRef("embedding").
			Title("Embedding Provider").
			Description("Select which plugin provides text embeddings for semantic search").
			DependsOn("enabled", true)).
		Property("chat_provider", sdk.PluginRef("chat").
			Title("Chat Provider").
			Description("Select which plugin provides chat/LLM capabilities").
			DependsOn("enabled", true)).
		// --- Ollama-specific settings ---
		// These ONLY appear when AI Features is selected as a provider (via capability filter)
		// They are NOT in the ai-config section, so they don't show in main view
		Property("base_url", sdk.String().
			Title("Ollama Server URL").
			Description("URL of the Ollama server").
			Default("http://localhost:11434")).
		Property("embedding_model", embeddingModelProp).
		Property("chat_model", chatModelProp).
		// --- Sections ---
		// Main section - ONLY these properties show when viewing AI Features directly
		Section(sdk.NewSection("main").
			Properties("enabled", "embedding_provider", "chat_provider").
			Actions("embedding-models", "chat-models")).
		// Embedding capability section - shown when AI Features is selected as embedding provider
		Section(sdk.NewSection("embedding").
			Properties("base_url", "embedding_model").
			Actions("test-connection").
			Capabilities("embedding")).
		// Chat capability section - shown when AI Features is selected as chat provider
		Section(sdk.NewSection("chat").
			Properties("base_url", "chat_model").
			Actions("test-connection").
			Capabilities("chat")).
		// Actions
		Action(sdk.TestAction("test-connection", "/health")).
		Action(modelListAction("embedding-models", "Ollama Embedding Models", "embedding", "embedding_provider")).
		Action(modelListAction("chat-models", "Ollama Chat Models", "chat", "chat_provider"))
}

// modelListAction creates a list action for managing models.
// The providerField parameter specifies which form field to check (e.g., "embedding_provider").
func modelListAction(id, tabTitle, modelType, providerField string) *sdk.ListActionDef {
	return sdk.ListAction(id, "/models/recommended").
		Title("Ollama Models").
		TabTitle(tabTitle).
		Params(map[string]string{"type": modelType}).
		ShowSystemInfo().
		// DependsOn controls when this tab appears - only when ai-features is selected as the provider
		DependsOn(providerField, "ai-features").
		Display(sdk.NewListDisplay("name").
			SecondaryField("description").
			Badge(sdk.NewBadge("installed", true, "Installed", "emerald")).
			Badge(sdk.NewBadge("canRun", false, "Insufficient Resources", "red")).
			Metadata("size", "minRam")).
		ItemAction(sdk.NewStreamingAction("pull", "Pull", "/models/pull").
			ShowWhen("installed", false)).
		ItemAction(sdk.NewDeleteAction("delete", "/models/:id").
			ShowWhen("installed", true).
			Confirm("Delete Model", "Are you sure you want to delete this model?"))
}
