package httpapi

import (
	"errors"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/security/token"
	"go.uber.org/zap"
)

type AccessTokenVerifier interface {
	Verify(rawToken string) (uuid.UUID, error)
}

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
				zap.String("request_id", requestIDFromContext(r.Context())),
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
			zap.String("request_id", requestID.String()),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", writer.status),
			zap.Duration("duration", time.Since(startedAt)),
		)
	})
}

func AuthenticationMiddleware(logger *zap.Logger, verifier AccessTokenVerifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			err := writeError(w, http.StatusUnauthorized, "authentication_error", "authentication failed")
			if err != nil {
				logger.Error(
					"failed to write error response",
					zap.Error(err),
					zap.String("request_id", requestIDFromContext(r.Context())),
				)
			}
			return
		}

		rawToken := parts[1]

		userID, err := verifier.Verify(rawToken)
		if err != nil {
			switch {
			case errors.Is(err, token.ErrInvalidToken):
				logger.Debug(
					"invalid token", zap.Error(err),
					zap.String("request_id", requestIDFromContext(r.Context())),
				)
			case errors.Is(err, token.ErrTokenExpired):
				logger.Debug("expired token", zap.String("request_id", requestIDFromContext(r.Context())))
			default:
				logger.Error("failed to verify token", zap.Error(err), zap.String("request_id", requestIDFromContext(r.Context())))
			}

			err := writeError(w, http.StatusUnauthorized, "authentication_error", "authentication failed")
			if err != nil {
				logger.Error(
					"failed to write error response",
					zap.Error(err),
					zap.String("request_id", requestIDFromContext(r.Context())),
				)
			}
			return
		}

		ctx := withUserID(r.Context(), userID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
