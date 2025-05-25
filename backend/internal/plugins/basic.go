// Package plugins provides basic plugin implementations and context helpers.
package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
)

// BasicPlugin provides a basic implementation of the Plugin interface
type BasicPlugin struct {
	info    *PluginInfo
	ctx     *PluginContext
	running bool
}

// NewBasicPlugin creates a new basic plugin instance
func NewBasicPlugin(info *PluginInfo) *BasicPlugin {
	return &BasicPlugin{
		info:    info,
		running: false,
	}
}

// Initialize implements Plugin.Initialize
func (p *BasicPlugin) Initialize(ctx *PluginContext) error {
	p.ctx = ctx
	ctx.Logger.Info("Plugin initialized", "plugin", p.info.ID)
	return nil
}

// Start implements Plugin.Start
func (p *BasicPlugin) Start(ctx context.Context) error {
	if p.running {
		return fmt.Errorf("plugin already running")
	}
	
	p.running = true
	p.ctx.Logger.Info("Plugin started", "plugin", p.info.ID)
	return nil
}

// Stop implements Plugin.Stop
func (p *BasicPlugin) Stop(ctx context.Context) error {
	if !p.running {
		return nil
	}
	
	p.running = false
	p.ctx.Logger.Info("Plugin stopped", "plugin", p.info.ID)
	return nil
}

// Info implements Plugin.Info
func (p *BasicPlugin) Info() *PluginInfo {
	return p.info
}

// Health implements Plugin.Health
func (p *BasicPlugin) Health() error {
	if !p.running {
		return fmt.Errorf("plugin not running")
	}
	return nil
}

// =============================================================================
// PLUGIN CONTEXT IMPLEMENTATIONS
// =============================================================================

// basicPluginLogger provides logging functionality for plugins
type basicPluginLogger struct {
	pluginID string
	logger   PluginLogger
}

func (l *basicPluginLogger) Debug(msg string, fields ...interface{}) {
	l.logger.Debug(fmt.Sprintf("[%s] %s", l.pluginID, msg), fields...)
}

func (l *basicPluginLogger) Info(msg string, fields ...interface{}) {
	l.logger.Info(fmt.Sprintf("[%s] %s", l.pluginID, msg), fields...)
}

func (l *basicPluginLogger) Warn(msg string, fields ...interface{}) {
	l.logger.Warn(fmt.Sprintf("[%s] %s", l.pluginID, msg), fields...)
}

func (l *basicPluginLogger) Error(msg string, fields ...interface{}) {
	l.logger.Error(fmt.Sprintf("[%s] %s", l.pluginID, msg), fields...)
}

// basicPluginConfig provides configuration management for plugins
type basicPluginConfig struct {
	pluginID string
	manager  *Manager
}

func (c *basicPluginConfig) Get(key string) interface{} {
	info, exists := c.manager.GetPluginInfo(c.pluginID)
	if !exists || info.Config == nil {
		return nil
	}
	return info.Config[key]
}

func (c *basicPluginConfig) Set(key string, value interface{}) error {
	info, exists := c.manager.GetPluginInfo(c.pluginID)
	if !exists {
		return fmt.Errorf("plugin not found")
	}
	
	if info.Config == nil {
		info.Config = make(map[string]interface{})
	}
	
	info.Config[key] = value
	
	// Update in database
	configData, _ := json.Marshal(info.Config)
	db := c.manager.getDB()
	return db.Model(&database.Plugin{}).
		Where("plugin_id = ?", c.pluginID).
		Update("config_data", string(configData)).Error
}

func (c *basicPluginConfig) GetString(key string) string {
	if value := c.Get(key); value != nil {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

func (c *basicPluginConfig) GetInt(key string) int {
	if value := c.Get(key); value != nil {
		if num, ok := value.(float64); ok {
			return int(num)
		}
		if num, ok := value.(int); ok {
			return num
		}
	}
	return 0
}

func (c *basicPluginConfig) GetBool(key string) bool {
	if value := c.Get(key); value != nil {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return false
}

// basicHTTPClient provides HTTP functionality for plugins
type basicHTTPClient struct {
	client *http.Client
}

func (h *basicHTTPClient) ensureClient() {
	if h.client == nil {
		h.client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
}

func (h *basicHTTPClient) Get(url string) ([]byte, error) {
	h.ensureClient()
	
	resp, err := h.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	return io.ReadAll(resp.Body)
}

func (h *basicHTTPClient) Post(url string, data []byte) ([]byte, error) {
	h.ensureClient()
	
	resp, err := h.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	return io.ReadAll(resp.Body)
}

func (h *basicHTTPClient) Put(url string, data []byte) ([]byte, error) {
	h.ensureClient()
	
	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	return io.ReadAll(resp.Body)
}

func (h *basicHTTPClient) Delete(url string) ([]byte, error) {
	h.ensureClient()
	
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	return io.ReadAll(resp.Body)
}

// basicFileSystemAccess provides safe file system access for plugins
type basicFileSystemAccess struct {
	basePath string
}

func (f *basicFileSystemAccess) safePath(path string) (string, error) {
	// Resolve path relative to plugin base path
	fullPath := filepath.Join(f.basePath, path)
	
	// Check that resolved path is still within base path (prevent directory traversal)
	if !strings.HasPrefix(fullPath, f.basePath) {
		return "", fmt.Errorf("path outside plugin directory: %s", path)
	}
	
	return fullPath, nil
}

func (f *basicFileSystemAccess) ReadFile(path string) ([]byte, error) {
	safePath, err := f.safePath(path)
	if err != nil {
		return nil, err
	}
	
	return os.ReadFile(safePath)
}

func (f *basicFileSystemAccess) WriteFile(path string, data []byte) error {
	safePath, err := f.safePath(path)
	if err != nil {
		return err
	}
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	return os.WriteFile(safePath, data, 0644)
}

func (f *basicFileSystemAccess) Exists(path string) bool {
	safePath, err := f.safePath(path)
	if err != nil {
		return false
	}
	
	_, err = os.Stat(safePath)
	return err == nil
}

func (f *basicFileSystemAccess) ListFiles(dir string) ([]string, error) {
	safePath, err := f.safePath(dir)
	if err != nil {
		return nil, err
	}
	
	entries, err := os.ReadDir(safePath)
	if err != nil {
		return nil, err
	}
	
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		files = append(files, entry.Name())
	}
	
	return files, nil
}

func (f *basicFileSystemAccess) CreateDir(path string) error {
	safePath, err := f.safePath(path)
	if err != nil {
		return err
	}
	
	return os.MkdirAll(safePath, 0755)
}

// basicEventBus provides event functionality for plugins
type basicEventBus struct {
	pluginID string
	manager  *Manager
}

func (e *basicEventBus) Publish(event string, data interface{}) error {
	e.manager.emitEvent(PluginEventData{
		PluginID:  e.pluginID,
		EventType: event,
		Message:   fmt.Sprintf("Event published: %s", event),
		Data:      data,
		Timestamp: time.Now(),
	})
	return nil
}

func (e *basicEventBus) Subscribe(event string, handler func(data interface{})) error {
	// For now, just log the subscription
	// In a full implementation, we would maintain event subscriptions
	e.manager.logger.Info("Plugin subscribed to event", "plugin", e.pluginID, "event", event)
	return nil
}

func (e *basicEventBus) Unsubscribe(event string, handler func(data interface{})) error {
	// For now, just log the unsubscription
	e.manager.logger.Info("Plugin unsubscribed from event", "plugin", e.pluginID, "event", event)
	return nil
}

// basicHookRegistry provides hook functionality for plugins
type basicHookRegistry struct {
	pluginID string
	manager  *Manager
}

func (h *basicHookRegistry) Register(hook string, handler func(data interface{}) interface{}) error {
	hookHandler := HookHandler{
		PluginID: h.pluginID,
		Handler:  handler,
		Priority: 100, // Default priority
	}
	
	h.manager.RegisterHook(hook, hookHandler)
	h.manager.logger.Info("Plugin registered hook", "plugin", h.pluginID, "hook", hook)
	return nil
}

func (h *basicHookRegistry) Execute(hook string, data interface{}) interface{} {
	return h.manager.ExecuteHook(hook, data)
}

func (h *basicHookRegistry) Remove(hook string, handler func(data interface{}) interface{}) error {
	// For now, just log the removal
	// In a full implementation, we would remove the specific handler
	h.manager.logger.Info("Plugin removed hook", "plugin", h.pluginID, "hook", hook)
	return nil
}

// =============================================================================
// SPECIFIC PLUGIN TYPE IMPLEMENTATIONS
// =============================================================================

// ExampleMetadataScraperPlugin demonstrates how to implement a metadata scraper plugin
type ExampleMetadataScraperPlugin struct {
	*BasicPlugin
}

// NewExampleMetadataScraperPlugin creates a new example metadata scraper plugin
func NewExampleMetadataScraperPlugin(info *PluginInfo) *ExampleMetadataScraperPlugin {
	return &ExampleMetadataScraperPlugin{
		BasicPlugin: NewBasicPlugin(info),
	}
}

// CanHandle implements MetadataScraperPlugin.CanHandle
func (p *ExampleMetadataScraperPlugin) CanHandle(filePath string, mimeType string) bool {
	// Example: handle MP3 files
	return strings.HasSuffix(strings.ToLower(filePath), ".mp3")
}

// ExtractMetadata implements MetadataScraperPlugin.ExtractMetadata
func (p *ExampleMetadataScraperPlugin) ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	// Example metadata extraction
	metadata := map[string]interface{}{
		"extracted_by": p.info.ID,
		"file_path":    filePath,
		"extracted_at": time.Now(),
	}
	
	p.ctx.Logger.Info("Metadata extracted", "file", filePath)
	return metadata, nil
}

// SupportedTypes implements MetadataScraperPlugin.SupportedTypes
func (p *ExampleMetadataScraperPlugin) SupportedTypes() []string {
	return []string{"audio/mpeg", ".mp3"}
}

// ExampleAdminPagePlugin demonstrates how to implement an admin page plugin
type ExampleAdminPagePlugin struct {
	*BasicPlugin
}

// NewExampleAdminPagePlugin creates a new example admin page plugin
func NewExampleAdminPagePlugin(info *PluginInfo) *ExampleAdminPagePlugin {
	return &ExampleAdminPagePlugin{
		BasicPlugin: NewBasicPlugin(info),
	}
}

// RegisterRoutes implements AdminPagePlugin.RegisterRoutes
func (p *ExampleAdminPagePlugin) RegisterRoutes(router *gin.RouterGroup) error {
	// Example: register a simple admin endpoint
	pluginGroup := router.Group("/plugins/" + p.info.ID)
	{
		pluginGroup.GET("/status", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"plugin": p.info.ID,
				"status": "running",
				"health": p.Health() == nil,
			})
		})
		
		pluginGroup.GET("/config", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"config": p.info.Config,
			})
		})
	}
	
	p.ctx.Logger.Info("Admin routes registered", "plugin", p.info.ID)
	return nil
}

// GetAdminPages implements AdminPagePlugin.GetAdminPages
func (p *ExampleAdminPagePlugin) GetAdminPages() []AdminPageConfig {
	return []AdminPageConfig{
		{
			ID:       p.info.ID + "_dashboard",
			Title:    p.info.Name + " Dashboard",
			Path:     "/admin/plugins/" + p.info.ID,
			Icon:     "plugin",
			Category: "Plugins",
			URL:      "/api/admin/plugins/" + p.info.ID + "/dashboard",
			Type:     "iframe",
		},
	}
}
