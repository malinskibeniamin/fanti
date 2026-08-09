package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"connectrpc.com/connect"
)

// AccessLog returns a connect interceptor that logs every RPC as
// structured JSON with a request id, duration, and status.
func AccessLog(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			requestID := newRequestID()

			resp, err := next(ctx, req)

			attrs := []slog.Attr{
				slog.String("request_id", requestID),
				slog.String("procedure", req.Spec().Procedure),
				slog.Duration("duration", time.Since(start)),
			}

			if err != nil {
				attrs = append(attrs,
					slog.String("status", connect.CodeOf(err).String()),
					slog.String("error", err.Error()))
				logger.LogAttrs(ctx, slog.LevelWarn, "rpc", attrs...)

				return nil, err
			}

			attrs = append(attrs, slog.String("status", "ok"))
			logger.LogAttrs(ctx, slog.LevelInfo, "rpc", attrs...)

			return resp, nil
		}
	}
}

func newRequestID() string {
	var b [8]byte

	_, _ = rand.Read(b[:])

	return hex.EncodeToString(b[:])
}
