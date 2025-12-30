package logging

import (
	"context"
	"io"
	"log"
	"log/slog"

	"github.com/hashicorp/go-hclog"
)

// HclogAdapter adapts slog.Logger to hclog.Logger interface.
// go-plugin uses hclog, so we need this adapter.
type HclogAdapter struct {
	logger *slog.Logger
	name   string
	args   []interface{}
}

// NewHclogAdapter creates a new hclog adapter wrapping the given slog logger.
func NewHclogAdapter(logger *slog.Logger) hclog.Logger {
	return &HclogAdapter{
		logger: logger,
		name:   "",
		args:   nil,
	}
}

// Log writes a log message at the specified level.
func (l *HclogAdapter) Log(level hclog.Level, msg string, args ...interface{}) {
	allArgs := make([]interface{}, 0, len(l.args)+len(args))
	allArgs = append(allArgs, l.args...)
	allArgs = append(allArgs, args...)
	switch level {
	case hclog.Trace, hclog.Debug:
		l.logger.Debug(msg, allArgs...)
	case hclog.Info:
		l.logger.Info(msg, allArgs...)
	case hclog.Warn:
		l.logger.Warn(msg, allArgs...)
	case hclog.Error:
		l.logger.Error(msg, allArgs...)
	default:
		l.logger.Info(msg, allArgs...)
	}
}

// Trace implements hclog.Logger.
func (l *HclogAdapter) Trace(msg string, args ...interface{}) {
	l.Log(hclog.Trace, msg, args...)
}

// Debug implements hclog.Logger.
func (l *HclogAdapter) Debug(msg string, args ...interface{}) {
	l.Log(hclog.Debug, msg, args...)
}

// Info implements hclog.Logger.
func (l *HclogAdapter) Info(msg string, args ...interface{}) {
	l.Log(hclog.Info, msg, args...)
}

// Warn implements hclog.Logger.
func (l *HclogAdapter) Warn(msg string, args ...interface{}) {
	l.Log(hclog.Warn, msg, args...)
}

// Error implements hclog.Logger.
func (l *HclogAdapter) Error(msg string, args ...interface{}) {
	l.Log(hclog.Error, msg, args...)
}

// IsTrace implements hclog.Logger.
func (l *HclogAdapter) IsTrace() bool {
	return l.logger.Enabled(context.TODO(), slog.LevelDebug-4)
}

// IsDebug implements hclog.Logger.
func (l *HclogAdapter) IsDebug() bool {
	return l.logger.Enabled(context.TODO(), slog.LevelDebug)
}

// IsInfo implements hclog.Logger.
func (l *HclogAdapter) IsInfo() bool {
	return l.logger.Enabled(context.TODO(), slog.LevelInfo)
}

// IsWarn implements hclog.Logger.
func (l *HclogAdapter) IsWarn() bool {
	return l.logger.Enabled(context.TODO(), slog.LevelWarn)
}

// IsError implements hclog.Logger.
func (l *HclogAdapter) IsError() bool {
	return l.logger.Enabled(context.TODO(), slog.LevelError)
}

// ImpliedArgs implements hclog.Logger.
func (l *HclogAdapter) ImpliedArgs() []interface{} {
	return l.args
}

// With implements hclog.Logger.
func (l *HclogAdapter) With(args ...interface{}) hclog.Logger {
	newArgs := make([]interface{}, len(l.args)+len(args))
	copy(newArgs, l.args)
	copy(newArgs[len(l.args):], args)
	return &HclogAdapter{
		logger: l.logger,
		name:   l.name,
		args:   newArgs,
	}
}

// Name implements hclog.Logger.
func (l *HclogAdapter) Name() string {
	return l.name
}

// Named implements hclog.Logger.
func (l *HclogAdapter) Named(name string) hclog.Logger {
	newName := name
	if l.name != "" {
		newName = l.name + "." + name
	}
	return &HclogAdapter{
		logger: l.logger.With("logger", newName),
		name:   newName,
		args:   l.args,
	}
}

// ResetNamed implements hclog.Logger.
func (l *HclogAdapter) ResetNamed(name string) hclog.Logger {
	return &HclogAdapter{
		logger: l.logger.With("logger", name),
		name:   name,
		args:   l.args,
	}
}

// SetLevel implements hclog.Logger. slog doesn't support dynamic level changes.
func (l *HclogAdapter) SetLevel(_ hclog.Level) {
	// slog doesn't support dynamic level changes through the Logger interface
}

// GetLevel implements hclog.Logger.
func (l *HclogAdapter) GetLevel() hclog.Level {
	if l.IsTrace() {
		return hclog.Trace
	}
	if l.IsDebug() {
		return hclog.Debug
	}
	if l.IsInfo() {
		return hclog.Info
	}
	if l.IsWarn() {
		return hclog.Warn
	}
	return hclog.Error
}

// StandardLogger implements hclog.Logger.
func (l *HclogAdapter) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(l.StandardWriter(opts), "", 0)
}

// StandardWriter implements hclog.Logger.
func (l *HclogAdapter) StandardWriter(_ *hclog.StandardLoggerOptions) io.Writer {
	return &slogWriter{logger: l.logger}
}

// slogWriter adapts slog.Logger to io.Writer
type slogWriter struct {
	logger *slog.Logger
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	w.logger.Info(string(p))
	return len(p), nil
}
