package user

import "errors"

var ErrEmailAlreadyExists = errors.New("email already exists")

type ValidationError struct {
	Code    string
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}
