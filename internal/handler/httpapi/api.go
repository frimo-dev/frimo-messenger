package httpapi

import (
	"context"
	"net/http"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"go.uber.org/zap"
)

type AuthService interface {
	Register(ctx context.Context, input auth.RegistrationInput) (auth.User, error)
	ConfirmEmail(ctx context.Context, rawToken string) error
	ResendVerification(ctx context.Context, email string) error
}

type API struct {
	mux *http.ServeMux

	authService AuthService

	logger *zap.Logger
}

func New(logger *zap.Logger, authService AuthService) *API {
	api := &API{
		mux:         http.NewServeMux(),
		authService: authService,
		logger:      logger,
	}

	api.registerRoutes()

	return api
}

func (a *API) Handler() http.Handler {
	var handler http.Handler = a.mux
	handler = RecoveryMiddleware(a.logger, handler)
	handler = RequestLoggingMiddleware(a.logger, handler)

	return handler
}

func (a *API) registerRoutes() {
	a.mux.HandleFunc("GET /health", a.health)

	a.mux.HandleFunc("POST /auth/register", a.registerUser)
	// TODO: формально GET здесь не подходит, так как меняется состояние при подтверждении email
	a.mux.HandleFunc("GET /auth/verify-email", a.verifyEmail)
	a.mux.HandleFunc("POST /auth/resend", a.resendVerificationToken)
}
