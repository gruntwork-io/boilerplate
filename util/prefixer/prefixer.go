package prefixer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

type Handler struct {
	h      slog.Handler
	m      *sync.Mutex
	writer io.Writer
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.h.Enabled(ctx, level)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{h: h.h.WithAttrs(attrs), m: h.m, writer: h.writer}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{h: h.h.WithGroup(name), m: h.m, writer: h.writer}
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
	buf := &bytes.Buffer{}
	handler := &Handler{
		h:      slog.NewTextHandler(buf, &slog.HandlerOptions{}),
		m:      &sync.Mutex{},
		writer: os.Stdout,
	}

	return handler
}
