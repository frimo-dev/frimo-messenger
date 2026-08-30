package httpapi

import (
	"context"
	"net/http"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
	"github.com/frimo-dev/frimo-messenger/internal/service/user"
	"go.uber.org/zap"
)

type AuthService interface {
	Login(ctx context.Context, email string, password string) (string, error)
	Register(ctx context.Context, input auth.RegistrationInput) (auth.User, error)
	ConfirmEmail(ctx context.Context, rawToken string) error
	ResendVerification(ctx context.Context, email string) error
}

type UserService interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (user.User, error)
}

type API struct {
	mux *http.ServeMux

	logger              *zap.Logger
	accessTokenVerifier AccessTokenVerifier

	authService AuthService
	userService UserService
}

func New(logger *zap.Logger, accessTokenVerifier AccessTokenVerifier, authService AuthService, userService UserService) *API {
	api := &API{
		mux:                 http.NewServeMux(),
		logger:              logger,
		accessTokenVerifier: accessTokenVerifier,
		authService:         authService,
		userService:         userService,
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
	a.mux.HandleFunc("POST /auth/login", a.login)

	a.mux.Handle("GET /me", AuthenticationMiddleware(a.logger, a.accessTokenVerifier, http.HandlerFunc(a.me)))
}
