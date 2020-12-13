package amongus

import "github.com/automuteus/utils/pkg/game"

// GameDelays struct
type GameDelays struct {
	//maps from origin->new phases, with the integer number of seconds for the delay
	Delays map[game.PhaseNameString]map[game.PhaseNameString]int `json:"delays"`
}

func MakeDefaultDelays() GameDelays {
	return GameDelays{
		Delays: map[game.PhaseNameString]map[game.PhaseNameString]int{
			game.PhaseNames[game.LOBBY]: {
				game.PhaseNames[game.LOBBY]:   0,
				game.PhaseNames[game.TASKS]:   7,
				game.PhaseNames[game.DISCUSS]: 0,
			},
			game.PhaseNames[game.TASKS]: {
				game.PhaseNames[game.LOBBY]:   1,
				game.PhaseNames[game.TASKS]:   0,
				game.PhaseNames[game.DISCUSS]: 0,
			},
			game.PhaseNames[game.DISCUSS]: {
				game.PhaseNames[game.LOBBY]:   6,
				game.PhaseNames[game.TASKS]:   7,
				game.PhaseNames[game.DISCUSS]: 0,
			},
		},
	}
}

func (gd *GameDelays) GetDelay(origin, dest game.Phase) int {
	return gd.Delays[game.PhaseNames[origin]][game.PhaseNames[dest]]
}
