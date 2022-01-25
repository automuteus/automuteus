package amongus

import (
	"github.com/automuteus/utils/pkg/game"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
)

var PhaseMessages = map[game.Phase]*i18n.Message{
	game.LOBBY:    {ID: "state.phase.LOBBY", Other: "LOBBY"},
	game.TASKS:    {ID: "state.phase.TASKS", Other: "TASKS"},
	game.DISCUSS:  {ID: "state.phase.DISCUSSION", Other: "DISCUSSION"},
	game.MENU:     {ID: "state.phase.MENU", Other: "MENU"},
	game.GAMEOVER: {ID: "state.phase.GAMEOVER", Other: "GAME OVER"},
}

// TODO move to utils
func GetPhaseFromString(input string) game.Phase {
	if len(input) == 0 {
		return game.UNINITIALIZED
	}

	switch strings.ToLower(input) {
	case "lobby":
		fallthrough
	case "l":
		return game.LOBBY
	case "task":
		fallthrough
	case "t":
		fallthrough
	case "tasks":
		fallthrough
	case "game":
		fallthrough
	case "g":
		return game.TASKS
	case "discuss":
		fallthrough
	case "disc":
		fallthrough
	case "d":
		fallthrough
	case "discussion":
		return game.DISCUSS
	default:
		return game.UNINITIALIZED
	}
}

func ToLocale(phase game.Phase) *i18n.Message {
	return PhaseMessages[phase]
}
