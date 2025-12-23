package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/application/settings"
	"github.com/mantonx/viewra/internal/domain/ai"
	"github.com/mantonx/viewra/internal/domain/events"
	settingsDomain "github.com/mantonx/viewra/internal/domain/settings"
	"github.com/mantonx/viewra/internal/infrastructure/plugins"
)

// AISettingsHandler handles AI configuration endpoints.
type AISettingsHandler struct {
	settingsService       *settings.Service
	publisher             events.Publisher
	recommendationService *settings.ModelRecommendationService
	providerRegistry      *plugins.ProviderRegistry
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

// SetSystemInfoProvider sets the function to retrieve system RAM and VRAM.
func (h *AISettingsHandler) SetSystemInfoProvider(fn func() (ramBytes, vramBytes uint64)) {
	h.recommendationService = settings.NewModelRecommendationService(fn)
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

// --- Settings field mapping for updates ---

type settingField struct {
	key      string
	isAPIKey bool
}

var settingsFieldMap = map[string]settingField{
	"enabled":             {"ai.enabled", false},
	"embeddingProvider":   {"ai.embedding_provider", false},
	"chatProvider":        {"ai.chat_provider", false},
	"maxResults":          {"ai.max_results", false},
	"similarityThreshold": {"ai.similarity_threshold", false},
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
		Enabled:             h.getBool(ctx, "ai.enabled"),
		EmbeddingProvider:   h.getString(ctx, "ai.embedding_provider"),
		ChatProvider:        h.getString(ctx, "ai.chat_provider"),
		MaxResults:          h.getInt(ctx, "ai.max_results"),
		SimilarityThreshold: h.getString(ctx, "ai.similarity_threshold"),
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

// GetRecommendedModels handles GET /api/settings/ai/models/recommended
// @Summary Get recommended models for embedding
// @Description Returns model recommendations based on system RAM/VRAM
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} settings.ModelRecommendations
// @Router /api/settings/ai/models/recommended [get]
func (h *AISettingsHandler) GetRecommendedModels(c *gin.Context) {
	h.getRecommendations(c, false)
}

// GetRecommendedChatModels handles GET /api/settings/ai/models/recommended/chat
// @Summary Get recommended chat models
// @Description Returns chat model recommendations based on system RAM/VRAM
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} settings.ModelRecommendations
// @Router /api/settings/ai/models/recommended/chat [get]
func (h *AISettingsHandler) GetRecommendedChatModels(c *gin.Context) {
	h.getRecommendations(c, true)
}

// --- Private helper methods ---

func (h *AISettingsHandler) getRecommendations(c *gin.Context, forChat bool) {
	if h.recommendationService == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error:   "service_unavailable",
			Message: "Model recommendation service not configured",
		})
		return
	}

	ctx := c.Request.Context()

	// Get Ollama provider from registry to list installed models
	var lister settings.ModelLister
	if h.providerRegistry != nil {
		if provider := h.providerRegistry.Get("ollama"); provider != nil {
			lister = &pluginModelLister{client: provider.Client}
		}
	}

	if lister == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error:   "provider_unavailable",
			Message: "Ollama provider not available",
		})
		return
	}

	var recommendations *settings.ModelRecommendations
	var err error
	if forChat {
		recommendations, err = h.recommendationService.GetChatRecommendations(ctx, lister)
	} else {
		recommendations, err = h.recommendationService.GetEmbeddingRecommendations(ctx, lister)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "recommendation_error", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, recommendations)
}

func (h *AISettingsHandler) buildUpdatesMap(req *AISettingsRequest) map[string]any {
	updates := make(map[string]any)

	addIfSet := func(fieldName string, value any) {
		if field, ok := settingsFieldMap[fieldName]; ok {
			if field.isAPIKey {
				if s, ok := value.(string); ok && isMaskedValue(s) {
					return
				}
			}
			updates[field.key] = value
		}
	}

	if req.Enabled != nil {
		addIfSet("enabled", *req.Enabled)
	}
	if req.EmbeddingProvider != nil {
		addIfSet("embeddingProvider", *req.EmbeddingProvider)
	}
	if req.ChatProvider != nil {
		addIfSet("chatProvider", *req.ChatProvider)
	}
	if req.MaxResults != nil {
		addIfSet("maxResults", *req.MaxResults)
	}
	if req.SimilarityThreshold != nil {
		addIfSet("similarityThreshold", *req.SimilarityThreshold)
	}

	return updates
}

func (h *AISettingsHandler) getSetting(ctx context.Context, key string) any {
	val, err := h.settingsService.GetSystemValue(ctx, key)
	if err != nil {
		if def := settingsDomain.GetSystemDefinition(key); def != nil {
			return def.Default
		}
		return nil
	}
	return val
}

func (h *AISettingsHandler) getString(ctx context.Context, key string) string {
	if s, ok := h.getSetting(ctx, key).(string); ok {
		return s
	}
	return ""
}

func (h *AISettingsHandler) getBool(ctx context.Context, key string) bool {
	if b, ok := h.getSetting(ctx, key).(bool); ok {
		return b
	}
	return false
}

func (h *AISettingsHandler) getInt(ctx context.Context, key string) int {
	switch v := h.getSetting(ctx, key).(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

func isMaskedValue(val string) bool {
	return val == "" || val == "••••••••••••" || val == "********"
}

// pluginModelLister adapts a provider plugin client to the ModelLister interface.
type pluginModelLister struct {
	client pluginv1.PluginProviderClient
}

func (l *pluginModelLister) ListModels(ctx context.Context) ([]ai.ModelInfo, error) {
	resp, err := l.client.ListModels(ctx, &pluginv1.Empty{})
	if err != nil {
		return nil, err
	}

	models := make([]ai.ModelInfo, len(resp.Models))
	for i, m := range resp.Models {
		models[i] = ai.ModelInfo{
			ID:          m.Id,
			Name:        m.Name,
			Description: m.Description,
			Size:        m.Size,
			IsChat:      m.IsChat,
			IsEmbedding: m.IsEmbedding,
		}
	}
	return models, nil
}
