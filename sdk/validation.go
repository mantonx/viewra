package plugins

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (ve *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s (value: %v)", ve.Field, ve.Message, ve.Value)
}

// ValidationResult contains the results of a validation operation
type ValidationResults struct {
	Valid    bool
	Errors   []*ValidationError
	Warnings []string
}

// AddError adds a validation error
func (vr *ValidationResults) AddError(field, message string, value interface{}) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// AddWarning adds a validation warning
func (vr *ValidationResults) AddWarning(message string) {
	vr.Warnings = append(vr.Warnings, message)
}

// HasErrors returns true if there are any validation errors
func (vr *ValidationResults) HasErrors() bool {
	return len(vr.Errors) > 0
}

// GetErrorMessages returns all error messages
func (vr *ValidationResults) GetErrorMessages() []string {
	messages := make([]string, len(vr.Errors))
	for i, err := range vr.Errors {
		messages[i] = err.Error()
	}
	return messages
}

// PluginValidator provides validation utilities for plugin components
type PluginValidator struct{}

// NewPluginValidator creates a new plugin validator
func NewPluginValidator() *PluginValidator {
	return &PluginValidator{}
}

// ValidatePluginInfo validates plugin information structure
func (pv *PluginValidator) ValidatePluginInfo(info *PluginInfo) *ValidationResults {
	results := &ValidationResults{Valid: true}

	if info == nil {
		results.AddError("plugin_info", "cannot be nil", nil)
		return results
	}

	// Validate required fields
	if strings.TrimSpace(info.ID) == "" {
		results.AddError("id", "cannot be empty", info.ID)
	}

	if strings.TrimSpace(info.Name) == "" {
		results.AddError("name", "cannot be empty", info.Name)
	}

	if strings.TrimSpace(info.Version) == "" {
		results.AddError("version", "cannot be empty", info.Version)
	}

	if strings.TrimSpace(info.Type) == "" {
		results.AddError("type", "cannot be empty", info.Type)
	}

	// Validate plugin type
	validTypes := []string{
		PluginTypeMetadataScraper,
		PluginTypeScannerHook,
		PluginTypeAdminPage,
		PluginTypeGeneric,
	}

	isValidType := false
	for _, validType := range validTypes {
		if info.Type == validType {
			isValidType = true
			break
		}
	}

	if !isValidType {
		results.AddError("type", fmt.Sprintf("must be one of: %s", strings.Join(validTypes, ", ")), info.Type)
	}

	// Validate ID format (basic validation)
	if strings.Contains(info.ID, " ") {
		results.AddError("id", "cannot contain spaces", info.ID)
	}

	return results
}

// ValidatePluginContext validates plugin context structure
func (pv *PluginValidator) ValidatePluginContext(ctx *PluginContext) *ValidationResults {
	results := &ValidationResults{Valid: true}

	if ctx == nil {
		results.AddError("plugin_context", "cannot be nil", nil)
		return results
	}

	// Validate required fields
	if strings.TrimSpace(ctx.PluginID) == "" {
		results.AddError("plugin_id", "cannot be empty", ctx.PluginID)
	}

	if strings.TrimSpace(ctx.DatabaseURL) == "" {
		results.AddError("database_url", "cannot be empty", ctx.DatabaseURL)
	}

	if strings.TrimSpace(ctx.HostServiceAddr) == "" {
		results.AddError("host_service_addr", "cannot be empty", ctx.HostServiceAddr)
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	isValidLogLevel := false
	for _, level := range validLogLevels {
		if strings.ToLower(ctx.LogLevel) == level {
			isValidLogLevel = true
			break
		}
	}

	if !isValidLogLevel {
		results.AddError("log_level", fmt.Sprintf("must be one of: %s", strings.Join(validLogLevels, ", ")), ctx.LogLevel)
	}

	return results
}

// ValidateHealthThresholds validates health monitoring thresholds
func (pv *PluginValidator) ValidateHealthThresholds(thresholds *HealthThresholds) *ValidationResults {
	results := &ValidationResults{Valid: true}

	if thresholds == nil {
		results.AddError("health_thresholds", "cannot be nil", nil)
		return results
	}

	// Validate memory usage threshold
	if thresholds.MaxMemoryUsage < 0 {
		results.AddError("max_memory_usage", "cannot be negative", thresholds.MaxMemoryUsage)
	}

	// Validate CPU usage threshold
	if thresholds.MaxCPUUsage < 0 || thresholds.MaxCPUUsage > 100 {
		results.AddError("max_cpu_usage", "must be between 0 and 100", thresholds.MaxCPUUsage)
	}

	// Validate error rate threshold
	if thresholds.MaxErrorRate < 0 || thresholds.MaxErrorRate > 100 {
		results.AddError("max_error_rate", "must be between 0 and 100", thresholds.MaxErrorRate)
	}

	// Validate response time threshold
	if thresholds.MaxResponseTime < 0 {
		results.AddError("max_response_time", "cannot be negative", thresholds.MaxResponseTime)
	}

	// Validate health check interval
	if thresholds.HealthCheckInterval <= 0 {
		results.AddError("health_check_interval", "must be positive", thresholds.HealthCheckInterval)
	}

	// Add warnings for potentially problematic values
	if thresholds.MaxMemoryUsage > 1024*1024*1024 { // 1GB
		results.AddWarning("max_memory_usage is quite high (>1GB), consider lowering it")
	}

	if thresholds.MaxResponseTime > 30*time.Second {
		results.AddWarning("max_response_time is quite high (>30s), consider lowering it")
	}

	if thresholds.HealthCheckInterval < 5*time.Second {
		results.AddWarning("health_check_interval is quite low (<5s), may cause performance overhead")
	}

	return results
}

// ValidateImplementation validates that a plugin implementation satisfies the required interface
func (pv *PluginValidator) ValidateImplementation(impl Implementation) *ValidationResults {
	results := &ValidationResults{Valid: true}

	if impl == nil {
		results.AddError("implementation", "cannot be nil", nil)
		return results
	}

	// Use reflection to validate interface compliance
	implType := reflect.TypeOf(impl)
	implValue := reflect.ValueOf(impl)

	// Check that it's a pointer to a struct
	if implType.Kind() != reflect.Ptr || implType.Elem().Kind() != reflect.Struct {
		results.AddError("implementation", "must be a pointer to a struct", implType.Kind())
		return results
	}

	// Validate required methods exist and are callable
	requiredMethods := []string{"Initialize", "Start", "Stop", "Info", "Health"}

	for _, methodName := range requiredMethods {
		method := implValue.MethodByName(methodName)
		if !method.IsValid() {
			results.AddError("implementation", fmt.Sprintf("missing required method: %s", methodName), methodName)
			continue
		}

		if !method.CanInterface() {
			results.AddError("implementation", fmt.Sprintf("method %s is not accessible", methodName), methodName)
		}
	}

	return results
}

// ValidateServiceIntegration validates that optional services are properly implemented
func (pv *PluginValidator) ValidateServiceIntegration(impl Implementation) *ValidationResults {
	results := &ValidationResults{Valid: true}

	if impl == nil {
		results.AddError("implementation", "cannot be nil", nil)
		return results
	}

	// Check optional service implementations
	services := map[string]interface{}{
		"MetadataScraperService":    impl.MetadataScraperService(),
		"ScannerHookService":        impl.ScannerHookService(),
		"AssetService":              impl.AssetService(),
		"DatabaseService":           impl.DatabaseService(),
		"AdminPageService":          impl.AdminPageService(),
		"APIRegistrationService":    impl.APIRegistrationService(),
		"SearchService":             impl.SearchService(),
		"HealthMonitorService":      impl.HealthMonitorService(),
		"ConfigurationService":      impl.ConfigurationService(),
		"PerformanceMonitorService": impl.PerformanceMonitorService(),
	}

	servicesImplemented := 0
	for serviceName, service := range services {
		if service != nil {
			servicesImplemented++

			// Validate that the service interface is properly implemented
			serviceType := reflect.TypeOf(service)
			if serviceType.Kind() != reflect.Ptr && serviceType.Kind() != reflect.Interface {
				results.AddError(serviceName, "service must be a pointer or interface", serviceType.Kind())
			}
		}
	}

	// Warn if no services are implemented
	if servicesImplemented == 0 {
		results.AddWarning("no optional services implemented - plugin may have limited functionality")
	}

	return results
}

// ValidatePluginConfiguration validates a complete plugin configuration
func (pv *PluginValidator) ValidatePluginConfiguration(config *PluginConfiguration) *ValidationResults {
	results := &ValidationResults{Valid: true}

	if config == nil {
		results.AddError("plugin_configuration", "cannot be nil", nil)
		return results
	}

	// Validate version
	if strings.TrimSpace(config.Version) == "" {
		results.AddError("version", "cannot be empty", config.Version)
	}

	// Validate settings structure
	if config.Settings == nil {
		results.AddWarning("no settings configured - plugin may use defaults")
	}

	// Validate features structure
	if config.Features == nil {
		results.AddWarning("no features configured - plugin may use defaults")
	}

	// Validate thresholds if provided
	if config.Thresholds != nil {
		thresholdResults := pv.ValidateHealthThresholds(config.Thresholds)
		if !thresholdResults.Valid {
			for _, err := range thresholdResults.Errors {
				results.AddError(fmt.Sprintf("thresholds.%s", err.Field), err.Message, err.Value)
			}
		}
		results.Warnings = append(results.Warnings, thresholdResults.Warnings...)
	}

	// Validate modification tracking
	if config.LastModified.IsZero() {
		results.AddWarning("last_modified timestamp is not set")
	}

	if strings.TrimSpace(config.ModifiedBy) == "" {
		results.AddWarning("modified_by field is not set")
	}

	return results
}

// Common validation errors
var (
	ErrInvalidPluginInfo       = errors.New("invalid plugin info")
	ErrInvalidPluginContext    = errors.New("invalid plugin context")
	ErrInvalidHealthThresholds = errors.New("invalid health thresholds")
	ErrInvalidImplementation   = errors.New("invalid plugin implementation")
	ErrInvalidConfiguration    = errors.New("invalid plugin configuration")
)
