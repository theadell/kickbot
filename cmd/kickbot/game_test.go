package main

import (
	"testing"
)

func TestNewGame(t *testing.T) {
	player := "Player1"

	// Test for 2 vs 2 Game
	game1 := NewGame(GameTypeTwoVsTwo, player)
	if game1.quorum != 4 {
		t.Errorf("Expected quorum of 4 for TwoVsTwoGame, got %d", game1.quorum)
	}
	if len(game1.players) != 1 || game1.players[0] != player {
		t.Errorf("Expected player list [%s], got %v", player, game1.players)
	}

	// Test for 1 vs 1 Game
	game2 := NewGame(GameTypeOneVsOne, player)
	if game2.quorum != 2 {
		t.Errorf("Expected quorum of 2 for OneVsOneGame, got %d", game2.quorum)
	}
	if len(game2.players) != 1 || game2.players[0] != player {
		t.Errorf("Expected player list [%s], got %v", player, game2.players)
	}
}
