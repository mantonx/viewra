package plugins

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
)

// LoadPluginConfig is a convenience function for plugins to load their configuration
// It should be called in the Initialize method after setting up the logger
func LoadPluginConfig(ctx *PluginContext, config interface{}) error {
	if ctx == nil {
		return fmt.Errorf("plugin context is required")
	}

	if config == nil {
		return fmt.Errorf("config struct is required")
	}

	// Create config loader
	pluginDir := ctx.PluginBasePath
	logger := &stdLogger{prefix: fmt.Sprintf("[%s] ", ctx.PluginID)}
	configLoader := NewConfigLoader(pluginDir, ctx.PluginID, logger)

	// Extract runtime config from context if available
	var runtimeConfig map[string]string
	// TODO: When PluginContext gets Config field, extract it here
	// For now, create empty map
	runtimeConfig = make(map[string]string)

	// Load configuration with proper priority
	return configLoader.LoadConfig(config, runtimeConfig)
}

// stdLogger implements the Logger interface for config loading
type stdLogger struct {
	prefix string
}

func (l *stdLogger) Info(msg string, keysAndValues ...interface{}) {
	fmt.Printf("INFO: %s%s %v\n", l.prefix, msg, keysAndValues)
}

func (l *stdLogger) Warn(msg string, keysAndValues ...interface{}) {
	fmt.Printf("WARN: %s%s %v\n", l.prefix, msg, keysAndValues)
}

func (l *stdLogger) Error(msg string, keysAndValues ...interface{}) {
	fmt.Printf("ERROR: %s%s %v\n", l.prefix, msg, keysAndValues)
}

func (l *stdLogger) Debug(msg string, keysAndValues ...interface{}) {
	fmt.Printf("DEBUG: %s%s %v\n", l.prefix, msg, keysAndValues)
}

func (l *stdLogger) With(keysAndValues ...interface{}) hclog.Logger {
	// Simple implementation that returns a basic hclog logger
	// In a real implementation, this would create a new logger with additional context
	return hclog.New(&hclog.LoggerOptions{
		Name:  l.prefix,
		Level: hclog.Debug,
	})
}
