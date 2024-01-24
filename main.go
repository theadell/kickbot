package main

import (
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/slack-go/slack"
)

var token, channelID string
var apiClient *slack.Client

func main() {
	token = os.Getenv("KICKBOT_TOKEN")
	channelID = os.Getenv("KICKBOT_CHANNELID")
	apiClient = slack.New(token)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.HandleFunc("/commands", handleSlackCommands)
	slog.Info("Server running")
	http.ListenAndServe(":3000", nil)

}

func handleSlackCommands(w http.ResponseWriter, r *http.Request) {
	verifier, err := slack.NewSecretsVerifier(r.Header, os.Getenv("KICKBOT_SIGNING_SECRET"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Debug("Failed to read request body", "error", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := verifier.Write(body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := verifier.Ensure(); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	cmd, err := slack.SlashCommandParse(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch cmd.Command {
	case "/matchup":
		sendLineUpMsg()
	}
}

func sendLineUpMsg() {
	attachment := slack.Attachment{
		Text:       "New foosball game! Who's in?",
		CallbackID: "game_join",
		Color:      "#4af030",
		Actions: []slack.AttachmentAction{
			{
				Name:  "join",
				Text:  "Join",
				Type:  "button",
				Value: "join",
			},
		},
	}
	message := slack.MsgOptionAttachments(attachment)
	channel, timestamp, err := apiClient.PostMessage(channelID, message)
	if err != nil {
		slog.Error("Failed to send message", "error", err)
	}
	slog.Info("Message send successfully", "channel", channel, "timestamp", timestamp)
}
