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

func (gm *GameManager) CreateGame(channel SlackChannel, player string, gameType GameType) {

	// Ensure only one game per channel
	gm.mu.Lock()
	if _, exists := gm.games[channel]; exists {
		gm.mu.Unlock()
		gm.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Eine runde wird bereits vorbereitet!", false))
		return
	}

	game := NewGame(gameType, player)
	gm.games[channel] = game
	gm.mu.Unlock()

	// Send an initiation message to Slack
	msg := NewGameInitiationMsg(player, gameType)
	_, ts, err := gm.apiClient.PostMessage(string(channel), msg)
	if err != nil {
		slog.Error("Failed to send message", "error", err)
		gm.mu.Lock()
		delete(gm.games, channel)
		gm.mu.Unlock()
		gm.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Ein Feheler ist aufgetreten!", false))
		return
	}

	game.mu.Lock()
	game.messageTs = ts
	game.mu.Unlock()

}

func (gm *GameManager) JoinGame(channel SlackChannel, player string) {
	var isGameComplete bool
	var gameCopy *Game
	var updateMsg slack.MsgOption

	gm.mu.Lock()
	{
		g, exists := gm.games[channel]
		if !exists {
			gm.mu.Unlock()
			gm.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Ein Fehler ist aufgetreten", false))
			return
		}
		if len(g.players) == g.quorum {
			gm.mu.Unlock()
			gm.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Das Spiel ist bereits voll.", false))
			return
		}
		// snapshot of current game state
		g.mu.Lock()
		gameCopy = &Game{
			players:   slices.Clone(g.players),
			quorum:    g.quorum,
			messageTs: g.messageTs,
			mu:        &sync.Mutex{},
		}

		g.players = append(g.players, player)
		isGameComplete = len(g.players) == g.quorum
		players := slices.Clone(g.players)
		q := g.quorum
		g.mu.Unlock()
		updateMsg = NewGameUpdateMsg(players, q)

		// If the game is complete, delete it from the map
		if isGameComplete {
			delete(gm.games, channel)
		}
	}
	gm.mu.Unlock()

	// TODO: Implement retry mechanism to be reslient against transient network errors
	_, _, _, err := gm.apiClient.UpdateMessage(string(channel), gameCopy.messageTs, updateMsg)
	if err != nil {
		// TODO: Implement thread safe rollback of the game state
		slog.Error("Failed to update game message", "error", err)
		gm.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Es gab ein technisches Problem beim Beitritt zum Spiel.", false))
		return
	}
}

func (gm *GameManager) LeaveGame(channel SlackChannel, player string) {

	var isLastPayer bool
	var game *Game
	var gameCopy *Game

	gm.mu.Lock()
	{
		g, exists := gm.games[channel]
		if !exists {
			gm.mu.Unlock()
			gm.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Ein Feheler ist aufgetreten", false))
			return
		}
		idx := slices.Index(g.players, player)
		if idx < 0 {
			gm.mu.Unlock()
			gm.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Du bist nicht in der aktuellen Runde.", false))
			return
		}

		// snapshot of current game state
		g.mu.Lock()
		gameCopy = &Game{
			players:   slices.Clone(g.players),
			quorum:    g.quorum,
			messageTs: g.messageTs,
			mu:        &sync.Mutex{},
		}

		g.players = append(g.players[:idx], g.players[idx+1:]...)
		isLastPayer = len(g.players) == 0
		g.mu.Unlock()

		game = g
		if isLastPayer {
			delete(gm.games, channel)
		}

	}
	gm.mu.Unlock()

	if isLastPayer {
		_, _, err := gm.apiClient.DeleteMessage(string(channel), gameCopy.messageTs)
		if err != nil {
			slog.Error("Failed to delete game message", "error", err)
		}
		return
	}
	game.mu.Lock()
	players := slices.Clone(game.players)
	q := game.quorum
	ts := game.messageTs
	game.mu.Unlock()

	updateMsg := NewGameUpdateMsg(players, q)
	_, _, _, err := gm.apiClient.UpdateMessage(string(channel), ts, updateMsg)
	if err != nil {
		slog.Error("Failed to update game message", "error", err)
	}

}
