package httpapi

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
)

type ValidationError struct {
	Field string
	Code  string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed: field=%s code=%s", e.Field, e.Code)
}

type Validator interface {
	Validate() error
}

func decodeJSON[T Validator](r *http.Request, destination T) error {
	if err := json.UnmarshalRead(r.Body, destination, json.RejectUnknownMembers(true)); err != nil {
		return err
	}

	return destination.Validate()
}
