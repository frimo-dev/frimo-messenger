package httpapi

import (
	"net/http"

	"github.com/frimo-dev/frimo-messenger/internal/emailverification"
	"github.com/frimo-dev/frimo-messenger/internal/user"
)

type API struct {
	mux                      *http.ServeMux
	userService              *user.Service
	emailVerificationService *emailverification.Service
}

func New(userService *user.Service, emailVerificationService *emailverification.Service) *API {
	api := &API{
		mux:                      http.NewServeMux(),
		userService:              userService,
		emailVerificationService: emailVerificationService,
	}

	api.registerRoutes()

	return api
}

func (a *API) Handler() http.Handler {
	return a.mux
}

func (a *API) registerRoutes() {
	a.mux.HandleFunc("GET /health", a.health)
	a.mux.HandleFunc("POST /auth/register", a.registerUser)
	// TODO: формально GET здесь не подходит, так как меняется состояние при подтверждении email
	a.mux.HandleFunc("GET /auth/verify-email", a.verifyEmail)
}
