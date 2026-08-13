package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}

	w.status = status
	w.wroteHeader = true

	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	return w.ResponseWriter.Write(body)
}

func requestLoggingMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		requestID := uuid.NewString()

		ctx := withRequestID(r.Context(), requestID)
		r = r.WithContext(ctx)

		writer := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(writer, r)

		logger.Info(
			"http request completed",
			zap.String(string(requestIDKey), requestID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", writer.status),
			zap.Duration("duration", time.Since(startedAt)),
		)
	})
}
