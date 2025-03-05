package xlog

import (
	"context"
	"log/slog"
)

type nopHandler struct{}

func (m *nopHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return false
}

func (m *nopHandler) Handle(ctx context.Context, record slog.Record) error {
	return nil
}

func (m *nopHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return m
}

func (m *nopHandler) WithGroup(name string) slog.Handler {
	return m
}

func NewNopLogger() *slog.Logger {
	return slog.New(new(nopHandler))
}
