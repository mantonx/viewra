package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/settings"
	"github.com/mantonx/viewra/internal/domain/ai"
	"github.com/mantonx/viewra/internal/domain/events"
	settingsDomain "github.com/mantonx/viewra/internal/domain/settings"
	"github.com/mantonx/viewra/internal/infrastructure/ai/providers"
)

// AISettingsHandler handles AI configuration endpoints.
type AISettingsHandler struct {
	settingsService       *settings.Service
	publisher             events.Publisher
	factory               *providers.Factory
	recommendationService *settings.ModelRecommendationService
}

// NewAISettingsHandler creates a new AI settings handler.
func NewAISettingsHandler(settingsService *settings.Service, publisher events.Publisher) *AISettingsHandler {
	return &AISettingsHandler{
		settingsService: settingsService,
		publisher:       publisher,
		factory:         providers.NewFactory(),
	}
}

// SetSystemInfoProvider sets the function to retrieve system RAM and VRAM.
func (h *AISettingsHandler) SetSystemInfoProvider(fn func() (ramBytes, vramBytes uint64)) {
	h.recommendationService = settings.NewModelRecommendationService(fn)
}

// --- Request/Response types ---

// AISettingsResponse represents the AI configuration response.
type AISettingsResponse struct {
	Enabled bool `json:"enabled"`

	// Embedding settings
	EmbeddingProvider    string `json:"embeddingProvider"`
	OllamaEmbeddingModel string `json:"ollamaEmbeddingModel"`
	OpenAIEmbeddingModel string `json:"openaiEmbeddingModel"`
	VoyageEmbeddingModel string `json:"voyageEmbeddingModel"`

	// Chat settings
	ChatProvider        string `json:"chatProvider"`
	OllamaChatModel     string `json:"ollamaChatModel"`
	OpenAIChatModel     string `json:"openaiChatModel"`
	AnthropicChatModel  string `json:"anthropicChatModel"`
	OpenRouterChatModel string `json:"openrouterChatModel"`

	// Shared settings
	OllamaURL string `json:"ollamaUrl"`

	// API keys (masked)
	OpenAIAPIKey     string `json:"openaiApiKey"`
	AnthropicAPIKey  string `json:"anthropicApiKey"`
	VoyageAPIKey     string `json:"voyageApiKey"`
	OpenRouterAPIKey string `json:"openrouterApiKey"`

	// Search settings
	MaxResults          int    `json:"maxResults"`
	SimilarityThreshold string `json:"similarityThreshold"`

	// Status
	OllamaAvailable bool `json:"ollamaAvailable"`
}

// AISettingsRequest represents the request to update AI settings.
type AISettingsRequest struct {
	Enabled *bool `json:"enabled,omitempty"`

	// Embedding settings
	EmbeddingProvider    *string `json:"embeddingProvider,omitempty"`
	OllamaEmbeddingModel *string `json:"ollamaEmbeddingModel,omitempty"`
	OpenAIEmbeddingModel *string `json:"openaiEmbeddingModel,omitempty"`
	VoyageEmbeddingModel *string `json:"voyageEmbeddingModel,omitempty"`

	// Chat settings
	ChatProvider        *string `json:"chatProvider,omitempty"`
	OllamaChatModel     *string `json:"ollamaChatModel,omitempty"`
	OpenAIChatModel     *string `json:"openaiChatModel,omitempty"`
	AnthropicChatModel  *string `json:"anthropicChatModel,omitempty"`
	OpenRouterChatModel *string `json:"openrouterChatModel,omitempty"`

	// Shared settings
	OllamaURL *string `json:"ollamaUrl,omitempty"`

	// API keys
	OpenAIAPIKey     *string `json:"openaiApiKey,omitempty"`
	AnthropicAPIKey  *string `json:"anthropicApiKey,omitempty"`
	VoyageAPIKey     *string `json:"voyageApiKey,omitempty"`
	OpenRouterAPIKey *string `json:"openrouterApiKey,omitempty"`

	// Search settings
	MaxResults          *int    `json:"maxResults,omitempty"`
	SimilarityThreshold *string `json:"similarityThreshold,omitempty"`
}

type ProvidersResponse struct {
	Providers []ai.ProviderInfo `json:"providers"`
}

type ModelsResponse struct {
	Models []ai.ModelInfo `json:"models"`
}

type ConnectionTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type PullOllamaModelRequest struct {
	Model string `json:"model" binding:"required"`
}

type DeleteOllamaModelRequest struct {
	Model string `json:"model" binding:"required"`
}

// --- Provider configuration ---

// providerConfig maps provider types to their settings keys.
var providerConfig = map[ai.ProviderType]struct {
	keyOrURL   string
	isURL      bool
	keyName    string
	modelParam string
}{
	ai.ProviderOllama:     {"ai.ollama_url", true, "Ollama URL", ""},
	ai.ProviderOpenAI:     {"ai.openai_api_key", false, "OpenAI API key", ""},
	ai.ProviderAnthropic:  {"ai.anthropic_api_key", false, "Anthropic API key", "claude-sonnet-4-5-20250929"},
	ai.ProviderVoyage:     {"ai.voyage_api_key", false, "Voyage AI API key", "voyage-3-lite"},
	ai.ProviderOpenRouter: {"ai.openrouter_api_key", false, "OpenRouter API key", ""},
}

// --- Settings field mapping for updates ---

type settingField struct {
	key      string
	isAPIKey bool
}

var settingsFieldMap = map[string]settingField{
	"enabled":              {"ai.enabled", false},
	"embeddingProvider":    {"ai.embedding_provider", false},
	"ollamaEmbeddingModel": {"ai.ollama_embedding_model", false},
	"openaiEmbeddingModel": {"ai.openai_embedding_model", false},
	"voyageEmbeddingModel": {"ai.voyage_embedding_model", false},
	"chatProvider":         {"ai.chat_provider", false},
	"ollamaChatModel":      {"ai.ollama_chat_model", false},
	"openaiChatModel":      {"ai.openai_chat_model", false},
	"anthropicChatModel":   {"ai.anthropic_chat_model", false},
	"openrouterChatModel":  {"ai.openrouter_chat_model", false},
	"ollamaUrl":            {"ai.ollama_url", false},
	"openaiApiKey":         {"ai.openai_api_key", true},
	"anthropicApiKey":      {"ai.anthropic_api_key", true},
	"voyageApiKey":         {"ai.voyage_api_key", true},
	"openrouterApiKey":     {"ai.openrouter_api_key", true},
	"maxResults":           {"ai.max_results", false},
	"similarityThreshold":  {"ai.similarity_threshold", false},
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
		Enabled:              h.getBool(ctx, "ai.enabled"),
		EmbeddingProvider:    h.getString(ctx, "ai.embedding_provider"),
		OllamaEmbeddingModel: h.getString(ctx, "ai.ollama_embedding_model"),
		OpenAIEmbeddingModel: h.getString(ctx, "ai.openai_embedding_model"),
		VoyageEmbeddingModel: h.getString(ctx, "ai.voyage_embedding_model"),
		ChatProvider:         h.getString(ctx, "ai.chat_provider"),
		OllamaChatModel:      h.getString(ctx, "ai.ollama_chat_model"),
		OpenAIChatModel:      h.getString(ctx, "ai.openai_chat_model"),
		AnthropicChatModel:   h.getString(ctx, "ai.anthropic_chat_model"),
		OpenRouterChatModel:  h.getString(ctx, "ai.openrouter_chat_model"),
		OllamaURL:            h.getString(ctx, "ai.ollama_url"),
		OpenAIAPIKey:         h.getMasked(ctx, "ai.openai_api_key"),
		AnthropicAPIKey:      h.getMasked(ctx, "ai.anthropic_api_key"),
		VoyageAPIKey:         h.getMasked(ctx, "ai.voyage_api_key"),
		OpenRouterAPIKey:     h.getMasked(ctx, "ai.openrouter_api_key"),
		MaxResults:           h.getInt(ctx, "ai.max_results"),
		SimilarityThreshold:  h.getString(ctx, "ai.similarity_threshold"),
		OllamaAvailable:      h.checkOllamaAvailable(ctx),
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
	c.JSON(http.StatusOK, ProvidersResponse{Providers: providers.GetAllProviders()})
}

// ListModels handles GET /api/settings/ai/providers/:provider/models
// @Summary List available models for a provider
// @Description Returns list of models available for the specified provider
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Param provider path string true "Provider type (ollama, openai, anthropic, voyage, openrouter)"
// @Success 200 {object} ModelsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse "Provider unavailable"
// @Router /api/settings/ai/providers/{provider}/models [get]
func (h *AISettingsHandler) ListModels(c *gin.Context) {
	providerType := ai.ProviderType(c.Param("provider"))
	ctx := c.Request.Context()

	apiKeyOrURL, err := h.getProviderCredential(ctx, providerType)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_provider", Message: err.Error()})
		return
	}

	models, err := h.factory.ListAvailableModels(ctx, providerType, apiKeyOrURL)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "provider_unavailable", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ModelsResponse{Models: models})
}

// TestConnection handles POST /api/settings/ai/providers/:provider/test
// @Summary Test provider connection
// @Description Tests connectivity to the specified provider
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Param provider path string true "Provider type (ollama, openai, anthropic, voyage, openrouter)"
// @Success 200 {object} ConnectionTestResponse
// @Failure 400 {object} ErrorResponse
// @Failure 503 {object} ConnectionTestResponse
// @Router /api/settings/ai/providers/{provider}/test [post]
func (h *AISettingsHandler) TestConnection(c *gin.Context) {
	providerType := ai.ProviderType(c.Param("provider"))
	ctx := c.Request.Context()

	provider, err := h.createProvider(ctx, providerType)
	if err != nil {
		c.JSON(http.StatusBadRequest, ConnectionTestResponse{Success: false, Error: err.Error()})
		return
	}

	if err := provider.HealthCheck(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, ConnectionTestResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ConnectionTestResponse{
		Success: true,
		Message: fmt.Sprintf("%s is accessible", providerType),
	})
}

// --- Ollama-specific endpoints ---

// ListOllamaModels handles GET /api/settings/ai/models (legacy endpoint)
// @Summary List available Ollama models
// @Description Returns list of models available on the Ollama server
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} ModelsResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse "Ollama unavailable"
// @Router /api/settings/ai/models [get]
func (h *AISettingsHandler) ListOllamaModels(c *gin.Context) {
	ctx := c.Request.Context()

	provider := h.createOllamaProvider(ctx)
	models, err := provider.ListModels(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "ollama_unavailable", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ModelsResponse{Models: models})
}

// PullOllamaModel handles POST /api/settings/ai/models/pull
// @Summary Pull an Ollama model
// @Description Initiates download of an Ollama model with SSE progress streaming
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce text/event-stream
// @Param request body PullOllamaModelRequest true "Model to pull"
// @Success 200 "SSE stream of pull progress"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse "Ollama unavailable"
// @Router /api/settings/ai/models/pull [post]
func (h *AISettingsHandler) PullOllamaModel(c *gin.Context) {
	var req PullOllamaModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	ctx := c.Request.Context()
	provider := h.createOllamaProvider(ctx)

	// Set up SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	progress, err := provider.Pull(ctx, req.Model)
	if err != nil {
		h.writeSSE(c, "error", map[string]string{"error": err.Error()})
		return
	}

	for p := range progress {
		if err := h.writeSSE(c, "progress", p); err != nil {
			return
		}
		if p.Done || p.Error != "" {
			break
		}
	}
}

// DeleteOllamaModel handles DELETE /api/settings/ai/models
// @Summary Delete an Ollama model
// @Description Removes a model from the local Ollama installation
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body DeleteOllamaModelRequest true "Model to delete"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse "Ollama unavailable"
// @Router /api/settings/ai/models [delete]
func (h *AISettingsHandler) DeleteOllamaModel(c *gin.Context) {
	var req DeleteOllamaModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	ctx := c.Request.Context()
	provider := h.createOllamaProvider(ctx)

	if err := provider.DeleteModel(ctx, req.Model); err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "ollama_error", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Model deleted successfully"})
}

// GetRecommendedModels handles GET /api/settings/ai/models/recommended
// @Summary Get recommended Ollama models
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
// @Summary Get recommended Ollama chat models
// @Description Returns chat model recommendations based on system RAM/VRAM
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} settings.ModelRecommendations
// @Router /api/settings/ai/models/recommended/chat [get]
func (h *AISettingsHandler) GetRecommendedChatModels(c *gin.Context) {
	h.getRecommendations(c, true)
}

// --- Legacy endpoints for backward compatibility ---

// TestOllamaConnection handles POST /api/settings/ai/test/ollama (legacy)
// @Summary Test Ollama connection
// @Description Tests connectivity to the Ollama server
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} ConnectionTestResponse
// @Failure 503 {object} ConnectionTestResponse
// @Router /api/settings/ai/test/ollama [post]
func (h *AISettingsHandler) TestOllamaConnection(c *gin.Context) {
	c.Params = append(c.Params, gin.Param{Key: "provider", Value: "ollama"})
	h.TestConnection(c)
}

// TestOpenAIConnection handles POST /api/settings/ai/test/openai (legacy)
// @Summary Test OpenAI connection
// @Description Tests connectivity to OpenAI with the configured API key
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} ConnectionTestResponse
// @Failure 503 {object} ConnectionTestResponse
// @Router /api/settings/ai/test/openai [post]
func (h *AISettingsHandler) TestOpenAIConnection(c *gin.Context) {
	c.Params = append(c.Params, gin.Param{Key: "provider", Value: "openai"})
	h.TestConnection(c)
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
	lister := h.createOllamaProvider(ctx)

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

func (h *AISettingsHandler) createOllamaProvider(ctx context.Context) *providers.OllamaProvider {
	return providers.NewOllamaProvider(h.getString(ctx, "ai.ollama_url"), "")
}

func (h *AISettingsHandler) getProviderCredential(ctx context.Context, providerType ai.ProviderType) (string, error) {
	config, ok := providerConfig[providerType]
	if !ok {
		return "", fmt.Errorf("unknown provider: %s", providerType)
	}

	if config.isURL {
		return h.getString(ctx, config.keyOrURL), nil
	}
	return h.getString(ctx, config.keyOrURL), nil
}

func (h *AISettingsHandler) createProvider(ctx context.Context, providerType ai.ProviderType) (ai.HealthChecker, error) {
	config, ok := providerConfig[providerType]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", providerType)
	}

	var credential string
	if config.isURL {
		credential = h.getString(ctx, config.keyOrURL)
	} else {
		credential = h.getString(ctx, config.keyOrURL)
		if credential == "" {
			return nil, fmt.Errorf("%s not configured", config.keyName)
		}
	}

	switch providerType {
	case ai.ProviderOllama:
		return providers.NewOllamaProvider(credential, ""), nil
	case ai.ProviderOpenAI:
		return providers.NewOpenAIProvider(credential, ""), nil
	case ai.ProviderAnthropic:
		return providers.NewAnthropicProvider(credential, config.modelParam), nil
	case ai.ProviderVoyage:
		return providers.NewVoyageProvider(credential, config.modelParam), nil
	case ai.ProviderOpenRouter:
		return providers.NewOpenRouterProvider(credential, ""), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerType)
	}
}

func (h *AISettingsHandler) buildUpdatesMap(req *AISettingsRequest) map[string]any {
	updates := make(map[string]any)

	addIfSet := func(fieldName string, value any) {
		if field, ok := settingsFieldMap[fieldName]; ok {
			if field.isAPIKey {
				if s, ok := value.(string); ok && isMaskedValue(s) {
					return // Skip masked values
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
	if req.OllamaEmbeddingModel != nil {
		addIfSet("ollamaEmbeddingModel", *req.OllamaEmbeddingModel)
	}
	if req.OpenAIEmbeddingModel != nil {
		addIfSet("openaiEmbeddingModel", *req.OpenAIEmbeddingModel)
	}
	if req.VoyageEmbeddingModel != nil {
		addIfSet("voyageEmbeddingModel", *req.VoyageEmbeddingModel)
	}
	if req.ChatProvider != nil {
		addIfSet("chatProvider", *req.ChatProvider)
	}
	if req.OllamaChatModel != nil {
		addIfSet("ollamaChatModel", *req.OllamaChatModel)
	}
	if req.OpenAIChatModel != nil {
		addIfSet("openaiChatModel", *req.OpenAIChatModel)
	}
	if req.AnthropicChatModel != nil {
		addIfSet("anthropicChatModel", *req.AnthropicChatModel)
	}
	if req.OpenRouterChatModel != nil {
		addIfSet("openrouterChatModel", *req.OpenRouterChatModel)
	}
	if req.OllamaURL != nil {
		addIfSet("ollamaUrl", *req.OllamaURL)
	}
	if req.OpenAIAPIKey != nil {
		addIfSet("openaiApiKey", *req.OpenAIAPIKey)
	}
	if req.AnthropicAPIKey != nil {
		addIfSet("anthropicApiKey", *req.AnthropicAPIKey)
	}
	if req.VoyageAPIKey != nil {
		addIfSet("voyageApiKey", *req.VoyageAPIKey)
	}
	if req.OpenRouterAPIKey != nil {
		addIfSet("openrouterApiKey", *req.OpenRouterAPIKey)
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

func (h *AISettingsHandler) getMasked(ctx context.Context, key string) string {
	if val, _ := h.settingsService.GetSystemValueMasked(ctx, key); val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func (h *AISettingsHandler) checkOllamaAvailable(ctx context.Context) bool {
	provider := h.createOllamaProvider(ctx)
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return provider.HealthCheck(checkCtx) == nil
}

func (h *AISettingsHandler) writeSSE(c *gin.Context, eventType string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, jsonData)
	c.Writer.Flush()
	return err
}

func isMaskedValue(val string) bool {
	return val == "" || val == "••••••••••••" || val == "********"
}
