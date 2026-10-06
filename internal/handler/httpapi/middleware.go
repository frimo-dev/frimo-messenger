package httpapi

import (
	"context"
	"errors"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/execution"
	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/zap"
)

type AccessTokenAuthenticator interface {
	Authenticate(ctx context.Context, rawAccessToken string) (auth.Identity, error)
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
				zap.String("request_id", execution.IDFromContext(r.Context())),
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

		ctx := execution.WithID(r.Context(), requestID.String())
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

func AuthenticationMiddleware(logger *zap.Logger, authenticator AccessTokenAuthenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			err := writeError(w, http.StatusUnauthorized, "authentication_error", "authentication failed")
			if err != nil {
				logger.Error(
					"failed to write error response",
					zap.Error(err),
					zap.String("request_id", execution.IDFromContext(r.Context())),
				)
			}
			return
		}

		rawAccessToken := parts[1]

		identity, err := authenticator.Authenticate(r.Context(), rawAccessToken)
		if err != nil {
			if errors.Is(err, auth.ErrAccessTokenInvalid) || errors.Is(err, auth.ErrSessionInactive) {
				err = writeError(w, http.StatusUnauthorized, "authentication_error", "authentication failed")
				if err != nil {
					logger.Error(
						"failed to write error response",
						zap.Error(err),
						zap.String("request_id", execution.IDFromContext(r.Context())),
					)
				}
				return
			}

			logger.Error(
				"failed to authenticate request",
				zap.String("request_id", execution.IDFromContext(r.Context())),
				zap.Error(err),
			)

			err = writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			if err != nil {
				logger.Error(
					"failed to write error response",
					zap.Error(err),
					zap.String("request_id", execution.IDFromContext(r.Context())),
				)
			}
			return
		}

		ctx := withIdentity(r.Context(), identity)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
