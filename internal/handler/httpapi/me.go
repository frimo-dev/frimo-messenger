package httpapi

import (
	"errors"
	"net/http"
	"time"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/service/user"
)

type meResponse struct {
	UserID    uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Surname   string    `json:"surname"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	identity, ok := identityFromContext(r.Context())
	if !ok {
		a.respondError(r.Context(), w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	us, err := a.userService.GetUserByID(r.Context(), identity.UserID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			a.respondError(r.Context(), w, http.StatusUnauthorized, "authentication_error", "authentication failed")
		} else {
			a.respondError(r.Context(), w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}

	a.respondJSON(r.Context(), w, http.StatusOK, meResponse{
		UserID:    us.ID,
		Email:     us.Email,
		Username:  us.Username,
		Name:      us.Name,
		Surname:   us.Surname,
		CreatedAt: us.CreatedAt,
	})
}
