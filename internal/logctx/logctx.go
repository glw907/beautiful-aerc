package logctx

import (
	"context"
	"log/slog"
)

type opIDKey struct{}

// WithOpID stores id in ctx under the op_id log attribute key.
func WithOpID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, opIDKey{}, id)
}

// Handler injects op_id from the context into every log record.
type Handler struct {
	slog.Handler
}

func (h Handler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := ctx.Value(opIDKey{}).(string); ok {
		r.AddAttrs(slog.String("op_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return Handler{h.Handler.WithAttrs(attrs)}
}

func (h Handler) WithGroup(name string) slog.Handler {
	return Handler{h.Handler.WithGroup(name)}
}
