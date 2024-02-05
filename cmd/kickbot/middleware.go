package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"

	"github.com/slack-go/slack"
)

func SlackVerifyMiddleware(signingSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			verifier, err := slack.NewSecretsVerifier(r.Header, signingSecret)
			if err != nil {
				slog.Warn("Failed to create verifier", "error", err.Error())
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				slog.Warn("Failed to read request body", "error", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if _, err := verifier.Write(bodyBytes); err != nil {
				slog.Warn("failed to write body to verifier", "error", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if err := verifier.Ensure(); err != nil {
				slog.Warn("Message verification failed", "message", string(bodyBytes), "sender_ip", r.RemoteAddr)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Reassign the body for further processing in the next handlers
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			next.ServeHTTP(w, r)
		})
	}
}
