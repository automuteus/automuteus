package game

type VoiceRules struct {
	MuteRules map[PhaseNameString]map[string]bool
	DeafRules map[PhaseNameString]map[string]bool
}

func (rules *VoiceRules) GetVoiceState(isAlive, isTracked bool, phase Phase) (bool, bool) {
	if !isTracked {
		return false, false
	}
	aliveStr := "dead"
	if isAlive {
		aliveStr = "alive"
	}
	phaseStr := PhaseNames[phase]

	return rules.MuteRules[phaseStr][aliveStr], rules.DeafRules[phaseStr][aliveStr]
}

func MakeMuteAndDeafenRules() VoiceRules {
	rules := VoiceRules{
		MuteRules: map[PhaseNameString]map[string]bool{
			PhaseNames[LOBBY]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
			PhaseNames[TASKS]: map[string]bool{
				"alive": true,
				"dead":  false,
			},
			PhaseNames[DISCUSS]: map[string]bool{
				"alive": false,
				"dead":  true,
			},
		},
		DeafRules: map[PhaseNameString]map[string]bool{
			PhaseNames[LOBBY]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
			PhaseNames[TASKS]: map[string]bool{
				"alive": true,
				"dead":  false,
			},
			PhaseNames[DISCUSS]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
		},
	}
	return rules
}

func MakeMuteOnlyRules() VoiceRules {
	rules := VoiceRules{
		MuteRules: map[PhaseNameString]map[string]bool{
			PhaseNames[LOBBY]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
			PhaseNames[TASKS]: map[string]bool{
				"alive": true,
				"dead":  true,
			},
			PhaseNames[DISCUSS]: map[string]bool{
				"alive": false,
				"dead":  true,
			},
		},
		DeafRules: map[PhaseNameString]map[string]bool{
			PhaseNames[LOBBY]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
			PhaseNames[TASKS]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
			PhaseNames[DISCUSS]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
		},
	}
	return rules
}
