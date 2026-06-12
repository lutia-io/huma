package logger

import (
	"context"
	"log/slog"

	"github.com/lutia-io/huma/pkg/requestid"
)

type Logger struct {
	inner *slog.Logger
}

func New() *Logger {
	return &Logger{
		inner: slog.Default(),
	}
}

func (l *Logger) Slog() *slog.Logger {
	return l.inner
}

func (l *Logger) attrsFromContext(ctx context.Context, args []any) []any {
	if rid, ok := requestid.FromContext(ctx); ok {
		return append([]any{slog.String(KeyRequestID, rid)}, args...)
	}
	return args
}

func (l *Logger) Info(msg string, args ...any) {
	l.inner.Info(msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.inner.Error(msg, args...)
}

func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.inner.InfoContext(ctx, msg, l.attrsFromContext(ctx, args)...)
}

func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.inner.WarnContext(ctx, msg, l.attrsFromContext(ctx, args)...)
}

func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.inner.ErrorContext(ctx, msg, l.attrsFromContext(ctx, args)...)
}
