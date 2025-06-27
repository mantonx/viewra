package errors

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
)

// EnhancedError provides detailed error information with stack traces
type EnhancedError struct {
	*ViewraError
	StackTrace  []StackFrame      `json:"stack_trace,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	RequestID   string            `json:"request_id,omitempty"`
	UserAgent   string            `json:"user_agent,omitempty"`
	RequestPath string            `json:"request_path,omitempty"`
	Method      string            `json:"method,omitempty"`
	RemoteAddr  string            `json:"remote_addr,omitempty"`
	InnerErrors []error           `json:"inner_errors,omitempty"`
	Breadcrumbs []Breadcrumb      `json:"breadcrumbs,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// StackFrame represents a single frame in a stack trace
type StackFrame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Package  string `json:"package"`
}

// Breadcrumb represents a breadcrumb for error tracking
type Breadcrumb struct {
	Timestamp time.Time              `json:"timestamp"`
	Message   string                 `json:"message"`
	Category  string                 `json:"category"`
	Level     string                 `json:"level"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// ErrorReporter handles enhanced error reporting
type ErrorReporter struct {
	enableStackTrace  bool
	maxStackDepth     int
	enableBreadcrumbs bool
	maxBreadcrumbs    int
	breadcrumbs       []Breadcrumb
}

// NewErrorReporter creates a new enhanced error reporter
func NewErrorReporter(enableStackTrace, enableBreadcrumbs bool) *ErrorReporter {
	return &ErrorReporter{
		enableStackTrace:  enableStackTrace,
		maxStackDepth:     50,
		enableBreadcrumbs: enableBreadcrumbs,
		maxBreadcrumbs:    20,
		breadcrumbs:       make([]Breadcrumb, 0),
	}
}

// NewEnhancedError creates a new enhanced error with stack trace
func NewEnhancedError(code, message string, cause error) *EnhancedError {
	enhanced := &EnhancedError{
		ViewraError: &ViewraError{
			Code:    code,
			Message: message,
			Cause:   cause,
			Context: make(map[string]interface{}),
		},
		Timestamp:   time.Now(),
		InnerErrors: []error{},
		Breadcrumbs: []Breadcrumb{},
		Tags:        make(map[string]string),
	}

	// Capture stack trace
	enhanced.StackTrace = captureStackTrace(3, 50) // Skip NewEnhancedError and callers

	return enhanced
}

// NewEnhancedErrorFromGin creates an enhanced error from a gin context
func NewEnhancedErrorFromGin(c *gin.Context, code, message string, cause error) *EnhancedError {
	enhanced := NewEnhancedError(code, message, cause)

	// Add gin context information
	enhanced.RequestPath = c.Request.URL.Path
	enhanced.Method = c.Request.Method
	enhanced.UserAgent = c.GetHeader("User-Agent")
	enhanced.RemoteAddr = c.ClientIP()

	// Get request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		enhanced.RequestID = fmt.Sprintf("%v", requestID)
	}

	// Add query parameters as context
	if len(c.Request.URL.RawQuery) > 0 {
		enhanced.Context["query"] = c.Request.URL.RawQuery
	}

	return enhanced
}

// captureStackTrace captures the current stack trace
func captureStackTrace(skip, maxDepth int) []StackFrame {
	frames := make([]StackFrame, 0)

	for i := skip; i < skip+maxDepth; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}

		funcName := fn.Name()

		// Extract package name
		var packageName string
		if lastSlash := strings.LastIndex(funcName, "/"); lastSlash >= 0 {
			if lastDot := strings.LastIndex(funcName[lastSlash:], "."); lastDot >= 0 {
				packageName = funcName[:lastSlash+lastDot]
				funcName = funcName[lastSlash+lastDot+1:]
			}
		} else if lastDot := strings.LastIndex(funcName, "."); lastDot >= 0 {
			packageName = funcName[:lastDot]
			funcName = funcName[lastDot+1:]
		}

		frame := StackFrame{
			Function: funcName,
			File:     file,
			Line:     line,
			Package:  packageName,
		}

		frames = append(frames, frame)
	}

	return frames
}

// WithContext adds context to the error
func (e *EnhancedError) WithContext(key string, value interface{}) *EnhancedError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithTag adds a tag to the error
func (e *EnhancedError) WithTag(key, value string) *EnhancedError {
	if e.Tags == nil {
		e.Tags = make(map[string]string)
	}
	e.Tags[key] = value
	return e
}

// WithInnerError adds an inner error
func (e *EnhancedError) WithInnerError(err error) *EnhancedError {
	e.InnerErrors = append(e.InnerErrors, err)
	return e
}

// AddBreadcrumb adds a breadcrumb to the error reporter
func (er *ErrorReporter) AddBreadcrumb(message, category, level string, data map[string]interface{}) {
	if !er.enableBreadcrumbs {
		return
	}

	breadcrumb := Breadcrumb{
		Timestamp: time.Now(),
		Message:   message,
		Category:  category,
		Level:     level,
		Data:      data,
	}

	er.breadcrumbs = append(er.breadcrumbs, breadcrumb)

	// Keep only the most recent breadcrumbs
	if len(er.breadcrumbs) > er.maxBreadcrumbs {
		er.breadcrumbs = er.breadcrumbs[1:]
	}
}

// ReportError reports an enhanced error with full context
func (er *ErrorReporter) ReportError(err *EnhancedError) {
	// Add current breadcrumbs to the error
	if er.enableBreadcrumbs {
		err.Breadcrumbs = append(err.Breadcrumbs, er.breadcrumbs...)
	}

	// Log the error with structured data
	fields := []interface{}{
		"error_code", err.Code,
		"error_message", err.Message,
		"timestamp", err.Timestamp,
	}

	if err.RequestID != "" {
		fields = append(fields, "request_id", err.RequestID)
	}

	if err.RequestPath != "" {
		fields = append(fields, "path", err.RequestPath, "method", err.Method)
	}

	if len(err.Tags) > 0 {
		fields = append(fields, "tags", err.Tags)
	}

	if len(err.Context) > 0 {
		fields = append(fields, "context", err.Context)
	}

	logger.Error("Enhanced error reported", fields...)

	// Log stack trace if enabled
	if er.enableStackTrace && len(err.StackTrace) > 0 {
		logger.Debug("Stack trace", "frames", err.StackTrace)
	}

	// Log breadcrumbs if available
	if len(err.Breadcrumbs) > 0 {
		logger.Debug("Error breadcrumbs", "breadcrumbs", err.Breadcrumbs)
	}
}

// ToGinResponse sends the enhanced error as a JSON response
func (e *EnhancedError) ToGinResponse(c *gin.Context) {
	statusCode := e.HTTPStatus
	if statusCode == 0 {
		statusCode = 500
	}

	response := gin.H{
		"error":     e.Message,
		"code":      e.Code,
		"timestamp": e.Timestamp,
	}

	// Include request ID if available
	if e.RequestID != "" {
		response["request_id"] = e.RequestID
	}

	// Include context in development mode
	if gin.Mode() == gin.DebugMode && len(e.Context) > 0 {
		response["details"] = e.Context
	}

	// Include stack trace in development mode
	if gin.Mode() == gin.DebugMode && len(e.StackTrace) > 0 {
		response["stack_trace"] = e.formatStackTrace()
	}

	c.JSON(statusCode, response)
}

// formatStackTrace formats the stack trace for display
func (e *EnhancedError) formatStackTrace() []string {
	formatted := make([]string, len(e.StackTrace))

	for i, frame := range e.StackTrace {
		formatted[i] = fmt.Sprintf("%s:%d %s()", frame.File, frame.Line, frame.Function)
	}

	return formatted
}

// Error implements the error interface
func (e *EnhancedError) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Message)

	if e.Cause != nil {
		sb.WriteString(": ")
		sb.WriteString(e.Cause.Error())
	}

	if len(e.InnerErrors) > 0 {
		sb.WriteString(" (with ")
		sb.WriteString(fmt.Sprintf("%d", len(e.InnerErrors)))
		sb.WriteString(" inner errors)")
	}

	return sb.String()
}

// Convenience functions for common enhanced errors

// NewEnhancedValidationError creates an enhanced validation error
func NewEnhancedValidationError(message, field string) *EnhancedError {
	return NewEnhancedError("VALIDATION_ERROR", message, nil).
		WithContext("field", field).
		WithTag("type", "validation")
}

// NewEnhancedNotFoundError creates an enhanced not found error
func NewEnhancedNotFoundError(resource, id string) *EnhancedError {
	return NewEnhancedError("NOT_FOUND", resource+" not found", nil).
		WithContext("resource", resource).
		WithContext("id", id).
		WithTag("type", "not_found")
}

// NewEnhancedDatabaseError creates an enhanced database error
func NewEnhancedDatabaseError(operation string, cause error) *EnhancedError {
	return NewEnhancedError("DATABASE_ERROR", "Database operation failed", cause).
		WithContext("operation", operation).
		WithTag("type", "database")
}

// NewEnhancedPluginError creates an enhanced plugin error
func NewEnhancedPluginError(pluginID, operation string, cause error) *EnhancedError {
	return NewEnhancedError("PLUGIN_ERROR", "Plugin operation failed", cause).
		WithContext("plugin", pluginID).
		WithContext("operation", operation).
		WithTag("type", "plugin")
}

// Gin middleware for enhanced error handling

// EnhancedErrorMiddleware creates a gin middleware for enhanced error handling
func EnhancedErrorMiddleware(reporter *ErrorReporter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add request breadcrumb
		reporter.AddBreadcrumb(
			fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path),
			"request",
			"info",
			map[string]interface{}{
				"method": c.Request.Method,
				"path":   c.Request.URL.Path,
				"ip":     c.ClientIP(),
			},
		)

		// Set request ID
		requestID := generateRequestID()
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// Continue processing
		c.Next()

		// Check for errors
		if len(c.Errors) > 0 {
			for _, ginErr := range c.Errors {
				if enhanced, ok := ginErr.Err.(*EnhancedError); ok {
					// Add request context if not already set
					if enhanced.RequestID == "" {
						enhanced.RequestID = requestID
						enhanced.RequestPath = c.Request.URL.Path
						enhanced.Method = c.Request.Method
						enhanced.UserAgent = c.GetHeader("User-Agent")
						enhanced.RemoteAddr = c.ClientIP()
					}

					reporter.ReportError(enhanced)
				} else {
					// Create enhanced error from gin error
					enhanced := NewEnhancedErrorFromGin(c, "GIN_ERROR", ginErr.Error(), ginErr.Err)
					reporter.ReportError(enhanced)
				}
			}
		}
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Recovery middleware with enhanced error reporting
func EnhancedRecoveryMiddleware(reporter *ErrorReporter) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		var err error
		if e, ok := recovered.(error); ok {
			err = e
		} else {
			err = fmt.Errorf("%v", recovered)
		}

		enhanced := NewEnhancedErrorFromGin(c, "PANIC", "Panic recovered", err).
			WithTag("type", "panic").
			WithContext("recovered", recovered)

		reporter.ReportError(enhanced)
		enhanced.ToGinResponse(c)
	})
}
