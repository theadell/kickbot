package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSlackRequestvVrificationMiddleWare(t *testing.T) {

	secret := "super-secret-signature-string"
	invalidSecret := "invalid-super-secret-signature-string"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	middleware := SlackVerifyMiddleware(secret)

	bodyData := []byte("message from slack")

	testCases := []struct {
		name         string
		request      *http.Request
		expectedCode int
	}{
		{
			name:         "Valid Signature",
			request:      signSlackHttpRequest(httptest.NewRequest(http.MethodPost, "/commands", strings.NewReader(string(bodyData))), bodyData, secret),
			expectedCode: http.StatusOK,
		},
		{
			name:         "Invalid Signature",
			request:      signSlackHttpRequest(httptest.NewRequest(http.MethodPost, "/commands", strings.NewReader(string(bodyData))), bodyData, invalidSecret),
			expectedCode: http.StatusUnauthorized,
		},
		{
			name:         "No Signature",
			request:      httptest.NewRequest(http.MethodPost, "/commands", strings.NewReader(string(bodyData))),
			expectedCode: http.StatusUnauthorized,
		},
		{
			name:         "Error Reading Body",
			request:      signSlackHttpRequest(httptest.NewRequest(http.MethodPost, "/commands", &errorReader{}), bodyData, secret),
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			middleware(handler).ServeHTTP(recorder, tc.request)
			if recorder.Result().StatusCode != tc.expectedCode {
				t.Errorf("TestSlackRequestVerificationMiddleware - %s: Expected status %d but found %d", tc.name, tc.expectedCode, recorder.Result().StatusCode)
			}
		})
	}

}

func signSlackHttpRequest(r *http.Request, data []byte, secret string) *http.Request {
	timestammp := time.Now().Unix()

	basestring := "v0" + ":" + fmt.Sprint(timestammp) + ":" + string(data)

	hmac := hmac.New(sha256.New, []byte(secret))
	hmac.Write([]byte(basestring))
	signature := hex.EncodeToString(hmac.Sum(nil))

	r.Header.Set("x-slack-request-timestamp", fmt.Sprint(timestammp))
	r.Header.Set("x-slack-signature", signature)

	return r
}

// errorReader simulates a read error on request body
type errorReader struct{}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

func (er *errorReader) Close() error {
	return nil
}
