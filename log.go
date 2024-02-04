package rx

import (
	"context"
	"os"
	"sync/atomic"

	"log/slog"
)

var LogLevel slog.LevelVar

type genLogHandler struct {
	handler slog.Handler
	level   *slog.LevelVar

	norec *atomic.Bool
	buf   []slog.Record
}

func newLogHandler() *genLogHandler {
	return &genLogHandler{
		handler: slog.NewTextHandler(os.Stderr, nil),
		level:   &LogLevel,
		norec:   new(atomic.Bool),
	}
}

func (h genLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return !h.norec.Load() || level >= h.level.Level()
}

// TODO(rdo) sharing buffer is nice, but instead should make sure attrs are set in shared elements too
func (h *genLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &genLogHandler{level: h.level, norec: h.norec, buf: h.buf,
		handler: h.handler.WithAttrs(attrs),
	}
}

func (h *genLogHandler) WithGroup(name string) slog.Handler {
	return &genLogHandler{level: h.level, norec: h.norec, buf: h.buf,
		handler: h.handler.WithGroup(name),
	}
}

func (h *genLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= h.level.Level() {
		return h.handler.Handle(ctx, r)
	}

	h.buf = append(h.buf, r)
	return nil
}

func (h *genLogHandler) Discard() { h.buf = h.buf[:0] }
func (h *genLogHandler) Dump() {
	for _, r := range h.buf {
		h.handler.Handle(context.Background(), r)
	}
}
