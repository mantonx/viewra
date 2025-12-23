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

// SettingsSchema returns the JSON Schema for Ollama plugin settings.
// It accepts model options to populate the dynamic dropdowns.
func SettingsSchema(opts ModelOptions) *sdk.Schema {
	// Build embedding model property
	embeddingProp := sdk.String().
		Title("Embedding Model").
		Description("Model to use for generating embeddings")
	if len(opts.EmbeddingModels) > 0 {
		embeddingProp.EnumStrings(opts.EmbeddingModels...)
		if opts.DefaultEmbedding != "" {
			embeddingProp.Default(opts.DefaultEmbedding)
		}
	} else {
		embeddingProp.Description("No embedding models installed. Go to the Models tab to pull one.")
	}

	// Build chat model property
	chatProp := sdk.String().
		Title("Chat Model").
		Description("Model to use for chat completions")
	if len(opts.ChatModels) > 0 {
		chatProp.EnumStrings(opts.ChatModels...)
		if opts.DefaultChat != "" {
			chatProp.Default(opts.DefaultChat)
		}
	} else {
		chatProp.Description("No chat models installed. Go to the Models tab to pull one.")
	}

	return sdk.NewSchema("Ollama Settings").
		Meta(sdk.PluginMeta{
			DisplayName: "Ollama",
			Description: "Local AI inference using Ollama",
			Tip:         "Runs on your hardware. Larger models need more RAM/VRAM.",
			IsLocal:     true,
			Icon:        "hard-drive",
		}).
		Property("base_url", sdk.String().
			Title("Server URL").
			Description("URL of the Ollama server").
			Default("http://localhost:11434")).
		Property("embedding_model", embeddingProp).
		Property("chat_model", chatProp).
		// Sections for capability-based UI filtering
		Section(sdk.NewSection("connection").
			Properties("base_url").
			Actions("test-connection").
			Capabilities("embedding", "chat")).
		Section(sdk.NewSection("embedding-models").
			Properties("embedding_model").
			Actions("embedding-models").
			Capabilities("embedding")).
		Section(sdk.NewSection("chat-models").
			Properties("chat_model").
			Actions("chat-models").
			Capabilities("chat")).
		// Actions
		Action(sdk.TestAction("test-connection", "/health")).
		Action(modelListAction("embedding-models", "Embedding Models", "embedding")).
		Action(modelListAction("chat-models", "Chat Models", "chat"))
}

// modelListAction creates a list action for managing models.
func modelListAction(id, title, modelType string) *sdk.ListActionDef {
	return sdk.ListAction(id, "/models/recommended").
		Title(title).
		TabTitle(title).
		Params(map[string]string{"type": modelType}).
		ShowSystemInfo().
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
