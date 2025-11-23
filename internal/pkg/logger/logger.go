package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New creates a new structured logger based on the environment
func New(environment string) *slog.Logger {
	var handler slog.Handler

	// Determine log level from environment variable, defaulting based on environment
	logLevel := getLogLevel(environment)

	// Use JSON logging in production, pretty logging in development
	if environment == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		})
	}

	return slog.New(handler)
}

// getLogLevel determines the log level from LOG_LEVEL env var or defaults
func getLogLevel(environment string) slog.Level {
	levelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))

	switch levelStr {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	case "":
		// No LOG_LEVEL set - use defaults based on environment
		if environment == "production" {
			return slog.LevelInfo
		}
		return slog.LevelInfo // Default to INFO for dev to reduce noise
	default:
		// Invalid LOG_LEVEL - use default
		if environment == "production" {
			return slog.LevelInfo
		}
		return slog.LevelInfo
	}
}

// NewDefault creates a logger with default settings (development mode)
func NewDefault() *slog.Logger {
	return New("development")
}
