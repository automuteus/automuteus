package game

import "strings"

// Phase type
type Phase int

// Phase constants
const (
	LOBBY   Phase = iota
	TASKS   Phase = iota
	DISCUSS Phase = iota
	MENU    Phase = iota
	//VOTING        Phase = iota
	//GAMEOVER      Phase = iota
	UNINITIALIZED Phase = iota
)

// PhaseNames for lowercase, possibly for translation if needed
var PhaseNames = map[string]Phase{
	"lobby":      LOBBY,
	"tasks":      TASKS,
	"discussion": DISCUSS,
	"menu":       MENU,
}

func getPhaseNameForInt(phase *Phase) string {
	for str, idx := range PhaseNames {
		if idx == *phase {
			return str
		}
	}
	return ""
}

// ToString for a phase
func (phase *Phase) ToString() string {
	return strings.ToUpper(getPhaseNameForInt(phase))
}

// Player struct
type Player struct {
	Action       int    `json:"Action"`
	Name         string `json:"Name"`
	Color        int    `json:"Color"`
	IsDead       bool   `json:"IsDead"`
	Disconnected bool   `json:"Disconnected"`
}

// PlayerUpdate struct
type PlayerUpdate struct {
	Player  Player
	GuildID string
}

// PhaseUpdate struct
type PhaseUpdate struct {
	Phase   Phase
	GuildID string
}
