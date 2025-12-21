// Package ai provides domain types for AI functionality including LLM providers,
// embeddings, and semantic search.
package ai

import "time"

// ProviderType represents the type of LLM provider.
type ProviderType string

const (
	ProviderOllama     ProviderType = "ollama"
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderOpenAI     ProviderType = "openai"
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderVoyage     ProviderType = "voyage"
)

// String returns the string representation of the provider type.
func (p ProviderType) String() string {
	return string(p)
}

// IsValid checks if the provider type is valid.
func (p ProviderType) IsValid() bool {
	switch p {
	case ProviderOllama, ProviderOpenRouter, ProviderOpenAI, ProviderAnthropic, ProviderVoyage:
		return true
	default:
		return false
	}
}

// SupportsEmbeddings returns true if the provider supports embedding generation.
func (p ProviderType) SupportsEmbeddings() bool {
	switch p {
	case ProviderOllama, ProviderOpenAI, ProviderVoyage:
		return true
	default:
		return false
	}
}

// SupportsChat returns true if the provider supports chat completion.
func (p ProviderType) SupportsChat() bool {
	switch p {
	case ProviderOllama, ProviderOpenRouter, ProviderOpenAI, ProviderAnthropic:
		return true
	default:
		return false
	}
}

// ProviderConfig holds configuration for an LLM provider.
type ProviderConfig struct {
	Type    ProviderType
	Model   string
	APIKey  string // Encrypted at rest
	BaseURL string // Optional, for Ollama or custom endpoints
}

// EmbeddingConfig holds configuration for embedding generation.
type EmbeddingConfig struct {
	Provider   ProviderType
	Model      string
	APIKey     string // Encrypted at rest, may differ from LLM provider
	Dimensions int    // Target dimension for normalization (e.g., 768)
}

// Message represents a chat message.
type Message struct {
	Role    Role
	Content string
}

// Role represents the role of a message sender.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ChatRequest represents a request to an LLM for chat completion.
type ChatRequest struct {
	Messages    []Message
	MaxTokens   int
	Temperature float64
	Stream      bool
}

// ChatResponse represents a response from an LLM.
type ChatResponse struct {
	Content      string
	FinishReason string
	Usage        TokenUsage
}

// TokenUsage tracks token consumption for cost tracking.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// EmbeddingRequest represents a request for text embeddings.
type EmbeddingRequest struct {
	Texts []string
}

// EmbeddingResponse represents the response containing embeddings.
type EmbeddingResponse struct {
	Embeddings [][]float32
	Usage      TokenUsage
}

// Embedding represents a single embedding vector with metadata.
type Embedding struct {
	ID         int64
	EntityType EntityType
	EntityID   int64
	Vector     []float32
	Text       string // Original text that was embedded
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// EntityType represents the type of entity that can be embedded.
type EntityType string

const (
	EntityMovie       EntityType = "movie"
	EntityTVShow      EntityType = "tv_show"
	EntityTVEpisode   EntityType = "tv_episode"
	EntityMusicArtist EntityType = "music_artist"
	EntityMusicAlbum  EntityType = "music_album"
	EntityMusicTrack  EntityType = "music_track"
)

// String returns the string representation of the entity type.
func (e EntityType) String() string {
	return string(e)
}

// SemanticSearchRequest represents a semantic search query.
type SemanticSearchRequest struct {
	Query     string       // Natural language query
	SimilarTo int64        // OR find similar to entity ID
	Types     []EntityType // Filter by entity types
	Limit     int
	Offset    int
}

// SemanticSearchResult represents a single search result.
type SemanticSearchResult struct {
	EntityType EntityType
	EntityID   int64
	Score      float32 // Similarity score (0-1, higher is better)
	Text       string  // The embedded text that matched
}

// SemanticSearchResponse represents the response from semantic search.
type SemanticSearchResponse struct {
	Results    []SemanticSearchResult
	TotalCount int
	Query      string
}

// MoodTag represents a mood/vibe tag for content.
type MoodTag struct {
	ID         int64
	MediaID    int64
	Tag        string
	Confidence float32 // 0-1, how confident the LLM was
	CreatedAt  time.Time
}

// MoodCategory represents categories of mood tags.
type MoodCategory string

const (
	MoodCategoryEmotionalTone MoodCategory = "emotional_tone"
	MoodCategoryEnergyLevel   MoodCategory = "energy_level"
	MoodCategorySocialContext MoodCategory = "social_context"
	MoodCategoryThemes        MoodCategory = "themes"
)

// Usage tracking is done via TokenUsage in responses.
// Cost estimation is intentionally NOT included in providers as pricing
// changes frequently. If cost tracking is needed, implement a separate
// pricing service that fetches current rates from OpenRouter's API or
// uses user-configured rates stored in ai_settings.

// PullProgress represents progress during a model pull operation.
type PullProgress struct {
	Status    string  `json:"status"`              // "pulling manifest", "downloading", "verifying", etc.
	Digest    string  `json:"digest,omitempty"`    // Layer digest being pulled
	Total     int64   `json:"total,omitempty"`     // Total bytes to download
	Completed int64   `json:"completed,omitempty"` // Bytes downloaded so far
	Percent   float64 `json:"percent,omitempty"`   // Completion percentage (0-100)
	Done      bool    `json:"done"`                // True when pull is complete
	Error     string  `json:"error,omitempty"`     // Error message if failed
}
