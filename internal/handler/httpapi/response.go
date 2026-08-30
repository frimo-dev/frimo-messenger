package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (a *API) respondError(ctx context.Context, w http.ResponseWriter, status int, code string, message string) {
	if err := writeError(w, status, code, message); err != nil {
		a.logger.Error(
			"failed to write error response",
			zap.Error(err),
			zap.String("request_id", requestIDFromContext(ctx)),
			zap.Int("status", status),
			zap.String("code", code),
		)
	}
}

func (a *API) respondJSON(ctx context.Context, w http.ResponseWriter, status int, value any) {
	if err := writeJSON(w, status, value); err != nil {
		a.logger.Error(
			"failed to write JSON response",
			zap.Error(err),
			zap.String("request_id", requestIDFromContext(ctx)),
			zap.Int("status", status),
		)
	}
}

func writeError(w http.ResponseWriter, status int, code string, message string) error {
	return writeJSON(w, status, errorResponse{Error: errorBody{
		Code:    code,
		Message: message,
	},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed marshal HTTP response to JSON: %w", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if _, err := w.Write(append(body, '\n')); err != nil {
		return fmt.Errorf("failed to write HTTP response: %w", err)
	}
	return nil
}
