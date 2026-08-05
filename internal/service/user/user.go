package user

import (
	"time"
)

type User struct {
	ID        string
	Email     string
	CreatedAt time.Time
}

type RegistrationInput struct {
	Email    string
	Password string
}
