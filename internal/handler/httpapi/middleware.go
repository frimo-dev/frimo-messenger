package httpapi

import (
	"net/http"
	"runtime/debug"
	"time"
	"uuid"

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

func RecoveryMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}

			logger.Error(
				"http handler panic",
				zap.String(string(requestIDKey), requestIDFromContext(r.Context())),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Any("panic", recovered),
				zap.ByteString("stack", debug.Stack()),
			)

			err := writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			if err != nil {
				logger.Error("failed to write error response", zap.Error(err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func RequestLoggingMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		requestID := uuid.New()

		ctx := withRequestID(r.Context(), requestID.String())
		r = r.WithContext(ctx)

		writer := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(writer, r)

		logger.Info(
			"http request completed",
			zap.String(string(requestIDKey), requestID.String()),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", writer.status),
			zap.Duration("duration", time.Since(startedAt)),
		)
	})
}
