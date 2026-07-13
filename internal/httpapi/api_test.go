package httpapi

import (
	"github.com/frimo-dev/frimo-messenger/internal/password"
	"github.com/frimo-dev/frimo-messenger/internal/user"
)

func newTestAPI() *API {
	repository := user.NewMemoryRepository()
	passwordHasher := password.NewArgon2Hasher()

	service := user.NewService(repository, passwordHasher)

	return New(service)
}
