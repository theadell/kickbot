package main

import "sync"

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

type Game struct {
	players   []string
	quorum    int
	messageTs string
	mu        *sync.Mutex
}

func NewGame(gameType GameType, player string) *Game {
	return &Game{
		players:   []string{player},
		quorum:    quorumMap[gameType],
		messageTs: "",
		mu:        &sync.Mutex{},
	}
}
