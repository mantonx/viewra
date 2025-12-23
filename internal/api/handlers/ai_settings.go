package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/settings"
	"github.com/mantonx/viewra/internal/domain/ai"
	"github.com/mantonx/viewra/internal/domain/events"
	"github.com/mantonx/viewra/internal/infrastructure/plugins"
)

// aiSettingsKeys maps JSON field names to settings keys for core AI settings.
var aiSettingsKeys = map[string]string{
	"enabled":             "ai.enabled",
	"embeddingProvider":   "ai.embedding_provider",
	"chatProvider":        "ai.chat_provider",
	"maxResults":          "ai.max_results",
	"similarityThreshold": "ai.similarity_threshold",
}

// AISettingsHandler handles AI configuration endpoints.
type AISettingsHandler struct {
	settingsService  *settings.Service
	publisher        events.Publisher
	providerRegistry *plugins.ProviderRegistry
}

// NewAISettingsHandler creates a new AI settings handler.
func NewAISettingsHandler(settingsService *settings.Service, publisher events.Publisher) *AISettingsHandler {
	return &AISettingsHandler{
		settingsService: settingsService,
		publisher:       publisher,
	}
}

// SetProviderRegistry sets the provider registry for provider lookup.
func (h *AISettingsHandler) SetProviderRegistry(registry *plugins.ProviderRegistry) {
	h.providerRegistry = registry
}

// --- Request/Response types ---

// AISettingsResponse represents the AI configuration response.
type AISettingsResponse struct {
	Enabled bool `json:"enabled"`

	// Provider selection
	EmbeddingProvider string `json:"embeddingProvider"`
	ChatProvider      string `json:"chatProvider"`

	// Search settings
	MaxResults          int    `json:"maxResults"`
	SimilarityThreshold string `json:"similarityThreshold"`
}

// AISettingsRequest represents the request to update AI settings.
type AISettingsRequest struct {
	Enabled *bool `json:"enabled,omitempty"`

	// Provider selection
	EmbeddingProvider *string `json:"embeddingProvider,omitempty"`
	ChatProvider      *string `json:"chatProvider,omitempty"`

	// Search settings
	MaxResults          *int    `json:"maxResults,omitempty"`
	SimilarityThreshold *string `json:"similarityThreshold,omitempty"`
}

type ProvidersResponse struct {
	Providers []ai.ProviderInfo `json:"providers"`
}

// --- Handlers ---

// GetAISettings handles GET /api/settings/ai
// @Summary Get AI settings
// @Description Returns AI configuration with sensitive values masked
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} AISettingsResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/settings/ai [get]
func (h *AISettingsHandler) GetAISettings(c *gin.Context) {
	ctx := c.Request.Context()

	resp := AISettingsResponse{
		Enabled:             h.settingsService.GetSystemBool(ctx, "ai.enabled", false),
		EmbeddingProvider:   h.settingsService.GetSystemString(ctx, "ai.embedding_provider", ""),
		ChatProvider:        h.settingsService.GetSystemString(ctx, "ai.chat_provider", ""),
		MaxResults:          h.settingsService.GetSystemInt(ctx, "ai.max_results", 0),
		SimilarityThreshold: h.settingsService.GetSystemString(ctx, "ai.similarity_threshold", ""),
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateAISettings handles PUT /api/settings/ai
// @Summary Update AI settings
// @Description Updates AI configuration
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param settings body AISettingsRequest true "AI settings to update"
// @Success 200 {object} AISettingsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/settings/ai [put]
func (h *AISettingsHandler) UpdateAISettings(c *gin.Context) {
	var req AISettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	ctx := c.Request.Context()
	updates := h.buildUpdatesMap(&req)

	for key, value := range updates {
		if err := h.settingsService.SetSystem(ctx, key, value, ""); err != nil {
			handleSettingsError(c, err)
			return
		}
	}

	if h.publisher != nil && len(updates) > 0 {
		event := events.NewEvent(events.EventSettingsChanged, "ai-settings-handler").WithCategory("ai")
		h.publisher.Publish(event.Build())
	}

	h.GetAISettings(c)
}

// GetProviders handles GET /api/settings/ai/providers
// @Summary Get available AI providers
// @Description Returns information about all available AI providers and their capabilities
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} ProvidersResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/settings/ai/providers [get]
func (h *AISettingsHandler) GetProviders(c *gin.Context) {
	if h.providerRegistry == nil {
		c.JSON(http.StatusOK, ProvidersResponse{Providers: []ai.ProviderInfo{}})
		return
	}

	providerInfos := h.getProvidersFromRegistry()
	c.JSON(http.StatusOK, ProvidersResponse{Providers: providerInfos})
}

// getProvidersFromRegistry converts registered providers to ProviderInfo.
func (h *AISettingsHandler) getProvidersFromRegistry() []ai.ProviderInfo {
	registered := h.providerRegistry.List()
	if len(registered) == 0 {
		return []ai.ProviderInfo{}
	}

	result := make([]ai.ProviderInfo, 0, len(registered))
	for _, p := range registered {
		caps := p.Capabilities
		info := ai.ProviderInfo{
			Type:              ai.ProviderType(caps.ProviderId),
			Name:              caps.DisplayName,
			Description:       caps.Description,
			SupportsEmbedding: caps.SupportsEmbeddings,
			SupportsChat:      caps.SupportsChat,
			RequiresAPIKey:    caps.RequiresApiKey,
			RequiresURL:       caps.RequiresUrl,
			IsPlugin:          true,
		}
		result = append(result, info)
	}
	return result
}

// --- Private helper methods ---

func (h *AISettingsHandler) buildUpdatesMap(req *AISettingsRequest) map[string]any {
	updates := make(map[string]any)

	if req.Enabled != nil {
		updates[aiSettingsKeys["enabled"]] = *req.Enabled
	}
	if req.EmbeddingProvider != nil {
		updates[aiSettingsKeys["embeddingProvider"]] = *req.EmbeddingProvider
	}
	if req.ChatProvider != nil {
		updates[aiSettingsKeys["chatProvider"]] = *req.ChatProvider
	}
	if req.MaxResults != nil {
		updates[aiSettingsKeys["maxResults"]] = *req.MaxResults
	}
	if req.SimilarityThreshold != nil {
		updates[aiSettingsKeys["similarityThreshold"]] = *req.SimilarityThreshold
	}

	return updates
}
