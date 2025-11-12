package logger

import (
	"log/slog"
	"os"
)

// New creates a new structured logger based on the environment
func New(environment string) *slog.Logger {
	var handler slog.Handler

	// Use JSON logging in production, pretty logging in development
	if environment == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	return slog.New(handler)
}

// NewDefault creates a logger with default settings (development mode)
func NewDefault() *slog.Logger {
	return New("development")
}
