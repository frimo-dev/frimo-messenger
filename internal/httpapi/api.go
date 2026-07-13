package httpapi

import (
	"net/http"

	"github.com/frimo-dev/frimo-messenger/internal/user"
)

type API struct {
	mux         *http.ServeMux
	userService *user.Service
}

func New(userService *user.Service) *API {
	api := &API{
		mux:         http.NewServeMux(),
		userService: userService,
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
}
