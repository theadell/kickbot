package main

import (
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
			var gameOptions = parseFlags(cmd.Text)
			gm.CreateGame(SlackChannel(cmd.ChannelID), cmd.UserID, gameOptions)
		case CMD_CANCEL_ROUND:
			gm.CancelGame(SlackChannel(cmd.ChannelID), cmd.UserID)
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
			slog.Warn("Failed to decode interaction body", "error", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		actions := interactionCallback.ActionCallback.BlockActions
		if len(actions) < 1 {
			slog.Warn("Invalid or empty block action callback", "payload", interactionCallback)
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

func parseFlags(params string) GameOpts {
	var timeout time.Duration
	var duel bool

	flagSet := flag.NewFlagSet("gameParameters", flag.ContinueOnError)
	flagSet.DurationVar(&timeout, "timeout", time.Minute*30, "")
	flagSet.DurationVar(&timeout, "t", timeout, "")
	flagSet.BoolVar(&duel, "duel", false, "")
	flagSet.BoolVar(&duel, "d", duel, "")
	err := flagSet.Parse(strings.Fields(params))

	if err != nil {
		slog.Error("error parsing flags in game request", err)
	}

	var gameType GameType
	if duel {
		gameType = GameTypeOneVsOne
	} else {
		gameType = GameTypeTwoVsTwo
	}

	return GameOpts{
		timeout:  timeout,
		gameType: gameType,
	}
}

type GameOpts struct {
	timeout  time.Duration
	gameType GameType
}
