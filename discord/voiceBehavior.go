package discord

import "github.com/denverquane/amongusdiscord/game"

type VoiceRules struct {
	MuteRules map[game.PhaseNameString]map[string]bool
	DeafRules map[game.PhaseNameString]map[string]bool
}

func (rules *VoiceRules) GetVoiceState(isAlive, isTracked bool, phase game.Phase) (bool, bool) {
	if !isTracked {
		return false, false
	}
	aliveStr := "dead"
	if isAlive {
		aliveStr = "alive"
	}
	phaseStr := game.PhaseNames[phase]

	return rules.MuteRules[phaseStr][aliveStr], rules.DeafRules[phaseStr][aliveStr]
}

func MakeMuteAndDeafenRules() VoiceRules {
	rules := VoiceRules{
		MuteRules: map[game.PhaseNameString]map[string]bool{
			game.PhaseNames[game.LOBBY]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
			game.PhaseNames[game.TASKS]: map[string]bool{
				"alive": true,
				"dead":  false,
			},
			game.PhaseNames[game.DISCUSS]: map[string]bool{
				"alive": false,
				"dead":  true,
			},
		},
		DeafRules: map[game.PhaseNameString]map[string]bool{
			game.PhaseNames[game.LOBBY]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
			game.PhaseNames[game.TASKS]: map[string]bool{
				"alive": true,
				"dead":  false,
			},
			game.PhaseNames[game.DISCUSS]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
		},
	}
	return rules
}

func MakeMuteOnlyRules() VoiceRules {
	rules := VoiceRules{
		MuteRules: map[game.PhaseNameString]map[string]bool{
			game.PhaseNames[game.LOBBY]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
			game.PhaseNames[game.TASKS]: map[string]bool{
				"alive": true,
				"dead":  true,
			},
			game.PhaseNames[game.DISCUSS]: map[string]bool{
				"alive": false,
				"dead":  true,
			},
		},
		DeafRules: map[game.PhaseNameString]map[string]bool{
			game.PhaseNames[game.LOBBY]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
			game.PhaseNames[game.TASKS]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
			game.PhaseNames[game.DISCUSS]: map[string]bool{
				"alive": false,
				"dead":  false,
			},
		},
	}
	return rules
}
