// Package proxy provides HTTP proxying for plugin routes.
// It handles routing HTTP requests to plugin gRPC handlers, including
// capability-based routing, rate limiting, and streaming support.
package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/registry"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
)

// PluginLookup provides access to plugin instances.
// This interface allows the proxy to work without depending on the manager package directly.
type PluginLookup interface {
	GetPlugin(id string) (*types.Instance, bool)
	RestartPlugin(ctx context.Context, pluginID string) error
}

// HTTPProxy handles proxying HTTP requests to plugin gRPC handlers.
type HTTPProxy struct {
	pluginLookup       PluginLookup
	routeRegistry      *registry.RouteRegistry
	capabilityRegistry *registry.CapabilityRegistry
	rateLimiter        *registry.RouteRateLimiter
	logger             *slog.Logger
}

// NewHTTPProxy creates a new HTTP proxy for plugin routes.
func NewHTTPProxy(
	pluginLookup PluginLookup,
	routeRegistry *registry.RouteRegistry,
	capabilityRegistry *registry.CapabilityRegistry,
	rateLimiter *registry.RouteRateLimiter,
	logger *slog.Logger,
) *HTTPProxy {
	return &HTTPProxy{
		pluginLookup:       pluginLookup,
		routeRegistry:      routeRegistry,
		capabilityRegistry: capabilityRegistry,
		rateLimiter:        rateLimiter,
		logger:             logger,
	}
}

// HandlePluginRoute handles requests to /api/plugins/:plugin_id/*path.
// It routes the request to the appropriate plugin's HandleHTTP or HandleHTTPStream.
func (p *HTTPProxy) HandlePluginRoute(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	// The path parameter captures everything after the plugin ID
	path := c.Param("path")
	if path == "" {
		path = "/"
	}

	p.handleRequest(c, pluginID, path)
}

// HandleCapabilityRoute returns a handler for capability-aliased routes.
// For example, /api/search is aliased to the semantic_search capability.
func (p *HTTPProxy) HandleCapabilityRoute(capability string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Find which plugin provides this capability
		mapping := p.capabilityRegistry.Resolve(capability)
		if mapping == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error":      "capability not available",
				"capability": capability,
			})
			return
		}

		// Route to the plugin's path
		p.handleRequest(c, mapping.PluginID, mapping.PluginPath)
	}
}

// handleRequest does the actual work of proxying an HTTP request to a plugin.
func (p *HTTPProxy) handleRequest(c *gin.Context, pluginID, path string) {
	// Get the plugin instance
	instance, ok := p.pluginLookup.GetPlugin(pluginID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     "plugin not found",
			"plugin_id": pluginID,
		})
		return
	}

	// Find the matching route
	route, pathParams, ok := p.routeRegistry.FindRoute(pluginID, path, c.Request.Method)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     "route not found",
			"plugin_id": pluginID,
			"path":      path,
			"method":    c.Request.Method,
		})
		return
	}

	// Check admin_only routes
	if route.AdminOnly {
		isAdmin, exists := c.Get("is_admin")
		if !exists || !isAdmin.(bool) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "admin access required",
			})
			return
		}
	}

	// Apply rate limiting
	if route.RateLimit != nil && route.RateLimit.RequestsPerMinute > 0 {
		userID := getUserIDFromContext(c)
		if !p.rateLimiter.CheckRateLimit(route, userID) {
			c.Header("X-RateLimit-Limit", formatRateLimit(route.RateLimit.RequestsPerMinute))
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": 60,
			})
			return
		}
	}

	// Build the request
	req, err := p.buildRequest(c, path, pathParams)
	if err != nil {
		p.logger.Error("failed to build plugin request",
			"plugin", pluginID,
			"path", path,
			"error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to build request",
		})
		return
	}

	// Dispatch to streaming or non-streaming handler
	if route.Streaming {
		p.handleStreamingRequest(c, instance, req)
	} else {
		p.handleSimpleRequest(c, instance, req)
	}
}

// buildRequest creates a PluginHTTPRequest from a gin.Context.
func (p *HTTPProxy) buildRequest(c *gin.Context, path string, pathParams map[string]string) (*pluginv1.PluginHTTPRequest, error) {
	// Read request body
	var body []byte
	if c.Request.Body != nil {
		var err error
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			return nil, err
		}
	}

	// Extract headers (lowercase)
	headers := make(map[string]string)
	for k, v := range c.Request.Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}

	// Extract query parameters
	query := make(map[string]string)
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}

	// Get user info from context
	userID := getUserIDFromContext(c)
	isAdmin := false
	if v, exists := c.Get("is_admin"); exists {
		isAdmin = v.(bool)
	}

	return &pluginv1.PluginHTTPRequest{
		Path:       path,
		Method:     c.Request.Method,
		Headers:    headers,
		Query:      query,
		PathParams: pathParams,
		Body:       body,
		UserId:     userID,
		IsAdmin:    isAdmin,
	}, nil
}

// handleSimpleRequest handles non-streaming requests.
func (p *HTTPProxy) handleSimpleRequest(c *gin.Context, instance *types.Instance, req *pluginv1.PluginHTTPRequest) {
	ctx := c.Request.Context()

	p.logger.Info("forwarding request to plugin",
		"plugin", instance.ID,
		"path", req.Path,
		"method", req.Method,
	)

	resp, err := instance.CoreClient.HandleHTTP(ctx, req)
	if err != nil {
		// Check if this is a connection error (plugin died/restarted)
		if isConnectionError(err) {
			p.logger.Warn("plugin connection failed, attempting restart",
				"plugin", instance.ID,
				"error", err)

			// Try to restart the plugin
			if restartErr := p.pluginLookup.RestartPlugin(ctx, instance.ID); restartErr != nil {
				p.logger.Error("failed to restart plugin",
					"plugin", instance.ID,
					"error", restartErr)
			} else {
				// Get refreshed instance and retry once
				if newInstance, ok := p.pluginLookup.GetPlugin(instance.ID); ok {
					resp, err = newInstance.CoreClient.HandleHTTP(ctx, req)
					if err == nil {
						// Success after restart - continue to response handling
						goto handleResponse
					}
				}
			}
		}

		p.logger.Error("plugin HandleHTTP failed",
			"plugin", instance.ID,
			"path", req.Path,
			"error", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error":  "plugin request failed",
			"detail": err.Error(),
		})
		return
	}

handleResponse:

	p.logger.Info("plugin returned response",
		"plugin", instance.ID,
		"path", req.Path,
		"status", resp.StatusCode,
	)

	// Write response headers
	for k, v := range resp.Headers {
		c.Header(k, v)
	}

	// Set content type
	contentType := resp.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	// Write response
	c.Data(int(resp.StatusCode), contentType, resp.Body)
}

// handleStreamingRequest handles streaming requests using HandleHTTPStream.
func (p *HTTPProxy) handleStreamingRequest(c *gin.Context, instance *types.Instance, req *pluginv1.PluginHTTPRequest) {
	ctx := c.Request.Context()

	// Open streaming RPC
	stream, err := instance.CoreClient.HandleHTTPStream(ctx)
	if err != nil {
		p.logger.Error("failed to open plugin stream",
			"plugin", instance.ID,
			"path", req.Path,
			"error", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error":  "failed to open stream",
			"detail": err.Error(),
		})
		return
	}

	// Send REQUEST_START with metadata (body is separate for streaming)
	startReq := &pluginv1.PluginHTTPRequest{
		Path:       req.Path,
		Method:     req.Method,
		Headers:    req.Headers,
		Query:      req.Query,
		PathParams: req.PathParams,
		UserId:     req.UserId,
		IsAdmin:    req.IsAdmin,
		// Body is sent separately in chunks
	}

	err = stream.Send(&pluginv1.PluginHTTPChunk{
		Type:    pluginv1.PluginHTTPChunk_REQUEST_START,
		Request: startReq,
	})
	if err != nil {
		p.logger.Error("failed to send request start",
			"plugin", instance.ID,
			"error", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "failed to send request",
		})
		return
	}

	// If there's a body, send it in chunks
	if len(req.Body) > 0 {
		// For simplicity, send the whole body as one chunk
		// In production, you might want to stream large bodies
		err = stream.Send(&pluginv1.PluginHTTPChunk{
			Type: pluginv1.PluginHTTPChunk_REQUEST_BODY,
			Data: req.Body,
		})
		if err != nil {
			p.logger.Error("failed to send request body",
				"plugin", instance.ID,
				"error", err)
			c.JSON(http.StatusBadGateway, gin.H{
				"error": "failed to send request body",
			})
			return
		}
	}

	// Signal end of request
	err = stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_REQUEST_END,
	})
	if err != nil {
		p.logger.Error("failed to send request end",
			"plugin", instance.ID,
			"error", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "failed to complete request",
		})
		return
	}

	// Close the send side
	if err := stream.CloseSend(); err != nil {
		p.logger.Error("failed to close send",
			"plugin", instance.ID,
			"error", err)
	}

	// Receive response chunks
	p.receiveStreamingResponse(c, stream, instance.ID)
}

// receiveStreamingResponse reads the response from a streaming RPC and writes it to gin.
func (p *HTTPProxy) receiveStreamingResponse(c *gin.Context, stream pluginv1.PluginCore_HandleHTTPStreamClient, pluginID string) {
	headersSent := false
	writer := c.Writer

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if !headersSent {
				c.JSON(http.StatusBadGateway, gin.H{
					"error":  "stream receive failed",
					"detail": err.Error(),
				})
			}
			return
		}

		switch chunk.Type {
		case pluginv1.PluginHTTPChunk_RESPONSE_START:
			// Set headers and status
			for k, v := range chunk.Headers {
				c.Header(k, v)
			}
			if chunk.ContentType != "" {
				c.Header("Content-Type", chunk.ContentType)
			}
			c.Status(int(chunk.StatusCode))
			headersSent = true

		case pluginv1.PluginHTTPChunk_RESPONSE_BODY:
			if !headersSent {
				// Default to 200 if no RESPONSE_START was sent
				c.Status(http.StatusOK)
				headersSent = true
			}
			if len(chunk.Data) > 0 {
				_, _ = writer.Write(chunk.Data)
				writer.Flush()
			}

		case pluginv1.PluginHTTPChunk_RESPONSE_END:
			// Stream complete
			if chunk.Error != "" {
				p.logger.Error("plugin stream ended with error",
					"plugin", pluginID,
					"error", chunk.Error)
			}
			return
		}
	}
}

// Stop cleans up resources.
func (p *HTTPProxy) Stop() {
	if p.rateLimiter != nil {
		p.rateLimiter.Stop()
	}
}

// RegisterCapabilityRoutes registers all capability alias routes on the router.
// Call this after loading plugins to set up stable URLs like /api/search.
// Routes are registered dynamically based on plugins that declare alias_path in their routes.
// The router should already have authentication middleware applied.
func (p *HTTPProxy) RegisterCapabilityRoutes(router *gin.RouterGroup) {
	for _, route := range p.routeRegistry.GetAllRoutes() {
		if route.AliasPath == "" {
			continue
		}

		// Strip /api prefix since router is already under /api
		path := strings.TrimPrefix(route.AliasPath, "/api")
		if path == "" {
			path = "/"
		}

		// Create a closure to capture the route
		pluginID := route.PluginID
		pluginPath := route.Path
		handler := func(c *gin.Context) {
			// Proxy to the plugin's actual route
			p.handleRequest(c, pluginID, pluginPath)
		}

		// Register for all HTTP methods
		router.Any(path, handler)
		// Also register with wildcard for sub-paths
		router.Any(path+"/*subpath", handler)

		p.logger.Debug("registered capability alias",
			"alias_path", route.AliasPath,
			"plugin_id", pluginID,
			"plugin_path", pluginPath)
	}
}

// InitFromPlugins initializes route and capability registries from loaded plugins.
// Call this after all plugins are loaded.
func (p *HTTPProxy) InitFromPlugins(ctx context.Context, plugins []*types.Instance) error {
	for _, plugin := range plugins {
		if err := p.registerPluginRoutes(ctx, plugin); err != nil {
			p.logger.Error("failed to register routes for plugin",
				"plugin", plugin.ID,
				"error", err)
			// Continue with other plugins
		}
	}

	return nil
}

// registerPluginRoutes fetches and registers routes for a single plugin.
func (p *HTTPProxy) registerPluginRoutes(ctx context.Context, plugin *types.Instance) error {
	if plugin.CoreClient == nil {
		return nil
	}

	// Get routes from plugin
	routes, err := plugin.CoreClient.GetRoutes(ctx, &pluginv1.Empty{})
	if err != nil {
		return err
	}

	if routes == nil || len(routes.Routes) == 0 {
		p.logger.Debug("plugin has no routes", "plugin", plugin.ID)
		return nil
	}

	// Register routes
	p.routeRegistry.RegisterRoutes(plugin.ID, routes.Routes)

	p.logger.Debug("registered plugin routes",
		"plugin", plugin.ID,
		"count", len(routes.Routes))

	// Register capabilities
	for _, route := range routes.Routes {
		if route.Capability != "" {
			if p.capabilityRegistry.Register(plugin.ID, route.Capability, route.Path) {
				p.logger.Debug("registered capability",
					"plugin", plugin.ID,
					"capability", route.Capability,
					"path", route.Path)
			} else {
				p.logger.Warn("capability already registered by another plugin",
					"capability", route.Capability,
					"plugin", plugin.ID)
			}
		}
	}

	return nil
}

// UnregisterPlugin removes routes and capabilities for a plugin.
func (p *HTTPProxy) UnregisterPlugin(pluginID string) {
	p.routeRegistry.UnregisterRoutes(pluginID)
	p.capabilityRegistry.Unregister(pluginID)
}

// GetRouteRegistry returns the route registry.
func (p *HTTPProxy) GetRouteRegistry() *registry.RouteRegistry {
	return p.routeRegistry
}

// GetCapabilityRegistry returns the capability registry.
func (p *HTTPProxy) GetCapabilityRegistry() *registry.CapabilityRegistry {
	return p.capabilityRegistry
}

// Helper functions

// getUserIDFromContext extracts the user ID from the gin context.
func getUserIDFromContext(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		switch v := userID.(type) {
		case string:
			return v
		case int64:
			return strconv.FormatInt(v, 10)
		case int:
			return strconv.Itoa(v)
		}
	}
	return ""
}

// formatRateLimit formats the rate limit for the header.
func formatRateLimit(rpm int32) string {
	return strconv.Itoa(int(rpm))
}

// isConnectionError checks if the error indicates the plugin connection is broken.
// This happens when the plugin process dies or is restarted.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for gRPC unavailable status (server not reachable)
	if s, ok := status.FromError(err); ok {
		switch s.Code() {
		case codes.Unavailable, codes.Internal:
			return true
		}
	}

	// Check for connection refused
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	// Check error message for common connection errors
	errStr := err.Error()
	connectionErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"EOF",
		"transport is closing",
		"no connection",
	}
	for _, ce := range connectionErrors {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(ce)) {
			return true
		}
	}

	return false
}
