package httpapi

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/zap"
)

type loginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	DeviceName string `json:"device_name"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest

	if err := json.UnmarshalRead(r.Body, &request, json.RejectUnknownMembers(true)); err != nil {
		a.respondError(r.Context(), w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	ip, err := clientIP(r)
	if err != nil {
		a.logger.Warn("failed to determine client ip", zap.Error(err), zap.String("request_id", requestIDFromContext(r.Context())))
	}

	loginResult, err := a.authService.Login(r.Context(), auth.LoginInput{
		Email:      request.Email,
		Password:   request.Password,
		DeviceName: request.DeviceName,
		IP:         ip,
	})
	if err != nil {
		if !errors.Is(err, auth.ErrAccessTokenNotStored) {
			var validationErr *auth.ValidationError

			switch {
			case errors.As(err, &validationErr):
				a.respondError(r.Context(), w, http.StatusBadRequest, validationErr.Code, validationErr.Message)
			case errors.Is(err, auth.ErrInvalidCredentials):
				a.respondError(r.Context(), w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
			case errors.Is(err, auth.ErrEmailNotVerified):
				a.respondError(r.Context(), w, http.StatusForbidden, "email_not_verified", "email not verified")
			default:
				a.logger.Error(
					"failed to login user",
					zap.Error(err),
					zap.String("request_id", requestIDFromContext(r.Context())),
				)
				a.respondError(r.Context(), w, http.StatusInternalServerError, "internal_error", "internal server error")
			}

			return
		}

		a.logger.Error("failed to store access token", zap.Error(err), zap.String("request_id", requestIDFromContext(r.Context())))
	}

	a.logger.Info("user session created", zap.String("request_id", requestIDFromContext(r.Context())))

	a.respondJSON(r.Context(), w, http.StatusOK, loginResponse{AccessToken: loginResult.AccessToken, RefreshToken: loginResult.RefreshToken})
}

func clientIP(r *http.Request) (netip.Addr, error) {
	if rawIP := r.Header.Get("X-Real-IP"); rawIP != "" {
		ip, err := netip.ParseAddr(rawIP)
		if err != nil {
			return netip.Addr{}, fmt.Errorf("parse X-Real-IP: %w", err)
		}

		return ip, nil
	}

	addrPort, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("parse remote addr: %w", err)
	}

	return addrPort.Addr(), nil
}
