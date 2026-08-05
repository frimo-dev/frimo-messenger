package http

import (
	"net/http"
)

type healthResponse struct {
	Status string `json:"status"`
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}
