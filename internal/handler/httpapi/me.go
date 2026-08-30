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
	CreatedAt time.Time `json:"created_at"`
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.respondError(r.Context(), w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	us, err := a.userService.GetUserByID(r.Context(), userID)
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
		CreatedAt: us.CreatedAt,
	})
}
