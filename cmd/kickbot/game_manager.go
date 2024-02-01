package main

import (
	"log/slog"
	"slices"
	"sync"

	"github.com/slack-go/slack"
)

const (
	CMD_START_ROUND     string = "/kicker"    // Start a standard game
	CMD_START_1V1_ROUND        = "/kicker1v1" // Start 1v1 duel game
	ACTION_JOIN_ROUND          = "GAME_JOIN"  // Join a game
	ACTION_LEAVE_ROUND         = "GAME_LEAVE" // Leave a game in "formation" state after joining
)

type SlackChannel string

type GameManager struct {
	apiClient SlackClient
	games     map[SlackChannel]*Game
	mu        sync.Mutex
}

func (gameMgr *GameManager) CreateGame(channel SlackChannel, player string, gameType GameType) {

	game := NewGame(gameType, player)

	if !gameMgr.setGameIfNotExists(channel, game) {
		gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Eine runde wird bereits vorbereitet!", false))
		return
	}

	msg := NewGameInitiationMsg(player, gameType)
	_, ts, err := gameMgr.apiClient.PostMessage(string(channel), msg)
	if err != nil {
		slog.Error("Failed to send message", "error", err)
		gameMgr.deleteGame(channel)
		gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Ein Feheler ist aufgetreten!", false))
		return
	}

	game.mu.Lock()
	game.messageTs = ts
	game.mu.Unlock()

}

func (gameMgr *GameManager) JoinGame(channel SlackChannel, player string) {
	var updateMsg slack.MsgOption
	var gameMsgTS string
	var isGameComplete bool

	game, exists := gameMgr.getGame(channel)
	if !exists {
		gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Ein Fehler ist aufgetreten", false))
		return
	}

	// lock game to prevent data races on concurrent joins & leaves
	game.mu.Lock()
	{
		// check if game is already full
		isGameComplete = len(game.players) == game.quorum
		if isGameComplete {
			game.mu.Unlock()
			gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Das Spiel ist bereits voll.", false))
			return
		}

		game.players = append(game.players, player)

		// check if game has become full after the player joined
		isGameComplete = len(game.players) == game.quorum
		gameMsgTS = game.messageTs
		updateMsg = NewGameUpdateMsg(game.players, game.quorum)
	}
	game.mu.Unlock()

	if isGameComplete {
		gameMgr.deleteGame(channel)
	}

	// TODO: Implement retry mechanism to be reslient against transient network errors
	_, _, _, err := gameMgr.apiClient.UpdateMessage(string(channel), gameMsgTS, updateMsg)
	if err != nil {
		// TODO: Implement thread safe rollback of the game state
		slog.Error("Failed to update game message", "error", err)
		gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Es gab ein technisches Problem beim Beitritt zum Spiel.", false))
		return
	}
}

func (gameMgr *GameManager) LeaveGame(channel SlackChannel, player string) {

	game, exists := gameMgr.getGame(channel)
	if !exists {
		gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Ein Fehler ist aufgetreten", false))
		return
	}

	var updateMsg slack.MsgOption
	var isLastPlayer bool
	var gameMsgTS string

	game.mu.Lock()
	{
		idx := slices.Index(game.players, player)
		if idx < 0 {
			game.mu.Unlock()
			gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Du bist nicht in der aktuellen Runde.", false))
			return
		}
		// remove player from game
		game.players = append(game.players[:idx], game.players[idx+1:]...)
		isLastPlayer = len(game.players) == 0
		updateMsg = NewGameUpdateMsg(game.players, game.quorum)
		gameMsgTS = game.messageTs
	}
	game.mu.Unlock()

	if isLastPlayer {
		gameMgr.deleteGame(channel)
		_, _, err := gameMgr.apiClient.DeleteMessage(string(channel), game.messageTs)
		if err != nil {
			slog.Error("Failed to delete game message", "error", err)
		}
		return
	}
	_, _, _, err := gameMgr.apiClient.UpdateMessage(string(channel), gameMsgTS, updateMsg)
	if err != nil {
		slog.Error("Failed to update game message", "error", err)
	}
}

func (gameMgr *GameManager) getGame(channel SlackChannel) (*Game, bool) {
	gameMgr.mu.Lock()
	defer gameMgr.mu.Unlock()

	game, exists := gameMgr.games[channel]
	return game, exists
}
func (gameMgr *GameManager) setGame(channel SlackChannel, game *Game) {
	gameMgr.mu.Lock()
	gameMgr.games[channel] = game
	gameMgr.mu.Unlock()
}

func (gameMgr *GameManager) deleteGame(channel SlackChannel) {
	gameMgr.mu.Lock()
	defer gameMgr.mu.Unlock()

	if _, exists := gameMgr.games[channel]; exists {
		delete(gameMgr.games, channel)
	}
}

// setGameIfNotExists sets a new game for the specified channel only if there isn't already a game present.
// It returns true if the new game was set, or false if a game already exists for the channel.
func (gm *GameManager) setGameIfNotExists(channel SlackChannel, game *Game) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if _, exists := gm.games[channel]; exists {
		return false
	}

	gm.games[channel] = game
	return true
}
