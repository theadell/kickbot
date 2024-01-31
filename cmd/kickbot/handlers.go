package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/slack-go/slack"
)

func handleSlackCommand(gm *GameManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd, err := slack.SlashCommandParse(r)
		if err != nil {
			slog.Error("Failed to parse slash command", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		switch cmd.Command {
		case CMD_START_ROUND:
			gm.CreateGame(SlackChannel(cmd.ChannelID), cmd.UserID, GameTypeTwoVsTwo)
		case CMD_START_1V1_ROUND:
			gm.CreateGame(SlackChannel(cmd.ChannelID), cmd.UserID, GameTypeOneVsOne)
		default:
			slog.Warn("Recieved an invalid command", "command", cmd.Command, "sender", r.RemoteAddr)
			w.WriteHeader(http.StatusBadRequest)
			return

		}
		w.WriteHeader(http.StatusOK)
	}
}

func handleSlackEvent(gm *GameManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var interactionCallback slack.InteractionCallback

		if err := json.Unmarshal([]byte(r.FormValue("payload")), &interactionCallback); err != nil {
			slog.Error("Failed to decode interaction body", "error", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		actions := interactionCallback.ActionCallback.BlockActions
		if len(actions) < 1 {
			slog.Error("Invalid or empty block action callback", "payload", interactionCallback)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		channel := SlackChannel(interactionCallback.Channel.ID)
		player := interactionCallback.User.ID
		switch actions[0].ActionID {
		case ACTION_JOIN_ROUND:
			gm.JoinGame(channel, player)
		case ACTION_LEAVE_ROUND:
			gm.LeaveGame(channel, player)
		default:
			slog.Warn("Invalid Action Id", "actionId", interactionCallback.ActionID, "sender", r.RemoteAddr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
