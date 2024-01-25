package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var bot *GameBot

func main() {
	token := os.Getenv("KICKBOT_TOKEN")
	channelID := os.Getenv("KICKBOT_CHANNELID")
	signingSecret := os.Getenv("KICKBOT_SIGNING_SECRET")
	bot = NewGameBot(token, channelID)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.RealIP)
	r.Use(SlackVerifyMiddleware(signingSecret))
	r.HandleFunc("/commands", handleSlackCommand)
	r.HandleFunc("/events", handleSlackEvent)
	slog.Info("Server running")
	http.ListenAndServe("127.0.0.1:3000", r)
}
