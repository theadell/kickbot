package main

import (
	"log/slog"
	"sync"

	"github.com/slack-go/slack"
)

type GameState int

const (
	WaitState GameState = iota
	SetupState
)

type GameBot struct {
	apiClient *slack.Client
	state     GameState
	players   []string
	quorum    int
	messageTs string
	channelID string
	mu        *sync.Mutex
}

func NewGameBot(apiToken, channelID string) *GameBot {
	api := slack.New(apiToken)
	return &GameBot{
		apiClient: api,
		channelID: channelID,
		state:     WaitState,
		players:   make([]string, 0),
		mu:        &sync.Mutex{},
	}
}

func (bot *GameBot) HandleCommand(command slack.SlashCommand) {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	switch bot.state {
	case WaitState:
		if command.Command == "/kicker" {
			bot.quorum = 4 // Standard game
			bot.startSetup(command.UserID)
		} else if command.Command == "/kicker1v1" {
			bot.quorum = 2 // 1v1 game
			bot.startSetup(command.UserID)
		}
	case SetupState:
		bot.sendEphemeral(command.UserID, command.ChannelID, "A game is already being set up.")
	}
}
func (bot *GameBot) startSetup(userID string) {
	bot.state = SetupState
	bot.players = []string{userID}

	mention := "<@" + userID + ">"
	attachment := slack.Attachment{
		Text:       mention + " hat eine Kickerrunde gestartet! Wer ist dabei?",
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
	channel, timestamp, err := bot.apiClient.PostMessage(bot.channelID, message)
	if err != nil {
		slog.Error("Failed to send message", "error", err)
		bot.state = WaitState
		bot.players = make([]string, 0)
		return
	}
	bot.messageTs = timestamp
	slog.Info("Message send successfully", "channel", channel, "timestamp", timestamp)

}

func (bot *GameBot) HandleJoin(userID string) {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	if bot.state != SetupState {
		return
	}
}

func (bot *GameBot) sendEphemeral(userID, channelID, text string) {
	bot.apiClient.PostEphemeral(channelID, userID, slack.MsgOptionText(text, false))
}
