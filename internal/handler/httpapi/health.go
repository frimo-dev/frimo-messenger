package httpapi

import (
	"net/http"
)

type healthResponse struct {
	Status string `json:"status"`
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	a.respondJSON(r.Context(), w, http.StatusOK, healthResponse{Status: "ok"})
}
