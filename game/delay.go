package game

// GameDelays struct
type GameDelays struct {
	//maps from origin->new phases, with the integer number of seconds for the delay
	Delays map[PhaseNameString]map[PhaseNameString]int `json:"delays"`
}

func MakeDefaultDelays() GameDelays {
	return GameDelays{
		Delays: map[PhaseNameString]map[PhaseNameString]int{
			PhaseNames[LOBBY]: {
				PhaseNames[LOBBY]:   0,
				PhaseNames[TASKS]:   7,
				PhaseNames[DISCUSS]: 0,
			},
			PhaseNames[TASKS]: {
				PhaseNames[LOBBY]:   1,
				PhaseNames[TASKS]:   0,
				PhaseNames[DISCUSS]: 0,
			},
			PhaseNames[DISCUSS]: {
				PhaseNames[LOBBY]:   6,
				PhaseNames[TASKS]:   7,
				PhaseNames[DISCUSS]: 0,
			},
		},
	}
}

func (gd *GameDelays) GetDelay(origin, dest Phase) int {
	return gd.Delays[PhaseNames[origin]][PhaseNames[dest]]
}
