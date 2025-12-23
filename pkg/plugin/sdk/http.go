// HTTP helpers for ViewRA plugin endpoints.
//
// This package provides types and utilities for handling HTTP requests
// in plugins that expose custom routes.
//
// # Quick Start
//
// Implement the HTTPProvider interface to expose routes:
//
//	func (p *MyPlugin) GetRoutes() []sdk.Route {
//	    return []sdk.Route{
//	        {Path: "/health", Methods: []string{"GET"}},
//	        {Path: "/items", Methods: []string{"GET", "POST"}},
//	        {Path: "/items/:id", Methods: []string{"DELETE"}},
//	    }
//	}
//
//	func (p *MyPlugin) HandleHTTP(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
//	    switch req.Path {
//	    case "/health":
//	        return sdk.JSONResponse(200, map[string]bool{"healthy": true})
//	    case "/items":
//	        if req.Method == "GET" {
//	            return p.listItems(ctx, req)
//	        }
//	        return p.createItem(ctx, req)
//	    default:
//	        return sdk.JSONError(404, "not found")
//	    }
//	}
//
// # Response Helpers
//
// Use the response helpers for common patterns:
//
//	// JSON response with any data
//	sdk.JSONResponse(200, map[string]any{"items": items})
//
//	// Error response
//	sdk.JSONError(400, "invalid request")
//
//	// Empty success
//	sdk.EmptyResponse(204)
//
// # SSE Streaming
//
// For streaming responses (like download progress):
//
//	func (p *MyPlugin) HandleHTTPStream(ctx context.Context, req *sdk.HTTPRequest, w sdk.HTTPStreamWriter) error {
//	    w.WriteHeader(200, "text/event-stream", nil)
//
//	    for progress := range downloadProgress {
//	        data, _ := json.Marshal(progress)
//	        w.WriteSSE("progress", data)
//	    }
//
//	    return nil
//	}
package sdk

import (
	"encoding/json"
	"fmt"
)

// Route describes an HTTP route exposed by a plugin.
type Route struct {
	// Path is relative to the plugin namespace, e.g., "/search" or "/items/:id"
	// Supports :param syntax for path parameters
	Path string

	// Methods lists HTTP methods this route handles: GET, POST, PUT, DELETE, PATCH
	Methods []string

	// AdminOnly restricts this route to admin users
	AdminOnly bool

	// Description is human-readable text for API docs
	Description string

	// Capability is a well-known capability for stable URL aliasing
	// e.g., "semantic_search" creates /api/search alias
	Capability string

	// Streaming indicates this route uses HandleHTTPStream instead of HandleHTTP
	Streaming bool

	// RateLimit optionally limits requests to this route
	RateLimit *RateLimit
}

// RateLimit configures rate limiting for a route.
type RateLimit struct {
	// RequestsPerMinute is the maximum requests per minute (0 = no limit)
	RequestsPerMinute int

	// PerUser limits per-user if true, otherwise global for this route
	PerUser bool
}

// HTTPRequest contains an incoming HTTP request to a plugin route.
type HTTPRequest struct {
	// Path is the request path relative to the plugin, e.g., "/search"
	Path string

	// Method is the HTTP method: GET, POST, etc.
	Method string

	// Headers contains HTTP headers (lowercase keys)
	Headers map[string]string

	// Query contains query string parameters
	Query map[string]string

	// PathParams contains parameters extracted from :param patterns
	PathParams map[string]string

	// Body is the request body (for POST, PUT, PATCH)
	Body []byte

	// UserID is the authenticated user ID (empty if not authenticated)
	UserID string

	// IsAdmin indicates whether the user has admin role
	IsAdmin bool
}

// HTTPResponse is the response from a plugin route handler.
type HTTPResponse struct {
	// StatusCode is the HTTP status code
	StatusCode int

	// Headers contains response headers
	Headers map[string]string

	// Body is the response body
	Body []byte

	// ContentType is the Content-Type header (convenience, also in Headers)
	ContentType string
}

// HTTPStreamWriter is used for streaming HTTP responses.
type HTTPStreamWriter interface {
	// WriteHeader sends the response status and headers.
	// Must be called before WriteChunk.
	WriteHeader(statusCode int, contentType string, headers map[string]string) error

	// WriteChunk sends a chunk of response data.
	WriteChunk(data []byte) error
}

// --- Response Helpers ---

// JSONResponse creates a JSON HTTP response.
// The data is marshaled to JSON automatically.
//
// Example:
//
//	return sdk.JSONResponse(200, map[string]any{"items": items})
func JSONResponse(statusCode int, data any) (*HTTPResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}
	return &HTTPResponse{
		StatusCode:  statusCode,
		ContentType: "application/json",
		Body:        body,
	}, nil
}

// JSONError creates a JSON error response.
// The message is wrapped in {"error": "message"}.
//
// Example:
//
//	return sdk.JSONError(400, "invalid request")
func JSONError(statusCode int, message string) (*HTTPResponse, error) {
	return JSONResponse(statusCode, map[string]string{"error": message})
}

// EmptyResponse creates an empty response with just a status code.
// Useful for 204 No Content responses.
//
// Example:
//
//	return sdk.EmptyResponse(204)
func EmptyResponse(statusCode int) (*HTTPResponse, error) {
	return &HTTPResponse{StatusCode: statusCode}, nil
}

// --- SSE Helpers ---

// WriteSSE writes a Server-Sent Event to a stream writer.
// This is a convenience method for writing SSE-formatted data.
//
// Example:
//
//	w.WriteHeader(200, "text/event-stream", nil)
//	sdk.WriteSSE(w, "progress", progressData)
func WriteSSE(w HTTPStreamWriter, event string, data []byte) error {
	var msg []byte
	if event != "" {
		msg = append(msg, []byte("event: "+event+"\n")...)
	}
	msg = append(msg, []byte("data: ")...)
	msg = append(msg, data...)
	msg = append(msg, '\n', '\n')
	return w.WriteChunk(msg)
}

// WriteSSEJSON writes a Server-Sent Event with JSON data.
//
// Example:
//
//	sdk.WriteSSEJSON(w, "progress", map[string]any{"percent": 50})
func WriteSSEJSON(w HTTPStreamWriter, event string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return WriteSSE(w, event, jsonData)
}

// --- Request Parsing Helpers ---

// ParseJSON parses the request body as JSON into the provided struct.
//
// Example:
//
//	var req CreateItemRequest
//	if err := sdk.ParseJSON(httpReq, &req); err != nil {
//	    return sdk.JSONError(400, "invalid JSON")
//	}
func ParseJSON(req *HTTPRequest, v any) error {
	return json.Unmarshal(req.Body, v)
}

// GetQuery gets a query parameter with a default value.
//
// Example:
//
//	limit := sdk.GetQuery(req, "limit", "10")
func GetQuery(req *HTTPRequest, key, defaultValue string) string {
	if v, ok := req.Query[key]; ok && v != "" {
		return v
	}
	return defaultValue
}

// GetPathParam gets a path parameter or returns empty string.
//
// Example:
//
//	id := sdk.GetPathParam(req, "id")
func GetPathParam(req *HTTPRequest, key string) string {
	return req.PathParams[key]
}

// GetHeader gets a header value (case-insensitive lookup assumed in Headers map).
//
// Example:
//
//	contentType := sdk.GetHeader(req, "content-type")
func GetHeader(req *HTTPRequest, key string) string {
	return req.Headers[key]
}
