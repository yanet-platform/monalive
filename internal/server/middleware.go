package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/yanet-platform/monalive/internal/types/requestid"
)

// requestIDMiddleware adds a request ID to the request context and response headers
func requestIDMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate a new request ID
			reqID := requestid.Generate()

			// Add request ID to the context
			ctx := requestid.NewContext(r.Context(), reqID)
			r = r.WithContext(ctx)

			// Add request ID to response headers
			w.Header().Set(requestid.HeaderKey, string(reqID))

			logger.Info("HTTP request received",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("request_id", string(reqID)),
			)

			start := time.Now()
			next.ServeHTTP(w, r)

			logger.Info("HTTP request completed",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("request_id", string(reqID)),
				slog.String("duration", time.Since(start).String()),
			)
		})
	}
}
