package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/slack-go/slack"
)

func (bot *GameBot) handleSlackCommand(w http.ResponseWriter, r *http.Request) {
	// verify request
	verifier, err := slack.NewSecretsVerifier(r.Header, signingSecret)
	if err != nil {
		slog.Error("failed to create verifier", "error", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Failed to read request body", "error", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.Body.Close()
	if _, err := verifier.Write(bodyBytes); err != nil {
		slog.Error("Failed to wrote request body to verifier", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := verifier.Ensure(); err != nil {
		slog.Info("Invalid request body", "error", err.Error())
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// parse command
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	cmd, err := slack.SlashCommandParse(r)
	if err != nil {
		slog.Error("Failed to parse slash command", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// handle command
	switch cmd.Command {
	case "/kicker":
		bot.startSetup(cmd.UserID)
	}
}

func (bot *GameBot) handleSlackEvent(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.Body.Close()
	sv, err := slack.NewSecretsVerifier(r.Header, signingSecret)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := sv.Write(bodyBytes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := sv.Ensure(); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var payload slack.InteractionCallback
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Error("Failed to decode interaction body", "error", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	bot.sendEphemeral(payload.User.ID, bot.channelID, "Got it, you are in!")

}
