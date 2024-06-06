package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"
)

const (
	CMD_START_ROUND    string = "/kicker"    // Start a game
	ACTION_JOIN_ROUND         = "GAME_JOIN"  // Join a game
	ACTION_LEAVE_ROUND        = "GAME_LEAVE" // Leave a game in "formation" state after joining
)

type SlackChannel string

type GameManager struct {
	apiClient    SlackClient
	gameRequests map[SlackChannel]*GameRequest
	timeoutChan  chan SlackChannel
	mu           sync.Mutex
}

func NewGameManager(client SlackClient) *GameManager {
	gameMgr := &GameManager{
		apiClient:    client,
		gameRequests: make(map[SlackChannel]*GameRequest),
		mu:           sync.Mutex{},
		timeoutChan:  make(chan SlackChannel, 10),
	}
	go gameMgr.handleTimeouts()
	return gameMgr
}

// CreateGame initializes a new game request in the specified Slack channel. It posts a game request message to
// the channel, allowing users to join. it handles the game creation process triggered by a Slack slash command (/kicker).
// If a game is already being prepared in the channel, no new game is created and
// it notifies the user who attempted to start a new game.
// The game type (e.g., TwoVsTwo, OneVsOne) is specified in the call. (/kicker & /kicker1v1)
func (gameMgr *GameManager) CreateGame(channel SlackChannel, player string, gameOptions GameOpts) {

	gameReq := NewGameRequest(gameOptions.gameType, player)

	if !gameMgr.setGameRequestIfNotExists(channel, gameReq) {
		gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Eine runde wird bereits vorbereitet!", false))
		return
	}

	msg := NewGameRequestMsg(player, gameOptions.gameType)
	_, ts, err := gameMgr.apiClient.PostMessage(string(channel), msg)
	if err != nil {
		slog.Error("Failed to send message", "error", err)
		gameMgr.deleteGameRequest(channel)
		gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Ein Feheler ist aufgetreten!", false))
		return
	}

	gameReq.mu.Lock()
	gameReq.messageTs = ts
	ctx, cancel := context.WithCancel(context.Background())
	gameReq.timerCancelFunc = cancel
	gameReq.timer = time.AfterFunc(gameOptions.timeout, func() {
		select {
		case <-ctx.Done():
			return
		default:
			gameMgr.timeoutChan <- channel
		}
	})
	gameReq.mu.Unlock()

}

// JoinGame is called when a user wants to join an existing game request. It updates the game request status
// in the Slack channel. If the game request reaches quorum, it marks the game as ready to start and notifies the users. This function
// handles user interactions with the 'join' or 'Bin dabei!' button on the Slack message interface
// which triggers the `ACTION_JOIN_ROUND` action.
func (gameMgr *GameManager) JoinGame(channel SlackChannel, player string) {
	var updateMsg slack.MsgOption
	var gameMsgTS string
	var isGameComplete bool
	var players []string

	gameReq, exists := gameMgr.getGameRequest(channel)
	if !exists {
		gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Ein Fehler ist aufgetreten", false))
		return
	}

	// lock game to prevent data races on concurrent joins & leaves
	gameReq.mu.Lock()
	{
		// check if game is already full
		isGameComplete = len(gameReq.players) == gameReq.quorum
		if isGameComplete {
			gameReq.mu.Unlock()
			gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Das Spiel ist bereits voll.", false))
			return
		}

		if idx := slices.Index(gameReq.players, player); idx != -1 {
			gameReq.mu.Unlock()
			gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Du bist bereits im Spiel.", false))
			return
		}

		gameReq.players = append(gameReq.players, player)

		// check if game has become full after the player joined
		isGameComplete = len(gameReq.players) == gameReq.quorum
		gameMsgTS = gameReq.messageTs
		updateMsg = GameRequestUpdateMsg(gameReq.players, gameReq.quorum)
		players = slices.Clone(gameReq.players)
	}
	gameReq.mu.Unlock()

	if isGameComplete {
		var playerString = "<@" + strings.Join(players, ">, <@") + ">"
		var gameStartMessage = fmt.Sprintf("Die Runde ist voll, %s zum Kickertisch! :kicker:", playerString)
		var wg sync.WaitGroup
		wg.Add(len(players))
		for _, playerId := range players {
			go func(playerId string) {
				_, err := gameMgr.apiClient.PostEphemeral(string(channel), playerId, slack.MsgOptionText(gameStartMessage, false))
				if err != nil {
					slog.Error("Failed to ping user", "userid", playerId, "error", err.Error())
				}
				wg.Done()
			}(playerId)
		}
		wg.Wait()
		gameMgr.deleteGameRequest(channel)
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

// LeaveGame is called when a user wants to leave a game request they had previously joined. This function updates the
// game request status in the Slack channel. If all players leave, the game request is cancelled. It handles
// user interactions with the 'leave' or 'bin raus' button on the Slack message interface which triggers
// the 'ACTION_LEAVE_ROUND' action
func (gameMgr *GameManager) LeaveGame(channel SlackChannel, player string) {

	gameReq, exists := gameMgr.getGameRequest(channel)
	if !exists {
		gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Ein Fehler ist aufgetreten", false))
		return
	}

	var updateMsg slack.MsgOption
	var isLastPlayer bool
	var gameMsgTS string

	gameReq.mu.Lock()
	{
		idx := slices.Index(gameReq.players, player)
		if idx < 0 {
			gameReq.mu.Unlock()
			gameMgr.apiClient.PostEphemeral(string(channel), player, slack.MsgOptionText("Du bist nicht in der aktuellen Runde.", false))
			return
		}
		// remove player from game
		gameReq.players = append(gameReq.players[:idx], gameReq.players[idx+1:]...)
		isLastPlayer = len(gameReq.players) == 0
		updateMsg = GameRequestUpdateMsg(gameReq.players, gameReq.quorum)
		gameMsgTS = gameReq.messageTs
	}
	gameReq.mu.Unlock()

	if isLastPlayer {
		gameMgr.deleteGameRequest(channel)
		_, _, err := gameMgr.apiClient.DeleteMessage(string(channel), gameMsgTS)
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

func (gameMgr *GameManager) getGameRequest(channel SlackChannel) (*GameRequest, bool) {
	gameMgr.mu.Lock()
	defer gameMgr.mu.Unlock()

	game, exists := gameMgr.gameRequests[channel]
	return game, exists
}
func (gameMgr *GameManager) setGameRequest(channel SlackChannel, game *GameRequest) {
	gameMgr.mu.Lock()
	gameMgr.gameRequests[channel] = game
	gameMgr.mu.Unlock()
}

func (gameMgr *GameManager) deleteGameRequest(channel SlackChannel) {
	gameMgr.mu.Lock()
	defer gameMgr.mu.Unlock()

	if gameReq, exists := gameMgr.gameRequests[channel]; exists {
		gameReq.mu.Lock()
		if gameReq.timer != nil {
			gameReq.timer.Stop()
		}
		gameReq.mu.Unlock()
		delete(gameMgr.gameRequests, channel)
	}
}

// setGameRequestIfNotExists sets a new game for the specified channel only if there isn't already a game present.
// It returns true if the new game was set, or false if a game already exists for the channel.
func (gm *GameManager) setGameRequestIfNotExists(channel SlackChannel, game *GameRequest) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if _, exists := gm.gameRequests[channel]; exists {
		return false
	}

	gm.gameRequests[channel] = game
	return true
}

// handleTimeouts manages the timeouts of game requests. It listens for timeout
// signals on a channel and handles the expiration of game requests accordingly. When a timeout occurs, the
// function updates the game request and its associated Slack message from the specified channel.
func (gameMgr *GameManager) handleTimeouts() {
	for channel := range gameMgr.timeoutChan {
		if gameReq, exists := gameMgr.getGameRequest(channel); exists {
			ts := gameReq.messageTs
			gameMgr.deleteGameRequest(channel)
			gameMgr.apiClient.UpdateMessage(string(channel), ts, timeoutMSG)
		}
	}
}

// Shutdown closes the timeout channel and releases all used resources
func (gameMgr *GameManager) Shutdown(ctx context.Context) {
	var wg sync.WaitGroup

	gameReqCancels := make([]struct {
		channel   string
		messageTs string
	}, 0)

	gameMgr.mu.Lock()
	for channel, gameReq := range gameMgr.gameRequests {
		if gameReq.timerCancelFunc != nil {
			gameReq.timerCancelFunc()
		}
		gameReqCancels = append(gameReqCancels, struct {
			channel   string
			messageTs string
		}{
			channel:   string(channel),
			messageTs: gameReq.messageTs,
		})
	}
	clear(gameMgr.gameRequests)
	gameMgr.mu.Unlock()

	close(gameMgr.timeoutChan)

	for _, gr := range gameReqCancels {
		wg.Add(1)
		go func(channel, ts string) {
			defer wg.Done()
			_, _, err := gameMgr.apiClient.DeleteMessageContext(ctx, channel, ts)
			if err != nil {
				slog.Warn("Failed to delete game message on shutdown", "error", err.Error())
			}
		}(gr.channel, gr.messageTs)
	}

	wg.Wait()
}
