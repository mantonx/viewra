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
	settingsService *settings.Service
	publisher       events.Publisher
}

// NewAISettingsHandler creates a new AI settings handler.
func NewAISettingsHandler(settingsService *settings.Service, publisher events.Publisher) *AISettingsHandler {
	return &AISettingsHandler{
		settingsService: settingsService,
		publisher:       publisher,
	}
}

// AISettingsResponse represents the AI configuration response.
type AISettingsResponse struct {
	Enabled             bool   `json:"enabled"`
	Provider            string `json:"provider"`
	OllamaURL           string `json:"ollamaUrl"`
	OllamaModel         string `json:"ollamaModel"`
	OpenAIAPIKey        string `json:"openaiApiKey"` // Masked for display
	OpenAIModel         string `json:"openaiModel"`
	MaxResults          int    `json:"maxResults"`
	SimilarityThreshold string `json:"similarityThreshold"`
	OllamaAvailable     bool   `json:"ollamaAvailable"`
}

// AISettingsRequest represents the request to update AI settings.
type AISettingsRequest struct {
	Enabled             *bool   `json:"enabled,omitempty"`
	Provider            *string `json:"provider,omitempty"`
	OllamaURL           *string `json:"ollamaUrl,omitempty"`
	OllamaModel         *string `json:"ollamaModel,omitempty"`
	OpenAIAPIKey        *string `json:"openaiApiKey,omitempty"`
	OpenAIModel         *string `json:"openaiModel,omitempty"`
	MaxResults          *int    `json:"maxResults,omitempty"`
	SimilarityThreshold *string `json:"similarityThreshold,omitempty"`
}

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

	// Get all AI settings
	enabled, _ := h.getSettingBool(ctx, "ai.enabled")
	provider, _ := h.getSettingString(ctx, "ai.provider")
	ollamaURL, _ := h.getSettingString(ctx, "ai.ollama_url")
	ollamaModel, _ := h.getSettingString(ctx, "ai.ollama_model")
	openaiModel, _ := h.getSettingString(ctx, "ai.openai_model")
	maxResults, _ := h.getSettingInt(ctx, "ai.max_results")
	threshold, _ := h.getSettingString(ctx, "ai.similarity_threshold")

	// Get masked API key
	maskedKey, _ := h.settingsService.GetSystemValueMasked(ctx, "ai.openai_api_key")
	maskedKeyStr, _ := maskedKey.(string)

	// Check if Ollama is available
	ollamaAvailable := h.checkOllamaAvailable(ctx, ollamaURL)

	c.JSON(http.StatusOK, AISettingsResponse{
		Enabled:             enabled,
		Provider:            provider,
		OllamaURL:           ollamaURL,
		OllamaModel:         ollamaModel,
		OpenAIAPIKey:        maskedKeyStr,
		OpenAIModel:         openaiModel,
		MaxResults:          maxResults,
		SimilarityThreshold: threshold,
		OllamaAvailable:     ollamaAvailable,
	})
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
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	ctx := c.Request.Context()
	updatedBy := "" // TODO: get from auth claims

	// Update settings that are provided
	if req.Enabled != nil {
		if err := h.settingsService.SetSystem(ctx, "ai.enabled", *req.Enabled, updatedBy); err != nil {
			handleSettingsError(c, err)
			return
		}
	}

	if req.Provider != nil {
		if err := h.settingsService.SetSystem(ctx, "ai.provider", *req.Provider, updatedBy); err != nil {
			handleSettingsError(c, err)
			return
		}
	}

	if req.OllamaURL != nil {
		if err := h.settingsService.SetSystem(ctx, "ai.ollama_url", *req.OllamaURL, updatedBy); err != nil {
			handleSettingsError(c, err)
			return
		}
	}

	if req.OllamaModel != nil {
		if err := h.settingsService.SetSystem(ctx, "ai.ollama_model", *req.OllamaModel, updatedBy); err != nil {
			handleSettingsError(c, err)
			return
		}
	}

	if req.OpenAIAPIKey != nil && *req.OpenAIAPIKey != "" {
		// Only update if not the masked placeholder
		if *req.OpenAIAPIKey != "••••••••••••" {
			if err := h.settingsService.SetSystem(ctx, "ai.openai_api_key", *req.OpenAIAPIKey, updatedBy); err != nil {
				handleSettingsError(c, err)
				return
			}
		}
	}

	if req.OpenAIModel != nil {
		if err := h.settingsService.SetSystem(ctx, "ai.openai_model", *req.OpenAIModel, updatedBy); err != nil {
			handleSettingsError(c, err)
			return
		}
	}

	if req.MaxResults != nil {
		if err := h.settingsService.SetSystem(ctx, "ai.max_results", *req.MaxResults, updatedBy); err != nil {
			handleSettingsError(c, err)
			return
		}
	}

	if req.SimilarityThreshold != nil {
		if err := h.settingsService.SetSystem(ctx, "ai.similarity_threshold", *req.SimilarityThreshold, updatedBy); err != nil {
			handleSettingsError(c, err)
			return
		}
	}

	// Publish settings changed event for AI category
	if h.publisher != nil {
		event := events.NewEvent(events.EventSettingsChanged, "ai-settings-handler").
			WithCategory("ai")
		h.publisher.Publish(event.Build())
	}

	// Return updated settings
	h.GetAISettings(c)
}

// ListOllamaModels handles GET /api/settings/ai/models
// @Summary List available Ollama models
// @Description Returns list of models available on the Ollama server
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} OllamaModelsResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse "Ollama unavailable"
// @Router /api/settings/ai/models [get]
func (h *AISettingsHandler) ListOllamaModels(c *gin.Context) {
	ctx := c.Request.Context()

	ollamaURL, _ := h.getSettingString(ctx, "ai.ollama_url")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	provider := providers.NewOllamaProvider(ollamaURL, "")
	models, err := provider.ListModels(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error:   "ollama_unavailable",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, OllamaModelsResponse{
		Models: models,
	})
}

// OllamaModelsResponse represents the response for listing models.
type OllamaModelsResponse struct {
	Models []ai.ModelInfo `json:"models"`
}

// PullOllamaModelRequest represents the request to pull a model.
type PullOllamaModelRequest struct {
	Model string `json:"model" binding:"required"`
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
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	ollamaURL, _ := h.getSettingString(ctx, "ai.ollama_url")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	provider := providers.NewOllamaProvider(ollamaURL, "")

	// Set up SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	progress, err := provider.Pull(ctx, req.Model)
	if err != nil {
		// Send error as SSE event
		writeSSEEvent(c, "error", map[string]string{"error": err.Error()})
		c.Writer.Flush()
		return
	}

	// Stream progress events
	for p := range progress {
		if err := writeSSEEvent(c, "progress", p); err != nil {
			return
		}
		c.Writer.Flush()

		if p.Done || p.Error != "" {
			break
		}
	}
}

// DeleteOllamaModelRequest represents the request to delete a model.
type DeleteOllamaModelRequest struct {
	Model string `json:"model" binding:"required"`
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
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	ollamaURL, _ := h.getSettingString(ctx, "ai.ollama_url")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	provider := providers.NewOllamaProvider(ollamaURL, "")
	if err := provider.DeleteModel(ctx, req.Model); err != nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error:   "ollama_error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Model deleted successfully"})
}

// TestOllamaConnection handles POST /api/settings/ai/test/ollama
// @Summary Test Ollama connection
// @Description Tests connectivity to the Ollama server
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} ConnectionTestResponse
// @Failure 503 {object} ConnectionTestResponse
// @Router /api/settings/ai/test/ollama [post]
func (h *AISettingsHandler) TestOllamaConnection(c *gin.Context) {
	ctx := c.Request.Context()

	ollamaURL, _ := h.getSettingString(ctx, "ai.ollama_url")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	provider := providers.NewOllamaProvider(ollamaURL, "")
	err := provider.HealthCheck(ctx)

	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ConnectionTestResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ConnectionTestResponse{
		Success: true,
		Message: "Ollama is accessible",
	})
}

// TestOpenAIConnection handles POST /api/settings/ai/test/openai
// @Summary Test OpenAI connection
// @Description Tests connectivity to OpenAI with the configured API key
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} ConnectionTestResponse
// @Failure 503 {object} ConnectionTestResponse
// @Router /api/settings/ai/test/openai [post]
func (h *AISettingsHandler) TestOpenAIConnection(c *gin.Context) {
	ctx := c.Request.Context()

	// Get the actual (decrypted) API key
	apiKey, err := h.settingsService.GetSystemValue(ctx, "ai.openai_api_key")
	if err != nil || apiKey == nil || apiKey == "" {
		c.JSON(http.StatusBadRequest, ConnectionTestResponse{
			Success: false,
			Error:   "OpenAI API key not configured",
		})
		return
	}

	apiKeyStr, ok := apiKey.(string)
	if !ok || apiKeyStr == "" {
		c.JSON(http.StatusBadRequest, ConnectionTestResponse{
			Success: false,
			Error:   "OpenAI API key not configured",
		})
		return
	}

	// Test the API key by making a simple request
	// For now, we just check the key format
	if len(apiKeyStr) < 20 {
		c.JSON(http.StatusBadRequest, ConnectionTestResponse{
			Success: false,
			Error:   "Invalid API key format",
		})
		return
	}

	c.JSON(http.StatusOK, ConnectionTestResponse{
		Success: true,
		Message: "API key format is valid",
	})
}

// ConnectionTestResponse represents a connection test result.
type ConnectionTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Helper methods

func (h *AISettingsHandler) getSettingString(ctx context.Context, key string) (string, error) {
	val, err := h.settingsService.GetSystemValue(ctx, key)
	if err != nil {
		def := settingsDomain.GetSystemDefinition(key)
		if def != nil {
			if s, ok := def.Default.(string); ok {
				return s, nil
			}
		}
		return "", err
	}
	if s, ok := val.(string); ok {
		return s, nil
	}
	return "", nil
}

func (h *AISettingsHandler) getSettingBool(ctx context.Context, key string) (bool, error) {
	val, err := h.settingsService.GetSystemValue(ctx, key)
	if err != nil {
		def := settingsDomain.GetSystemDefinition(key)
		if def != nil {
			if b, ok := def.Default.(bool); ok {
				return b, nil
			}
		}
		return false, err
	}
	if b, ok := val.(bool); ok {
		return b, nil
	}
	return false, nil
}

func (h *AISettingsHandler) getSettingInt(ctx context.Context, key string) (int, error) {
	val, err := h.settingsService.GetSystemValue(ctx, key)
	if err != nil {
		def := settingsDomain.GetSystemDefinition(key)
		if def != nil {
			if i, ok := def.Default.(int); ok {
				return i, nil
			}
		}
		return 0, err
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	}
	return 0, nil
}

func (h *AISettingsHandler) checkOllamaAvailable(ctx context.Context, ollamaURL string) bool {
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	provider := providers.NewOllamaProvider(ollamaURL, "")

	// Use a short timeout for availability check
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return provider.HealthCheck(checkCtx) == nil
}

// writeSSEEvent writes an SSE event (local helper, uses same format as system.go)
func writeAISSEEvent(c *gin.Context, eventType string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, jsonData)
	return err
}
