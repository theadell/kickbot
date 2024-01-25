package main

import (
	"log/slog"
	"sync"

	"github.com/slack-go/slack"
)

type GameState int

const (
	Idle          GameState = iota // Bot is idle, waiting for a new game to start
	GameFormation                  // A game is being formed
)

const (
	CMD_START_ROUND     string = "/kicker"    // Start a standard game
	CMD_START_1V1_ROUND        = "/kicker1v1" // Start 1v1 duel game
	ACTION_JOIN_ROUND          = "GAME_JOIN"  // Join a game
	ACTION_LEAVE_ROUND         = "GAME_LEAVE" // Leave a game in "formation" state after joining
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
		state:     Idle,
		players:   make([]string, 0),
		mu:        &sync.Mutex{},
	}
}

func (bot *GameBot) InitiateGame(command slack.SlashCommand) {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	switch bot.state {
	case Idle:

		var duel bool
		var quorum int

		if command.Command == CMD_START_ROUND {
			quorum = 4 // Standard game
		} else if command.Command == CMD_START_1V1_ROUND {
			duel = true
			quorum = 2 // 1v1 game
		}
		bot.setState(GameFormation, quorum, []string{command.UserID})
		msg := NewGameInitiationMsg(command.UserID, duel)
		_, ts, err := bot.apiClient.PostMessage(bot.channelID, msg)
		if err != nil {
			slog.Error("Failed to send message", "error", err)
			bot.resetState()
			return
		}
		bot.messageTs = ts
		slog.Info("Game initiation message was sent successfully", "timestamp", ts, "channel", bot.channelID)

	case GameFormation:
		bot.apiClient.PostEphemeral(bot.channelID, command.UserID, slack.MsgOptionText("Eine runde wird bereits vorbereitet!", false))
	}
}

func (bot *GameBot) JoinGame(userID string) {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	if bot.state != GameFormation {
		bot.apiClient.PostEphemeral(bot.channelID, userID, slack.MsgOptionText("Aktuell wird keine Runde vorbereitet. Starte gerne ein neues Spiel.", false))
		return
	}

	for _, player := range bot.players {
		if player == userID {
			bot.apiClient.PostEphemeral(bot.channelID, userID, slack.MsgOptionText("Du bist bereits im Spiel!", false))
			return
		}
	}

	bot.players = append(bot.players, userID)

	updateMsg := NewGameUpdateMsg(bot.players, bot.quorum)
	_, _, _, err := bot.apiClient.UpdateMessage(bot.channelID, bot.messageTs, updateMsg)
	if err != nil {
		// Rollback: Remove the last added player as update failed
		bot.players = bot.players[:len(bot.players)-1]
		slog.Error("Failed to update game message", "error", err)
		bot.apiClient.PostEphemeral(bot.channelID, userID, slack.MsgOptionText("Es gab ein technisches Problem beim Beitritt zum Spiel.", false))
		return
	}

	if len(bot.players) == bot.quorum {
		bot.resetState()
	}
}

func (bot *GameBot) LeaveGame(userID string) {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	if bot.state != GameFormation {
		bot.apiClient.PostEphemeral(bot.channelID, userID, slack.MsgOptionText("Du bist aktuell in keiner Spielrunde.", false))
		return
	}

	// Find the player's index
	playerIndex := -1
	for i, player := range bot.players {
		if player == userID {
			playerIndex = i
			break
		}
	}

	if playerIndex == -1 {
		bot.apiClient.PostEphemeral(bot.channelID, userID, slack.MsgOptionText("Du bist nicht in der aktuellen Runde.", false))
		return
	}

	// remove the player
	bot.players = append(bot.players[:playerIndex], bot.players[playerIndex+1:]...)

	// If no players are left, delete the message and reset the state
	if len(bot.players) == 0 {
		_, _, err := bot.apiClient.DeleteMessage(bot.channelID, bot.messageTs)
		if err != nil {
			slog.Error("Failed to delete game message", "error", err)
			bot.apiClient.PostEphemeral(bot.channelID, userID, slack.MsgOptionText("Es gab ein technisches Problem beim Abbrechen des Spiels.", false))
		} else {
			bot.apiClient.PostEphemeral(bot.channelID, userID, slack.MsgOptionText("Das Spiel wurde abgebrochen.", false))
		}
		bot.resetState()
		return
	}

	// Update the game message with remaining players
	updateMsg := NewGameUpdateMsg(bot.players, bot.quorum)
	_, _, _, err := bot.apiClient.UpdateMessage(bot.channelID, bot.messageTs, updateMsg)
	if err != nil {
		// Rollback: Add the player back as update failed
		bot.players = append(bot.players[:playerIndex], append([]string{userID}, bot.players[playerIndex:]...)...)
		slog.Error("Failed to update game message", "error", err)
		bot.apiClient.PostEphemeral(bot.channelID, userID, slack.MsgOptionText("Es gab ein technisches Problem. Dein Austritt wurde nicht verarbeitet.", false))
		return
	}
}

func (bot *GameBot) setState(state GameState, quorum int, players []string) {
	bot.state = state
	bot.quorum = quorum
	bot.players = players
}

func (bot *GameBot) resetState() {
	bot.state = Idle
	bot.quorum = 0
	bot.players = []string{}
}
