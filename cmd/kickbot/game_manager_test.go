package main

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	gomock "go.uber.org/mock/gomock"
)

func TestConcurrentGameCreationForSingleChannel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	// Only a single game should be created for the same channel -> PostMessage should be called once
	mockSlackClient.EXPECT().
		PostMessage(gomock.Any(), gomock.Any()).
		Return("channelID", "timestamp", nil).Times(1)

	// The rest should fail and users should get Ephemeral Message
	mockSlackClient.EXPECT().
		PostEphemeral(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("timestamp", nil).AnyTimes()

	gm := &GameManager{
		apiClient: mockSlackClient,
		games:     make(map[SlackChannel]*Game),
	}

	var wg sync.WaitGroup
	numberOfAttempts := 50
	for i := 0; i < numberOfAttempts; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			channelID := "sameChannel"
			gm.CreateGame(SlackChannel(channelID), userID, GameTypeTwoVsTwo)
		}(fmt.Sprintf("user%d", i))
	}

	wg.Wait()

	if len(gm.games) != 1 {
		t.Errorf("Expected only one game to be created, but found %d", len(gm.games))
	}
}

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

	gm := &GameManager{
		apiClient: mockSlackClient,
		games:     make(map[SlackChannel]*Game),
	}

	var wg sync.WaitGroup
	numberOfAttempts := 10
	for i := 0; i < numberOfAttempts; i++ {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			channelID := fmt.Sprintf("channel-%s", userID)
			gm.CreateGame(SlackChannel(channelID), userID, GameTypeTwoVsTwo)
		}(fmt.Sprintf("user%d", i))
	}

	wg.Wait()

	if len(gm.games) != nGames {
		t.Errorf("Expected only one game to be created, but found %d", len(gm.games))
	}
}

func TestConcurrentJoins(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	quorom := 4
	nJoins := 10

	// A game with a quorom of 4 should send one "PostMessage" and 3 "UpdateMessage"
	// 1 for the game announcement and 3 for each player Join

	mockSlackClient.EXPECT().
		PostMessage(gomock.Any(), gomock.Any()).
		Return("channelID", "timestamp", nil).Times(1)

	mockSlackClient.EXPECT().
		UpdateMessage(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("channelID", "ts", "text", nil).Times(quorom - 1)

	mockSlackClient.EXPECT().
		PostEphemeral(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("timestamp", nil).MaxTimes(nJoins - quorom + 1)

	gm := &GameManager{
		apiClient: mockSlackClient,
		games:     make(map[SlackChannel]*Game),
	}

	channelID := "12345678"

	// should trigger 1 "PostMessage"
	gm.CreateGame(SlackChannel(channelID), "user-0x", GameTypeTwoVsTwo)

	var wg sync.WaitGroup

	for i := 0; i < nJoins; i++ {
		userID := fmt.Sprintf("user%d", i)
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			gm.JoinGame(SlackChannel(channelID), userID)
		}(userID)
	}

	wg.Wait()

	if len(gm.games) != 0 {
		t.Errorf("Expected all games to be deleted, but found %d games", len(gm.games))
	}
}

func TestConcurrentLeaves(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	mockSlackClient.EXPECT().
		UpdateMessage(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("channelID", "ts", "text", nil).Times(2)

	mockSlackClient.EXPECT().
		DeleteMessage(gomock.Any(), gomock.Any()).
		Return("ch", "ts", nil).Times(1)

	gm := &GameManager{
		apiClient: mockSlackClient,
		games:     make(map[SlackChannel]*Game),
	}

	channel := "lpzg-24"
	players := []string{"p1", "p2", "p3"}

	gm.games[SlackChannel(channel)] = &Game{
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
			gm.LeaveGame(SlackChannel(channel), player)
		}(player)
	}
	wg.Wait()

	if len(gm.games) != 0 {
		t.Errorf("Expected all games to be deleted, but found %d games", len(gm.games))
	}
}

func TestConcurrentLeavesAndJoins(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

	channel := "lpzg-24"

	players := []string{"p1", "p2", "p3"}
	playersToLeave := []string{"p1", "p3"}
	playersToJoin := []string{"p4", "p5", "p6"}

	// Best Case Scenario 2 players leave, 3 pleyers join and we reach quorum of 4 -> 5 updates
	mockSlackClient.EXPECT().
		UpdateMessage(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("channelID", "ts", "text", nil).MaxTimes(5)

	// Worse case Scenario: 1 plyer joins before the other 2 leave, we reach quorum and game is deleted
	// the 2 other players to join and the 2 to leave all get error messages
	mockSlackClient.EXPECT().
		DeleteMessage(gomock.Any(), gomock.Any()).
		Return("ch", "ts", nil).MaxTimes(1)
	mockSlackClient.EXPECT().
		PostEphemeral(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("timestamp", nil).MaxTimes(4)

	gm := &GameManager{
		apiClient: mockSlackClient,
		games:     make(map[SlackChannel]*Game),
	}

	gm.games[SlackChannel(channel)] = &Game{
		players:   slices.Clone(players),
		quorum:    4,
		messageTs: "ts",
		mu:        &sync.Mutex{},
	}

	wg := &sync.WaitGroup{}
	for _, player := range playersToLeave {
		wg.Add(1)
		go func(player string) {
			defer wg.Done()
			gm.LeaveGame(SlackChannel(channel), player)
		}(player)
	}
	for _, player := range playersToJoin {
		wg.Add(1)
		go func(player string) {
			defer wg.Done()
			gm.JoinGame(SlackChannel(channel), player)
		}(player)
	}
	wg.Wait()

	if len(gm.games) != 0 {
		t.Errorf("Expected all games to be deleted, but found %d games", len(gm.games))
	}
}

func TestConcurrentLeaveAndJoin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSlackClient := NewMockSlackClient(ctrl)

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

	gm := &GameManager{
		apiClient: mockSlackClient,
		games:     make(map[SlackChannel]*Game),
	}

	gm.games[SlackChannel(channel)] = &Game{
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
		gm.LeaveGame(SlackChannel(channel), initialPlayer)
	}()
	go func() {
		defer wg.Done()
		gm.JoinGame(SlackChannel(channel), joiningPlayer)
	}()

	wg.Wait()

	game, exists := gm.games[SlackChannel(channel)]
	if exists {
		if len(game.players) != 1 {
			t.Errorf("Expected the game to have 1 player, found %d", len(game.players))
		}
	}
}
