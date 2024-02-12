package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	gomock "go.uber.org/mock/gomock"
)

// TestConcurrentGameCreationForSingleChannel verifies the behavior of concurrently creating game requests in the same Slack channel.
// The test uses a mock Slack client to simulate the GameManager's response to multiple concurrent
// game request commands (Creating game request) and interactions (leaving and joining).
// The GameManager is expected to handle these scenarios:
//  1. Only the first valid game request should lead to a public announcement in the channel (handled by PostMessage).
//  2. Any subsequent game requests in the same channel, while the first game request is still active, should result in
//     an ephemeral message to the user who attempted to start the new game (handled by PostEphemeral), indicating that
//     a game request is already in progress.
//
// This test creates 50 concurrent game request attempts in the same channel to ensure that the GameManager respects
// the constraint of one active game request per channel.
func TestConcurrentGameCreationForSingleChannel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)
	defer gameMgr.Shutdown(context.TODO())

	// A channel can only have a single active game request. when a game is created a message is sent to the channel when `PostMessage` is invoked
	// when other users try to create a new game request in the same channel they will recieve an ephemeral error message when `PostEphemeral` is invoked
	// Expected is: 1 `PostMessage` and the rest should be PostEphemeral
	mockSlackClient.EXPECT().
		PostMessage(gomock.Any(), gomock.Any()).
		Return("channelID", "timestamp", nil).Times(1)

	mockSlackClient.EXPECT().
		PostEphemeral(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("timestamp", nil).AnyTimes()

	// Shutdown should cleanup and delete any trailing game requests (which should be exactly 1 game)
	mockSlackClient.EXPECT().DeleteMessageContext(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	var wg sync.WaitGroup
	numberOfAttempts := 50
	for i := range numberOfAttempts {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			channelID := "sameChannel"
			gameMgr.CreateGame(SlackChannel(channelID), userID, GameTypeTwoVsTwo)
		}(fmt.Sprintf("user%d", i))
	}

	wg.Wait()

	if len(gameMgr.gameRequests) != 1 {
		t.Errorf("Expected only one game to be created, but found %d", len(gameMgr.gameRequests))
	}
}

// TestConcurrentGameCreationForMultipleChannels verifies that GameManager permits multiple game requests concurrently, each in a unique channel.
// The test concurrently initiates 10 game requests in 10 distinct channels. It confirms that GameManager permits one game per channel by expecting
// `PostMessage` to be called once for each channel, signaling a game announcement. No `PostEphemeral` calls should occur, indicating the absence of
// errors related to game request conflicts.
func TestConcurrentGameCreationForMultipleChannels(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	nGames := 10

	mockSlackClient.EXPECT().
		PostMessage(gomock.Any(), gomock.Any()).
		Return("channelID", "timestamp", nil).Times(nGames)

	mockSlackClient.EXPECT().
		PostEphemeral(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("timestamp", nil).MaxTimes(0)

	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)

	var wg sync.WaitGroup
	numberOfAttempts := 10
	for i := range numberOfAttempts {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			channelID := fmt.Sprintf("channel-%s", userID)
			gameMgr.CreateGame(SlackChannel(channelID), userID, GameTypeTwoVsTwo)
		}(fmt.Sprintf("user%d", i))
	}

	wg.Wait()

	if len(gameMgr.gameRequests) != nGames {
		t.Errorf("Expected only one game to be created, but found %d", len(gameMgr.gameRequests))
	}
}

// TestConcurrentJoins checks GameManager's handling of concurrent join requests for an existing game.
// The test ensures that once a game reaches its required quorum, further join attempts are blocked with error messages,
// and the game request is deleted. It verifies that only the necessary number of players can join, and any simultaneous extra join attempts
// are correctly handled as game already started.
func TestConcurrentJoins(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)
	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)
	defer gameMgr.Shutdown(context.TODO())

	quorom := 4
	nJoins := 10

	// Expectations for Slack client interaction:
	// - One "PostMessage" for the game's initial announcement.
	// - "UpdateMessage" called three times (`quorom - 1`) for player joins until quorum (4 players) is reached.
	// - "PostEphemeral" called for notifying the 4 successful joins and sending 7 error messages to players who attempt joining after the game is full.
	// In essence, 10 players try to join a game that already has 1 player. Only 3 additional players can join successfully,
	// with each successful join triggering an "UpdateMessage". All players, successful or not, receive a "PostEphemeral" notification about their join status.

	mockSlackClient.EXPECT().
		PostMessage(gomock.Any(), gomock.Any()).
		Return("channelID", "timestamp", nil).Times(1)

	mockSlackClient.EXPECT().
		UpdateMessage(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("channelID", "ts", "text", nil).Times(quorom - 1)

	mockSlackClient.EXPECT().
		PostEphemeral(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("timestamp", nil).MaxTimes(nJoins + 1)

	channelID := "12345678"

	// should trigger 1 "PostMessage"
	gameMgr.CreateGame(SlackChannel(channelID), "user-0x", GameTypeTwoVsTwo)

	var wg sync.WaitGroup

	for i := range nJoins {
		userID := fmt.Sprintf("user%d", i)
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			gameMgr.JoinGame(SlackChannel(channelID), userID)
		}(userID)
	}

	wg.Wait()

	gameMgr.mu.Lock()
	if len(gameMgr.gameRequests) != 0 {
		t.Errorf("Expected all games to be deleted, but found %d games", len(gameMgr.gameRequests))
	}
	gameMgr.mu.Unlock()
}

// TestConcurrentLeaves verifies the GameManager's handling of multiple players leaving a game simultaneously.
// The test focuses on ensuring that when several players leave, the game updates correctly and eventually gets deleted if all players leave.
func TestConcurrentLeaves(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)
	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)
	defer gameMgr.Shutdown(context.TODO())

	mockSlackClient.EXPECT().
		UpdateMessage(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("channelID", "ts", "text", nil).Times(2)

	mockSlackClient.EXPECT().
		DeleteMessage(gomock.Any(), gomock.Any()).
		Return("ch", "ts", nil).Times(1)

	channel := "lpzg-24"
	players := []string{"p1", "p2", "p3"}

	gameMgr.gameRequests[SlackChannel(channel)] = &GameRequest{
		players:   slices.Clone(players),
		quorum:    4,
		messageTs: "ts",
		mu:        &sync.Mutex{},
	}

	wg := &sync.WaitGroup{}
	for _, player := range players {
		wg.Add(1)
		go func(player string) {
			defer wg.Done()
			gameMgr.LeaveGame(SlackChannel(channel), player)
		}(player)
	}
	wg.Wait()

	gameMgr.mu.Lock()
	if len(gameMgr.gameRequests) != 0 {
		t.Errorf("Expected all games to be deleted, but found %d games", len(gameMgr.gameRequests))
	}
	gameMgr.mu.Unlock()
}

// TestConcurrentLeavesAndJoins checks that GameManager correctly handles concurrent leaves and joins for a game request.
// The test verifies proper game state management and appropriate player notifications during these simultaneous actions.
func TestConcurrentLeavesAndJoins(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	channel := "lpzg-24"

	players := []string{"p1", "p2", "p3"}
	playersToLeave := []string{"p1", "p3"}
	playersToJoin := []string{"p4", "p5", "p6"}

	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)
	defer gameMgr.Shutdown(context.TODO())

	// Case Scenario I : 2 players leave THEN 3 players join and we reach quorum of 4
	// EXPECT 5 updates for each leave and join actions. 4 notifications (PostEphemeral) for each player when game reaches quorum
	//
	// Case Scenario II: 1 player joins before the other 2 leave. Game reaches quorum and is deleted.
	// 2 Players send `join` and Players send leave after the game is deleted. They recieve 4 notifications (PostEphemeral) with errors
	// and the 4 players of the game recieve exactly 4 notifications (PostEphemeral) for the game completion
	//
	// From I And II: Expected Max of 5 UpdateMessage, 8 PostEphemeral and 1 DeleteMessage

	mockSlackClient.EXPECT().
		UpdateMessage(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("channelID", "ts", "text", nil).MaxTimes(5)

	mockSlackClient.EXPECT().
		DeleteMessage(gomock.Any(), gomock.Any()).
		Return("ch", "ts", nil).MaxTimes(1)
	mockSlackClient.EXPECT().
		PostEphemeral(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("timestamp", nil).AnyTimes()

	gameMgr.gameRequests[SlackChannel(channel)] = &GameRequest{
		players:   slices.Clone(players),
		quorum:    4,
		messageTs: "ts",
		mu:        &sync.Mutex{},
	}

	wg := sync.WaitGroup{}
	for _, player := range playersToLeave {
		wg.Add(1)
		go func(player string) {
			defer wg.Done()
			gameMgr.LeaveGame(SlackChannel(channel), player)
		}(player)
	}
	for _, player := range playersToJoin {
		wg.Add(1)
		go func(player string) {
			defer wg.Done()
			gameMgr.JoinGame(SlackChannel(channel), player)
		}(player)
	}
	wg.Wait()

	if len(gameMgr.gameRequests) != 0 {
		t.Errorf("Expected all games to be deleted, but found %d games", len(gameMgr.gameRequests))
	}
}

// TestConcurrentLeaveAndJoin checks that GameManager handles a case where the last or only player in a game request attempts to leave
// at the same time another player tries to join. The expected behavior varies depending on which action is processed first:
//   - If the last player's leave action is processed first, the game request is deleted, leading to an error message for the joining player.
//   - If the join action precedes the leave, the game updates to include the new player, followed by an update for the leave action.
func TestConcurrentLeaveAndJoin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)
	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)

	channel := "lpzg-24"

	// Starting with one player in the game
	initialPlayer := "p1"
	joiningPlayer := "p2"

	// Expecting at most 2 UpdateMessage if the join happens before the leave
	mockSlackClient.EXPECT().
		UpdateMessage(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("channelID", "ts", "text", nil).MaxTimes(2)

	// Expecting DeleteMessage if the leave happens before the join
	mockSlackClient.EXPECT().
		DeleteMessage(gomock.Any(), gomock.Any()).
		Return("ch", "ts", nil).MaxTimes(1)

	// Expecting an error message if the join happens after the game is deleted
	mockSlackClient.EXPECT().
		PostEphemeral(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("timestamp", nil).MaxTimes(1)

	gameMgr.gameRequests[SlackChannel(channel)] = &GameRequest{
		players:   []string{initialPlayer},
		quorum:    4,
		messageTs: "ts",
		mu:        &sync.Mutex{},
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	// Concurrent leave and join
	go func() {
		defer wg.Done()
		gameMgr.LeaveGame(SlackChannel(channel), initialPlayer)
	}()
	go func() {
		defer wg.Done()
		gameMgr.JoinGame(SlackChannel(channel), joiningPlayer)
	}()

	wg.Wait()

	game, exists := gameMgr.gameRequests[SlackChannel(channel)]
	if exists {
		if len(game.players) != 1 {
			t.Errorf("Expected the game to have 1 player, found %d", len(game.players))
		}
	}
}

// TestGameReqTimeout verfies that game requests will be deleted once they time out.
func TestGameReqTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	nGames := 20

	// nGames message for the game creation
	mockSlackClient.EXPECT().
		PostMessage(gomock.Any(), gomock.Any()).
		Return("channelID", "timestamp", nil).Times(nGames)

	// nGames messages for game deletion
	mockSlackClient.EXPECT().
		DeleteMessage(gomock.Any(), gomock.Any()).
		Return("ch", "ts", nil).MaxTimes(nGames)

	gameMgr := NewGameManager(mockSlackClient, time.Millisecond*100)
	defer gameMgr.Shutdown(context.TODO())

	for i := range nGames {
		gameMgr.CreateGame(SlackChannel(fmt.Sprintf("channel-%d", i)), "test", GameTypeTwoVsTwo)
	}
	time.Sleep(125 * time.Millisecond)
	gameMgr.mu.Lock()
	if len(gameMgr.gameRequests) != 0 {
		t.Errorf("Expected all games to be deleted, but found %d games", len(gameMgr.gameRequests))
	}
	gameMgr.mu.Unlock()
}

func TestGameCompletionBeforeTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	// Message For game creation
	mockSlackClient.EXPECT().
		PostMessage(gomock.Any(), gomock.Any()).
		Return("channelID", "timestamp", nil).Times(1)

	// Update message for game completion
	mockSlackClient.EXPECT().
		UpdateMessage(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("channelID", "ts", "text", nil).MaxTimes(2)

	// Ephemeral message to every player that joined the game
	mockSlackClient.EXPECT().
		PostEphemeral(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("timestamp", nil).Times(2)

	// No Deletion Should Happen
	mockSlackClient.EXPECT().
		DeleteMessage(gomock.Any(), gomock.Any()).
		Return("ch", "ts", nil).Times(0)

	gameMgr := NewGameManager(mockSlackClient, 100*time.Millisecond)
	defer gameMgr.Shutdown(context.TODO())
	channel := SlackChannel("test-channel")

	time.Sleep(50 * time.Millisecond)

	gameMgr.CreateGame(channel, "test-player-01", GameTypeOneVsOne)
	gameMgr.JoinGame(channel, "test-player-02")

	// Check if the game has been deleted
	gameMgr.mu.Lock()
	if _, exists := gameMgr.gameRequests[channel]; exists {
		t.Errorf("Game was not correctly deleted upon completion")
	}
	gameMgr.mu.Unlock()
}

func TestGameCancellationBeforeTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	// Expect 1 PostMessage For game creation
	mockSlackClient.EXPECT().
		PostMessage(gomock.Any(), gomock.Any()).
		Return("channelID", "timestamp", nil).Times(1)
	// Expect 1 deletion for cancellation
	mockSlackClient.EXPECT().
		DeleteMessage(gomock.Any(), gomock.Any()).
		Return("ch", "ts", nil).Times(1)

	gameMgr := NewGameManager(mockSlackClient, 100*time.Millisecond)
	defer gameMgr.Shutdown(context.TODO())

	channel := SlackChannel("test-channel")
	player := "test-player"
	gameMgr.CreateGame(channel, player, GameTypeTwoVsTwo)

	gameMgr.LeaveGame(channel, player)

	time.Sleep(100 * time.Millisecond)

	gameMgr.mu.Lock()
	if _, exists := gameMgr.gameRequests[channel]; exists {
		t.Errorf("Game was not correctly deleted upon cancellation")
	}
	gameMgr.mu.Unlock()
}

// TestPlayerCannotJoinGameTwice verfied that a user cannot double joins a game request
func TestPlayerCannotJoinGameTwice(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	channelId := "test-channel"
	channel := SlackChannel(channelId)
	player1 := "test-player-1"
	player2 := "test-player-2"

	// Expect 1 PostMessage For game creation
	mockSlackClient.EXPECT().
		PostMessage(channelId, gomock.Any()).
		Return("channelID", "timestamp", nil).Times(1)

	// Expect 1 update message for a valid join
	mockSlackClient.EXPECT().
		UpdateMessage(channelId, gomock.Any(), gomock.Any()).
		Return("channelID", "ts", "text", nil).Times(1)

	// Expect 1 PostEphemeral double joining error for player1
	mockSlackClient.EXPECT().
		PostEphemeral(channelId, player1, gomock.Any()).
		Return("timestamp", nil).Times(1)

	// Expect 1 PostEphemeral double joining error for player2
	mockSlackClient.EXPECT().
		PostEphemeral(channelId, player2, gomock.Any()).
		Return("timestamp", nil).Times(1)

	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)

	// player 1 creates game
	gameMgr.CreateGame(channel, player1, GameTypeTwoVsTwo)
	// player 2 double joins
	gameMgr.JoinGame(channel, player1)
	// player 2 joins
	gameMgr.JoinGame(channel, player2)
	// player 2 joins
	gameMgr.JoinGame(channel, player2)

	game, exists := gameMgr.getGameRequest(channel)
	if !exists {
		t.Error("Game incorrectly deleted")
	}
	if numPlayers := len(game.players); numPlayers != 2 {
		t.Errorf("Expected 2 players in game but found %d", numPlayers)
	}
}

func TestUserCannotLeaveGameRequestTheyNotPartOf(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	gameMgr := NewGameManager(mockSlackClient, 10*time.Second)

	channelId := "test-channel"
	channel := SlackChannel(channelId)
	gameMaker := "test-player"
	leaver := "player-attempting-to-leave-game-they-are-not-part-of"
	// Expect 1 PostMessage For game creation
	mockSlackClient.EXPECT().
		PostMessage(channelId, gomock.Any()).
		Return(channelId, "timestamp", nil).Times(1)
	// Expect 1 PostEphemeral double joining error for player1
	mockSlackClient.EXPECT().
		PostEphemeral(channelId, leaver, gomock.Any()).
		Return("timestamp", nil).Times(1)
	gameMgr.CreateGame(channel, gameMaker, GameTypeOneVsOne)
	gameMgr.LeaveGame(channel, leaver)

	gameRequest, _ := gameMgr.getGameRequest(channel)
	if numPlayers := len(gameRequest.players); numPlayers != 1 {
		t.Errorf("Expected to find 1 player in game request but found %d", numPlayers)
	}
}

func TestUserCannotLeaveOrJoinGameRequestThatDoesntExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	gameMgr := NewGameManager(mockSlackClient, 100*time.Millisecond)
	defer gameMgr.Shutdown(context.TODO())

	channelID := "channel-with-no-active-game-requests"
	channel := SlackChannel(channelID)

	user := "test-player"

	// Expect 2 PostEphemeral errors for user
	mockSlackClient.EXPECT().
		PostEphemeral(channelID, user, gomock.Any()).
		Return("timestamp", nil).AnyTimes()

	gameMgr.JoinGame(channel, user)
	gameMgr.LeaveGame(channel, user)

	if len(gameMgr.gameRequests) != 0 {
		t.Errorf("Expected to fine zero game requests, but found %d", len(gameMgr.gameRequests))
	}

}

// TestGameCreationFailure simulates a network failur to send a public announcement in the channel for the game creation
// and verfies that the game request will be deleted and the the game manager will attempt to send an ephemeral
// error message to the user
func TestGameCreationFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSlackClient := NewMockSlackClient(ctrl)
	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)
	defer gameMgr.Shutdown(context.TODO())

	channelId := "test-channel"
	channel := SlackChannel(channelId)
	player := "p1"

	// Expect 1 PostMessage For game creation
	mockSlackClient.EXPECT().
		PostMessage(channelId, gomock.Any()).
		Return("", "", errors.New("simulate network error")).Times(1)

	// Expect 1 PostEphemeral for error message
	mockSlackClient.EXPECT().
		PostEphemeral(channelId, player, gomock.Any()).
		Return("timestamp", nil).Times(1)

	gameMgr.CreateGame(channel, player, GameTypeOneVsOne)

	if len(gameMgr.gameRequests) != 0 {
		t.Errorf("Expected no game requests but found %d", len(gameMgr.gameRequests))
	}

}

func TestGameCompletionNotification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)
	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)
	defer gameMgr.Shutdown(context.TODO())

	channelID := "12345678"
	channel := SlackChannel(channelID)
	p1, p2, p3, p4 := "p1", "p2", "p3", "p4"

	// Game annonciation
	mockSlackClient.EXPECT().
		PostMessage(channelID, gomock.Any()).
		Return("channelID", "timestamp", nil).Times(1)

	// update on each join
	mockSlackClient.EXPECT().
		UpdateMessage(channelID, gomock.Any(), gomock.Any()).
		Return("channelID", "ts", "text", nil).Times(3)

	// 4 notifications
	mockSlackClient.EXPECT().
		PostEphemeral(channelID, p1, gomock.Any()).
		Return("timestamp", nil).MaxTimes(1)
	mockSlackClient.EXPECT().
		PostEphemeral(channelID, p2, gomock.Any()).
		Return("timestamp", nil).MaxTimes(1)
	mockSlackClient.EXPECT().
		PostEphemeral(channelID, p3, gomock.Any()).
		Return("timestamp", nil).MaxTimes(1)
	mockSlackClient.EXPECT().
		PostEphemeral(channelID, p4, gomock.Any()).
		Return("timestamp", nil).MaxTimes(1)

	// should trigger 1 "PostMessage"
	gameMgr.CreateGame(SlackChannel(channelID), p1, GameTypeTwoVsTwo)
	// should trigger 3 updates
	gameMgr.JoinGame(channel, p2)
	gameMgr.JoinGame(channel, p3)
	gameMgr.JoinGame(channel, p4)

	gameMgr.mu.Lock()
	if len(gameMgr.gameRequests) != 0 {
		t.Errorf("Expected all games to be deleted, but found %d games", len(gameMgr.gameRequests))
	}
	gameMgr.mu.Unlock()
}

func TestGameManagerShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)
	gameMgr := NewGameManager(mockSlackClient, DEFAULT_GAMEREQ_TIMEOUT)

	nGames := 10

	for i := range nGames {
		channelID := fmt.Sprintf("channel-%02d", i)

		ts := fmt.Sprintf("ts-%02d", i)

		// Expect a single announcement per game
		mockSlackClient.EXPECT().PostMessage(gomock.Any(), gomock.Any()).
			Return(channelID, ts, nil).
			Times(1)

		// Expect a single cleanup per game
		mockSlackClient.EXPECT().DeleteMessageContext(gomock.Any(), channelID, ts).Times(1)

		gameMgr.CreateGame(SlackChannel(channelID), fmt.Sprint(i), GameTypeTwoVsTwo)

	}

	gameMgr.Shutdown(context.TODO())
	if len(gameMgr.gameRequests) != 0 {
		t.Errorf("Expected all games to be deleted, but found %d games", len(gameMgr.gameRequests))
	}
}
