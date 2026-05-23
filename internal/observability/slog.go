package observability

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

// NewLogger возвращает slog.Logger, который пишет одновременно в stdout (для `docker logs`)
// и в OTel LoggerProvider (откуда улетает в Loki через collector).
//
// otelslog автоматически достаёт trace_id/span_id из контекста, поэтому если вызывать
// log.InfoContext(ctx, ...) или LogAttrs(ctx, ...) — в логах будет корреляция с трейсами.
func NewLogger(serviceName string, level slog.Level) *slog.Logger {
	stdHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	otelHandler := otelslog.NewHandler(serviceName)

	return slog.New(&multiHandler{
		hs: []slog.Handler{stdHandler, otelHandler},
	})
}

// NewStdoutLogger — fallback на случай, когда OTel выключен.
// Только stdout, без bridge'а.
func NewStdoutLogger(level slog.Level, format string) *slog.Logger {
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	return slog.New(h)
}

// multiHandler — fan-out для slog: одна запись разлетается во все вложенные handlers.
type multiHandler struct {
	hs []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	for _, h := range m.hs {
		if h.Enabled(ctx, lvl) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range m.hs {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, len(m.hs))
	for i, h := range m.hs {
		out[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{hs: out}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, len(m.hs))
	for i, h := range m.hs {
		out[i] = h.WithGroup(name)
	}
	return &multiHandler{hs: out}
}
