package amongus

import (
	"github.com/automuteus/utils/pkg/game"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var PhaseMessages = map[game.Phase]*i18n.Message{
	game.LOBBY:    {ID: "state.phase.LOBBY", Other: "LOBBY"},
	game.TASKS:    {ID: "state.phase.TASKS", Other: "TASKS"},
	game.DISCUSS:  {ID: "state.phase.DISCUSSION", Other: "DISCUSSION"},
	game.MENU:     {ID: "state.phase.MENU", Other: "MENU"},
	game.GAMEOVER: {ID: "state.phase.GAMEOVER", Other: "GAME OVER"},
}

func ToLocale(phase game.Phase) *i18n.Message {
	return PhaseMessages[phase]
}
