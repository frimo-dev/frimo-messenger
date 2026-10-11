package httpapi_test

import (
	"encoding/json/v2"
	"net/http/httptest"
	"testing"
)

func assertErrorResponse(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
	wantMessage string,
) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			rec.Code,
			wantStatus,
			rec.Body.String(),
		)
	}

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf(
			"Content-Type = %q, want %q",
			contentType,
			"application/json",
		)
	}

	var response struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf(
			"unmarshal error response: %v; body = %s",
			err,
			rec.Body.String(),
		)
	}

	if response.Error.Code != wantCode {
		t.Errorf(
			"error code = %q, want %q",
			response.Error.Code,
			wantCode,
		)
	}

	if response.Error.Message != wantMessage {
		t.Errorf(
			"error message = %q, want %q",
			response.Error.Message,
			wantMessage,
		)
	}
}
