// Package logger provides a log/slog-backed application.Logger implementation.
package logger

import (
	"log/slog"
	"os"

	"github.com/example/di_in_go/internal/application"
)

type slogLogger struct{ l *slog.Logger }

// New returns a text-format Logger writing to stdout.
// Lifecycle: Singleton — create once in main, inject everywhere.
func New() application.Logger {
	return &slogLogger{l: slog.New(slog.NewTextHandler(os.Stdout, nil))}
}

func (s *slogLogger) Info(msg string, args ...any)  { s.l.Info(msg, args...) }
func (s *slogLogger) Error(msg string, args ...any) { s.l.Error(msg, args...) }

// With returns a new Logger with the given key-value pairs pre-attached.
// Used to create per-request enriched loggers (scoped) from the singleton base.
func (s *slogLogger) With(args ...any) application.Logger {
	return &slogLogger{l: s.l.With(args...)}
}
