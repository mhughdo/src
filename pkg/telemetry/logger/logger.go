package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
)

type ctxKey string

const (
	RemoteAddrKey ctxKey = "remote_addr"
)

var (
	ctxKeys = []ctxKey{
		RemoteAddrKey,
	}
)

var defaultLogger atomic.Pointer[slog.Logger]

func init() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	ctxHandler := ContextHandler{Handler: handler}
	defaultLogger.Store(slog.New(ctxHandler))
}

type ContextHandler struct {
	slog.Handler
}

func Debug(ctx context.Context, template string, args ...any) {
	defaultLogger.Load().DebugContext(ctx, fmt.Sprintf(template, args...))
}

func Info(ctx context.Context, template string, args ...any) {
	defaultLogger.Load().InfoContext(ctx, fmt.Sprintf(template, args...))
}

func Warn(ctx context.Context, template string, args ...any) {
	defaultLogger.Load().WarnContext(ctx, fmt.Sprintf(template, args...))
}

func Error(ctx context.Context, template string, args ...any) {
	defaultLogger.Load().ErrorContext(ctx, fmt.Sprintf(template, args...))
}

// Handle adds contextual attributes to the Record before calling the underlying handler
func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, k := range ctxKeys {
		v := ctx.Value(k)
		if v == nil {
			continue
		}
		r.AddAttrs(slog.String(string(k), fmt.Sprintf("%v", ctx.Value(k))))
	}

	return h.Handler.Handle(ctx, r)
}

// AppendCtx adds an slog attribute to the provided context so that it will be
// included in any Record created with such context
func AppendCtx(parent context.Context, attr slog.Attr) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	ctx := parent
	for _, k := range ctxKeys {
		v, ok := ctx.Value(k).([]slog.Attr)
		if ok {
			v = append(v, attr)
			ctx = context.WithValue(ctx, k, v)
			continue
		}
		v = []slog.Attr{}
		v = append(v, attr)
		ctx = context.WithValue(ctx, k, v)
	}

	return ctx
}
