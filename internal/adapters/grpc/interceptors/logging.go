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

		log.Info("grpc call", attrs...)
		return resp, err
	}
}
