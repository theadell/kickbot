package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/slack-go/slack"
)

func handleSlackCommand(w http.ResponseWriter, r *http.Request) {

	cmd, err := slack.SlashCommandParse(r)
	if err != nil {
		slog.Error("Failed to parse slash command", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch cmd.Command {
	case "/kicker":
		bot.startSetup(cmd.UserID)
	}
}

func handleSlackEvent(w http.ResponseWriter, r *http.Request) {
	var payload slack.InteractionCallback

	if err := json.Unmarshal([]byte(r.FormValue("payload")), &payload); err != nil {
		slog.Error("Failed to decode interaction body", "error", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	bot.sendEphemeral(payload.User.ID, bot.channelID, "Got it, you are in!")

}
