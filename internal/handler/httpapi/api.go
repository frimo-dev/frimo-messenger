package httpapi

import (
	"net/http"

	"github.com/frimo-dev/frimo-messenger/internal/service/emailverification"
	"github.com/frimo-dev/frimo-messenger/internal/service/user"
	"go.uber.org/zap"
)

type API struct {
	mux *http.ServeMux

	userService              *user.Service
	emailVerificationService *emailverification.Service

	logger *zap.Logger
}

func New(logger *zap.Logger, userService *user.Service, emailVerificationService *emailverification.Service) *API {
	api := &API{
		mux:                      http.NewServeMux(),
		userService:              userService,
		emailVerificationService: emailVerificationService,
		logger:                   logger,
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
}
