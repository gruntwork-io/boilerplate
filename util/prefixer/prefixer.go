package prefixer

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

type Handler struct {
	writer io.Writer
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return h
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	msg := slog.StringValue(r.Message).String()
	if msg != "" {
		_, err := io.WriteString(h.writer, fmt.Sprintf("[boilerplate] %s\n", msg))
		return err
	}

	return nil
}

func New() *Handler {
	handler := &Handler{
		writer: os.Stderr,
	}

	return handler
}
