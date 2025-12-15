package plugins

import (
	"io"
	"log"
	"log/slog"

	"github.com/hashicorp/go-hclog"
)

// hclogAdapter adapts slog.Logger to hclog.Logger interface.
// go-plugin uses hclog, so we need this adapter.
type hclogAdapter struct {
	logger *slog.Logger
	name   string
	args   []interface{}
}

// newHCLogAdapter creates a new hclog adapter wrapping the given slog logger.
func newHCLogAdapter(logger *slog.Logger) hclog.Logger {
	return &hclogAdapter{
		logger: logger,
		name:   "",
		args:   nil,
	}
}

func (l *hclogAdapter) Log(level hclog.Level, msg string, args ...interface{}) {
	allArgs := append(l.args, args...)
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

func (l *hclogAdapter) Trace(msg string, args ...interface{}) {
	l.Log(hclog.Trace, msg, args...)
}

func (l *hclogAdapter) Debug(msg string, args ...interface{}) {
	l.Log(hclog.Debug, msg, args...)
}

func (l *hclogAdapter) Info(msg string, args ...interface{}) {
	l.Log(hclog.Info, msg, args...)
}

func (l *hclogAdapter) Warn(msg string, args ...interface{}) {
	l.Log(hclog.Warn, msg, args...)
}

func (l *hclogAdapter) Error(msg string, args ...interface{}) {
	l.Log(hclog.Error, msg, args...)
}

func (l *hclogAdapter) IsTrace() bool {
	return l.logger.Enabled(nil, slog.LevelDebug-4)
}

func (l *hclogAdapter) IsDebug() bool {
	return l.logger.Enabled(nil, slog.LevelDebug)
}

func (l *hclogAdapter) IsInfo() bool {
	return l.logger.Enabled(nil, slog.LevelInfo)
}

func (l *hclogAdapter) IsWarn() bool {
	return l.logger.Enabled(nil, slog.LevelWarn)
}

func (l *hclogAdapter) IsError() bool {
	return l.logger.Enabled(nil, slog.LevelError)
}

func (l *hclogAdapter) ImpliedArgs() []interface{} {
	return l.args
}

func (l *hclogAdapter) With(args ...interface{}) hclog.Logger {
	newArgs := make([]interface{}, len(l.args)+len(args))
	copy(newArgs, l.args)
	copy(newArgs[len(l.args):], args)
	return &hclogAdapter{
		logger: l.logger,
		name:   l.name,
		args:   newArgs,
	}
}

func (l *hclogAdapter) Name() string {
	return l.name
}

func (l *hclogAdapter) Named(name string) hclog.Logger {
	newName := name
	if l.name != "" {
		newName = l.name + "." + name
	}
	return &hclogAdapter{
		logger: l.logger.With("logger", newName),
		name:   newName,
		args:   l.args,
	}
}

func (l *hclogAdapter) ResetNamed(name string) hclog.Logger {
	return &hclogAdapter{
		logger: l.logger.With("logger", name),
		name:   name,
		args:   l.args,
	}
}

func (l *hclogAdapter) SetLevel(level hclog.Level) {
	// slog doesn't support dynamic level changes through the Logger interface
}

func (l *hclogAdapter) GetLevel() hclog.Level {
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

func (l *hclogAdapter) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(l.StandardWriter(opts), "", 0)
}

func (l *hclogAdapter) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
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
