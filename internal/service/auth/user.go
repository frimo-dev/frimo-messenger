package auth

import (
	"time"
	"uuid"
)

type User struct {
	ID        uuid.UUID
	Email     string
	CreatedAt time.Time
}

type RegistrationInput struct {
	Email    string
	Password string
}
