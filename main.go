package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/slack-go/slack"
)

var token, channelID, signingSecret string
var apiClient *slack.Client

func main() {
	token = os.Getenv("KICKBOT_TOKEN")
	channelID = os.Getenv("KICKBOT_CHANNELID")
	signingSecret = os.Getenv("KICKBOT_SIGNING_SECRET")
	gameBot := NewGameBot(token)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.HandleFunc("/commands", gameBot.handleSlackCommand)
	r.HandleFunc("/events", gameBot.handleSlackEvent)
	slog.Info("Server running")
	http.ListenAndServe("127.0.0.1:3000", r)
}
