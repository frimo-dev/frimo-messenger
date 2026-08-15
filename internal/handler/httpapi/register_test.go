package httpapi_test

//func TestRegisterUser(t *testing.T) {
//	api := newTestAPI()
//
//	body := []byte(`{
//		"email": "misha@example.com",
//		"password": "long-secret-password"
//	}`)
//
//	request := httptest.NewRequest(
//		httpapi.MethodPost,
//		"/auth/register",
//		bytes.NewReader(body),
//	)
//
//	request.Header.Set("Content-Type", "application/json")
//
//	recorder := httptest.NewRecorder()
//
//	api.Handler().ServeHTTP(recorder, request)
//
//	response := recorder.Result()
//	defer response.Body.Close()
//
//	if response.StatusCode != httpapi.StatusCreated {
//		t.Fatalf(
//			"expected status %d, got %d",
//			httpapi.StatusCreated,
//			response.StatusCode,
//		)
//	}
//
//	var responseBody registerResponse
//
//	if err := json.NewDecoder(response.Body).Decode(&responseBody); err != nil {
//		t.Fatalf("decode response: %v", err)
//	}
//
//	if responseBody.Email != "misha@example.com" {
//		t.Fatalf(
//			"expected email %q, got %q",
//			"misha@example.com",
//			responseBody.Email,
//		)
//	}
//
//	if responseBody.ID == "" {
//		t.Fatal("expected non-empty user ID")
//	}
//}

//func TestRegisterUserRejectsDuplicateEmail(t *testing.T) {
//	api := newTestAPI()
//
//	body := []byte(`{
//		"email": "misha@example.com",
//		"password": "long-secret-password"
//	}`)
//
//	firstRequest := httptest.NewRequest(
//		httpapi.MethodPost,
//		"/auth/register",
//		bytes.NewReader(body),
//	)
//
//	firstRecorder := httptest.NewRecorder()
//	api.Handler().ServeHTTP(firstRecorder, firstRequest)
//
//	secondRequest := httptest.NewRequest(
//		httpapi.MethodPost,
//		"/auth/register",
//		bytes.NewReader(body),
//	)
//
//	secondRecorder := httptest.NewRecorder()
//	api.Handler().ServeHTTP(secondRecorder, secondRequest)
//
//	if secondRecorder.Code != httpapi.StatusConflict {
//		t.Fatalf(
//			"expected status %d, got %d",
//			httpapi.StatusConflict,
//			secondRecorder.Code,
//		)
//	}
//}
