package httpapi

import "net/http"

type API struct {
	mux *http.ServeMux
}

func New() *API {
	api := &API{
		mux: http.NewServeMux(),
	}

	api.registerRoutes()

	return api
}

func (a *API) Handler() http.Handler {
	return a.mux
}

func (a *API) registerRoutes() {
	a.mux.HandleFunc("GET /health", a.health)
}
