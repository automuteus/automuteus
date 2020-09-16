package game

import (
	"bytes"
	"fmt"
)

type GamePhase int

const (
	UNINITIALIZED GamePhase = iota
	MENU          GamePhase = iota
	LOBBY         GamePhase = iota
	GAME          GamePhase = iota
	DISCUSS       GamePhase = iota
	VOTING        GamePhase = iota
	GAMEOVER      GamePhase = iota
)

var PhaseStrings = []string{
	"UNINITIALIZED",
	"MENU",
	"LOBBY",
	"GAME",
	"DISCUSS",
	"VOTING",
	"GAMEOVER",
}

type Player struct {
	Name  string `json:"playerName"`
	Color string `json:"color"`
	IsDead bool   `json:"isDead"`
}

type GameState struct {
	Phase   GamePhase `json:"phase"`
	Players []Player  `json:"players"`
}

func (state GameState) ToString() string {
	buf := bytes.NewBuffer([]byte("Game State:\n"))
	buf.WriteString(fmt.Sprintf("  Phase: %s\n", PhaseStrings[state.Phase]))
	for i, v := range state.Players {
		buf.WriteString(fmt.Sprintf("  Player %d: {Name: %s, Color: %s, IsDead: %v}\n", i, v.Name, v.Color, v.IsDead))
	}
	return buf.String()
}
