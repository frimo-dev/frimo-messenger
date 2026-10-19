package httpapi

import (
	"errors"
	"net/http"

	"github.com/frimo-dev/frimo-messenger/internal/service/user"
	"go.uber.org/zap"
)

type publicUserResponse struct {
	ID       string  `json:"id"`
	Username string  `json:"username"`
	Name     string  `json:"name"`
	Surname  *string `json:"surname"`
}

func (a *API) getUserByUsername(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	publicUser, err := a.userService.GetPublicUserByUsername(r.Context(), username)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrUserNotFound):
			a.respondError(r.Context(), w, http.StatusNotFound, "user_not_found", "user not found")
		default:
			a.logger.Error("failed to get user by username", zap.Error(err))

			a.respondError(r.Context(), w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}

	a.respondJSON(r.Context(), w, http.StatusOK,
		publicUserResponse{
			ID:       publicUser.ID.String(),
			Username: publicUser.Username,
			Name:     publicUser.Name,
			Surname:  publicUser.Surname,
		},
	)
}
