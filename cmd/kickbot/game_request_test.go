package main

import (
	"testing"
)

func TestNewGameRequest(t *testing.T) {
	player := "Player1"

	// Test for 2 vs 2 Game
	gameReq1 := NewGameRequest(GameTypeTwoVsTwo, player)
	if gameReq1.quorum != 4 {
		t.Errorf("Expected quorum of 4 for TwoVsTwoGame, got %d", gameReq1.quorum)
	}
	if len(gameReq1.players) != 1 || gameReq1.players[0] != player {
		t.Errorf("Expected player list [%s], got %v", player, gameReq1.players)
	}

	// Test for 1 vs 1 Game
	gameReq2 := NewGameRequest(GameTypeOneVsOne, player)
	if gameReq2.quorum != 2 {
		t.Errorf("Expected quorum of 2 for OneVsOneGame, got %d", gameReq2.quorum)
	}
	if len(gameReq2.players) != 1 || gameReq2.players[0] != player {
		t.Errorf("Expected player list [%s], got %v", player, gameReq2.players)
	}
}
