package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/middleware"
	"github.com/mantonx/viewra/internal/application/settings"
	settingsDomain "github.com/mantonx/viewra/internal/domain/settings"
)

// SettingsHandler handles settings HTTP requests.
type SettingsHandler struct {
	service *settings.Service
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(service *settings.Service) *SettingsHandler {
	return &SettingsHandler{service: service}
}

// GetAllSystem handles GET /api/settings/system
// @Summary List system settings
// @Description Returns all system settings (admin only)
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} SystemSettingsResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/settings/system [get]
func (h *SettingsHandler) GetAllSystem(c *gin.Context) {
	allSettings, err := h.service.GetAllSystem(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	// Convert to response format
	response := make([]SystemSettingResponse, len(allSettings))
	for i, s := range allSettings {
		response[i] = SystemSettingResponse{
			Key:         s.Key,
			Value:       s.Value,
			ValueType:   string(s.ValueType),
			Category:    string(s.Category),
			Description: s.Description,
			UpdatedAt:   s.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedBy:   s.UpdatedBy,
		}
	}

	c.JSON(http.StatusOK, SystemSettingsResponse{Settings: response})
}

// GetSystem handles GET /api/settings/system/:key
// @Summary Get system setting
// @Description Returns a system setting by key (admin only)
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Param key path string true "Setting key"
// @Success 200 {object} SystemSettingValueResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/settings/system/{key} [get]
func (h *SettingsHandler) GetSystem(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Setting key required",
		})
		return
	}

	value, err := h.service.GetSystemValue(c.Request.Context(), key)
	if err != nil {
		handleSettingsError(c, err)
		return
	}

	c.JSON(http.StatusOK, SystemSettingValueResponse{
		Key:   key,
		Value: value,
	})
}

// SetSystemRequest is the request body for updating a system setting.
type SetSystemRequest struct {
	Value any `json:"value" binding:"required"`
}

// SetSystem handles PUT /api/settings/system/:key
// @Summary Update system setting
// @Description Updates a system setting (admin only)
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param key path string true "Setting key"
// @Param setting body SetSystemRequest true "Setting value"
// @Success 200 {object} SystemSettingValueResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/settings/system/{key} [put]
func (h *SettingsHandler) SetSystem(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Setting key required",
		})
		return
	}

	var req SetSystemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Get the user who made the update (use Subject which is the public ID)
	claims := middleware.GetClaims(c)
	updatedBy := ""
	if claims != nil {
		updatedBy = claims.Subject
	}

	if err := h.service.SetSystem(c.Request.Context(), key, req.Value, updatedBy); err != nil {
		handleSettingsError(c, err)
		return
	}

	c.JSON(http.StatusOK, SystemSettingValueResponse{
		Key:   key,
		Value: req.Value,
	})
}

// GetAllUser handles GET /api/settings/user
// @Summary List user settings
// @Description Returns all settings for the current user
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} UserSettingsResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/settings/user [get]
func (h *SettingsHandler) GetAllUser(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	allSettings, err := h.service.GetAllUser(c.Request.Context(), claims.Subject)
	if err != nil {
		handleError(c, err)
		return
	}

	response := make([]UserSettingResponse, len(allSettings))
	for i, s := range allSettings {
		response[i] = UserSettingResponse{
			Key:       s.Key,
			Value:     s.Value,
			UpdatedAt: s.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	c.JSON(http.StatusOK, UserSettingsResponse{Settings: response})
}

// GetUser handles GET /api/settings/user/:key
// @Summary Get user setting
// @Description Returns a user setting by key for the current user
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Param key path string true "Setting key"
// @Success 200 {object} UserSettingValueResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/settings/user/{key} [get]
func (h *SettingsHandler) GetUser(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Setting key required",
		})
		return
	}

	value, err := h.service.GetUserValue(c.Request.Context(), claims.Subject, key)
	if err != nil {
		handleSettingsError(c, err)
		return
	}

	c.JSON(http.StatusOK, UserSettingValueResponse{
		Key:   key,
		Value: value,
	})
}

// SetUserRequest is the request body for updating a user setting.
type SetUserRequest struct {
	Value any `json:"value" binding:"required"`
}

// SetUser handles PUT /api/settings/user/:key
// @Summary Update user setting
// @Description Updates a user setting for the current user
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param key path string true "Setting key"
// @Param setting body SetUserRequest true "Setting value"
// @Success 200 {object} UserSettingValueResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/settings/user/{key} [put]
func (h *SettingsHandler) SetUser(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Setting key required",
		})
		return
	}

	var req SetUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	if err := h.service.SetUser(c.Request.Context(), claims.Subject, key, req.Value); err != nil {
		handleSettingsError(c, err)
		return
	}

	c.JSON(http.StatusOK, UserSettingValueResponse{
		Key:   key,
		Value: req.Value,
	})
}

// DeleteUser handles DELETE /api/settings/user/:key
// @Summary Delete user setting
// @Description Resets a user setting to default for the current user
// @Tags settings
// @Security BearerAuth
// @Param key path string true "Setting key"
// @Success 204 "No Content"
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/settings/user/{key} [delete]
func (h *SettingsHandler) DeleteUser(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Authentication required",
		})
		return
	}

	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Setting key required",
		})
		return
	}

	if err := h.service.DeleteUser(c.Request.Context(), claims.Subject, key); err != nil {
		handleSettingsError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetAllSystemEffective handles GET /api/settings/system/effective
// @Summary List effective system settings
// @Description Returns all system settings with their effective values and sources (admin only)
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} EffectiveSettingsResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/settings/system/effective [get]
func (h *SettingsHandler) GetAllSystemEffective(c *gin.Context) {
	effectiveSettings, err := h.service.GetAllEffectiveSystemValues(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	// Get definitions to include labels and descriptions
	defs := h.service.GetSystemDefinitions()
	defMap := make(map[string]settingsDomain.Definition)
	for _, d := range defs {
		defMap[d.Key] = d
	}

	response := make([]EffectiveSettingResponse, 0, len(effectiveSettings))
	for key, eff := range effectiveSettings {
		def := defMap[key]
		response = append(response, EffectiveSettingResponse{
			Key:         key,
			Value:       eff.Value,
			Source:      string(eff.Source),
			EnvVar:      eff.EnvVar,
			Locked:      eff.Locked,
			ReadOnly:    eff.ReadOnly,
			Label:       def.Label,
			Description: def.Description,
			Category:    string(def.Category),
			Type:        string(def.Type),
		})
	}

	c.JSON(http.StatusOK, EffectiveSettingsResponse{Settings: response})
}

// GetSystemInfo handles GET /api/system/info
// @Summary Get system information
// @Description Returns detected system hardware information
// @Tags system
// @Security BearerAuth
// @Produce json
// @Success 200 {object} SystemInfoResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/system/info [get]
func (h *SettingsHandler) GetSystemInfo(c *gin.Context) {
	profile := h.service.GetSystemProfile()
	if profile == nil {
		c.JSON(http.StatusOK, SystemInfoResponse{
			CPU: CPUInfoResponse{
				Model:        "Unknown",
				PhysicalCPUs: 0,
				LogicalCPUs:  0,
			},
			Memory: MemoryInfoResponse{
				TotalBytes:     0,
				TotalFormatted: "Unknown",
			},
			GPU: GPUInfoResponse{
				Available:   false,
				Type:        "none",
				DeviceNames: []string{},
			},
		})
		return
	}

	// Format memory size
	memFormatted := formatMemory(profile.Memory.TotalBytes)

	response := SystemInfoResponse{
		CPU: CPUInfoResponse{
			Model:        profile.CPU.Model,
			PhysicalCPUs: profile.CPU.NumPhysical,
			LogicalCPUs:  profile.CPU.NumCPU,
		},
		Memory: MemoryInfoResponse{
			TotalBytes:     profile.Memory.TotalBytes,
			TotalFormatted: memFormatted,
			AvailableBytes: profile.Memory.AvailableBytes,
		},
		GPU: GPUInfoResponse{
			Available:   profile.GPU.Available,
			Type:        string(profile.GPU.Type),
			DeviceCount: profile.GPU.DeviceCount,
			DeviceNames: profile.GPU.DeviceNames,
			HWAccelType: profile.GetHardwareAccelType(),
			HasVAAPI:    profile.GPU.HasVAAPI,
			HasOpenCL:   profile.GPU.HasOpenCL,
			HasVulkan:   profile.GPU.HasVulkan,
		},
	}

	// Include storage if available
	if profile.Storage.Type != "" {
		response.Storage = StorageInfoResponse{
			Type:         string(profile.Storage.Type),
			IsRemote:     profile.Storage.IsRemote,
			DetectedPath: profile.Storage.DetectedPath,
		}
	}

	c.JSON(http.StatusOK, response)
}

// formatMemory formats bytes into human-readable format
func formatMemory(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return formatFloat(float64(bytes)/float64(TB)) + " TB"
	case bytes >= GB:
		return formatFloat(float64(bytes)/float64(GB)) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/float64(MB)) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/float64(KB)) + " KB"
	default:
		return formatFloat(float64(bytes)) + " B"
	}
}

// formatFloat formats a float to 1 decimal place, removing trailing zeros
func formatFloat(f float64) string {
	s := fmt.Sprintf("%.1f", f)
	// Remove trailing .0
	if len(s) > 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}

// GetSchema handles GET /api/settings/schema
// @Summary Get settings schema
// @Description Returns the schema for all settings (for UI generation)
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} SettingsSchemaResponse
// @Failure 401 {object} ErrorResponse
// @Router /api/settings/schema [get]
func (h *SettingsHandler) GetSchema(c *gin.Context) {
	systemDefs := h.service.GetSystemDefinitions()
	userDefs := h.service.GetUserDefinitions()

	systemSchema := make([]SettingDefinitionResponse, len(systemDefs))
	for i, d := range systemDefs {
		systemSchema[i] = definitionToResponse(d)
	}

	userSchema := make([]SettingDefinitionResponse, len(userDefs))
	for i, d := range userDefs {
		userSchema[i] = definitionToResponse(d)
	}

	c.JSON(http.StatusOK, SettingsSchemaResponse{
		System: systemSchema,
		User:   userSchema,
	})
}

// Response types

// SystemSettingResponse represents a system setting in API responses.
type SystemSettingResponse struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueType   string `json:"value_type"`
	Category    string `json:"category"`
	Description string `json:"description,omitempty"`
	UpdatedAt   string `json:"updated_at"`
	UpdatedBy   string `json:"updated_by,omitempty"`
}

// SystemSettingsResponse is the response for listing system settings.
type SystemSettingsResponse struct {
	Settings []SystemSettingResponse `json:"settings"`
}

// SystemSettingValueResponse is the response for a single setting value.
type SystemSettingValueResponse struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// UserSettingResponse represents a user setting in API responses.
type UserSettingResponse struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}

// UserSettingsResponse is the response for listing user settings.
type UserSettingsResponse struct {
	Settings []UserSettingResponse `json:"settings"`
}

// UserSettingValueResponse is the response for a single user setting value.
type UserSettingValueResponse struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// SettingDefinitionResponse represents a setting definition for the UI.
type SettingDefinitionResponse struct {
	Key         string                     `json:"key"`
	Type        string                     `json:"type"`
	Category    string                     `json:"category"`
	Label       string                     `json:"label"`
	Description string                     `json:"description"`
	Default     any                        `json:"default"`
	Options     []SettingOptionResponse    `json:"options,omitempty"`
	Validation  *SettingValidationResponse `json:"validation,omitempty"`
	AdminOnly   bool                       `json:"adminOnly"`
	Restartable bool                       `json:"restartable"`
	Sensitive   bool                       `json:"sensitive"`        // True if value is encrypted (API keys)
	EnvVar      string                     `json:"envVar,omitempty"` // Associated env var name
	ReadOnly    bool                       `json:"readOnly"`         // True for display-only values
}

// SettingOptionResponse represents a select option.
type SettingOptionResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// SettingValidationResponse represents validation rules.
type SettingValidationResponse struct {
	Min      *int   `json:"min,omitempty"`
	Max      *int   `json:"max,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	Required bool   `json:"required,omitempty"`
}

// SettingsSchemaResponse is the response for the settings schema.
type SettingsSchemaResponse struct {
	System []SettingDefinitionResponse `json:"system"`
	User   []SettingDefinitionResponse `json:"user"`
}

// EffectiveSettingResponse represents a setting with its resolved value and source.
type EffectiveSettingResponse struct {
	Key         string `json:"key"`
	Value       any    `json:"value"`
	Source      string `json:"source"`                // default, database, env_var, detected
	EnvVar      string `json:"envVar,omitempty"`      // Env var name if source is env_var
	Locked      bool   `json:"locked"`                // True if cannot be changed via UI
	ReadOnly    bool   `json:"readOnly"`              // True for display-only values
	Label       string `json:"label"`                 // Human-readable label
	Description string `json:"description,omitempty"` // Setting description
	Category    string `json:"category"`              // Setting category
	Type        string `json:"type"`                  // Value type
}

// EffectiveSettingsResponse is the response for all effective system settings.
type EffectiveSettingsResponse struct {
	Settings []EffectiveSettingResponse `json:"settings"`
}

// SystemInfoResponse represents system information (read-only display).
type SystemInfoResponse struct {
	CPU     CPUInfoResponse     `json:"cpu"`
	Memory  MemoryInfoResponse  `json:"memory"`
	GPU     GPUInfoResponse     `json:"gpu"`
	Storage StorageInfoResponse `json:"storage,omitempty"`
}

// CPUInfoResponse represents CPU information.
type CPUInfoResponse struct {
	Model        string `json:"model"`
	PhysicalCPUs int    `json:"physicalCpus"`
	LogicalCPUs  int    `json:"logicalCpus"`
}

// MemoryInfoResponse represents memory information.
type MemoryInfoResponse struct {
	TotalBytes     uint64 `json:"totalBytes"`
	TotalFormatted string `json:"totalFormatted"`
	AvailableBytes uint64 `json:"availableBytes,omitempty"`
}

// GPUInfoResponse represents GPU information.
type GPUInfoResponse struct {
	Available   bool     `json:"available"`
	Type        string   `json:"type"`
	DeviceCount int      `json:"deviceCount"`
	DeviceNames []string `json:"deviceNames"`
	HWAccelType string   `json:"hwAccelType"`
	HasVAAPI    bool     `json:"hasVaapi"`
	HasOpenCL   bool     `json:"hasOpenCL"`
	HasVulkan   bool     `json:"hasVulkan"`
}

// StorageInfoResponse represents storage information.
type StorageInfoResponse struct {
	Type         string `json:"type"`
	IsRemote     bool   `json:"isRemote"`
	DetectedPath string `json:"detectedPath,omitempty"`
}

// Helper functions

func definitionToResponse(d settingsDomain.Definition) SettingDefinitionResponse {
	resp := SettingDefinitionResponse{
		Key:         d.Key,
		Type:        string(d.Type),
		Category:    string(d.Category),
		Label:       d.Label,
		Description: d.Description,
		Default:     d.Default,
		AdminOnly:   d.AdminOnly,
		Restartable: d.Restartable,
		Sensitive:   d.Sensitive,
		EnvVar:      d.EnvVar,
		ReadOnly:    d.ReadOnly,
	}

	if len(d.Options) > 0 {
		resp.Options = make([]SettingOptionResponse, len(d.Options))
		for i, o := range d.Options {
			resp.Options[i] = SettingOptionResponse{
				Value: o.Value,
				Label: o.Label,
			}
		}
	}

	if d.Validation != nil {
		resp.Validation = &SettingValidationResponse{
			Min:      d.Validation.Min,
			Max:      d.Validation.Max,
			Pattern:  d.Validation.Pattern,
			Required: d.Validation.Required,
		}
	}

	return resp
}

func handleSettingsError(c *gin.Context, err error) {
	switch err {
	case settingsDomain.ErrSettingNotFound:
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Setting not found",
		})
	case settingsDomain.ErrUnknownSetting:
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Unknown setting key",
		})
	case settingsDomain.ErrInvalidValue:
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid setting value",
		})
	default:
		handleError(c, err)
	}
}
