package game

import "strings"

// Phase type
type Phase int

// Phase constants
const (
	LOBBY Phase = iota
	TASKS
	DISCUSS
	MENU
	GAMEOVER
	UNINITIALIZED
)

type PhaseNameString string

// PhaseNames for lowercase, possibly for translation if needed
var PhaseNames = map[Phase]PhaseNameString{
	LOBBY:   "LOBBY",
	TASKS:   "TASKS",
	DISCUSS: "DISCUSSION",
	MENU:    "MENU",
}

// ToString for a Phase
func (phase *Phase) ToString() PhaseNameString {
	return PhaseNames[*phase]
}

func GetPhaseFromString(input string) Phase {
	if len(input) == 0 {
		return UNINITIALIZED
	}

	switch strings.ToLower(input) {
	case "lobby":
		fallthrough
	case "l":
		return LOBBY
	case "task":
		fallthrough
	case "t":
		fallthrough
	case "tasks":
		fallthrough
	case "game":
		fallthrough
	case "g":
		return TASKS
	case "discuss":
		fallthrough
	case "disc":
		fallthrough
	case "d":
		fallthrough
	case "discussion":
		return DISCUSS
	default:
		return UNINITIALIZED
	}
}
