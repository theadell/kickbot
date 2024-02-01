package main

import (
	"sync"
	"time"
)

type GameType int

const (
	GameTypeTwoVsTwo GameType = iota // 2 vs 2 football table game
	GameTypeOneVsOne                 // 1 vs 1 football table game
)

// quorumMap maps game types to their quorum values
var quorumMap = map[GameType]int{
	GameTypeTwoVsTwo: 4, // 2 vs 2 game requires 4 players
	GameTypeOneVsOne: 2, // 1 vs 1 game requires 2 players
}

type GameRequest struct {
	players   []string
	quorum    int    // number of players needed for the game
	messageTs string // slack timestamp for the message of the game request sent by the bot
	mu        *sync.Mutex
	timer     *time.Timer // Timeout timer
}

func NewGameRequest(gameType GameType, player string) *GameRequest {
	return &GameRequest{
		players:   []string{player},
		quorum:    quorumMap[gameType],
		messageTs: "",
		mu:        &sync.Mutex{},
	}
}
