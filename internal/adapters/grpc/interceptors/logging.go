package interceptors

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryLogging — структурированный лог на каждый вызов:
// метод, gRPC code, latency.
// Тело запросов/ответов не логируем — может содержать секреты.
func UnaryLogging(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		st, _ := status.FromError(err)

		attrs := []any{
			slog.String("method", info.FullMethod),
			slog.String("code", st.Code().String()),
			slog.Duration("duration", time.Since(start)),
		}
		if err != nil && st.Code() != 0 {
			attrs = append(attrs, slog.String("err", st.Message()))
		}

		// LogAttrs с ctx — чтобы otelslog-bridge подтянул trace_id/span_id
		// из активного span'а и положил в OTel-лог. Без ctx корреляции с трейсами не будет.
		slogAttrs := make([]slog.Attr, 0, len(attrs))
		for _, a := range attrs {
			if attr, ok := a.(slog.Attr); ok {
				slogAttrs = append(slogAttrs, attr)
			}
		}
		log.LogAttrs(ctx, slog.LevelInfo, "grpc call", slogAttrs...)
		return resp, err
	}
}
