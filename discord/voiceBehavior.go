package discord

import "github.com/denverquane/amongusdiscord/game"

type VoiceRules struct {
	muteRules   map[game.Phase]map[bool]bool
	deafenRules map[game.Phase]map[bool]bool
}

func (rules *VoiceRules) GetVoiceState(isAlive, isTracked bool, phase game.Phase) (bool, bool) {
	if !isTracked {
		return false, false
	}

	return rules.muteRules[phase][isAlive], rules.deafenRules[phase][isAlive]
}

func MakeMuteAndDeafenRules() VoiceRules {
	rules := VoiceRules{
		muteRules: map[game.Phase]map[bool]bool{
			game.LOBBY: map[bool]bool{
				true:  false,
				false: false,
			},
			game.TASKS: map[bool]bool{
				true:  true,
				false: false,
			},
			game.DISCUSS: map[bool]bool{
				true:  false,
				false: true,
			},
		},
		deafenRules: map[game.Phase]map[bool]bool{
			game.LOBBY: map[bool]bool{
				true:  false,
				false: false,
			},
			game.TASKS: map[bool]bool{
				true:  true,
				false: false,
			},
			game.DISCUSS: map[bool]bool{
				true:  false,
				false: false,
			},
		},
	}
	return rules
}

func MakeMuteOnlyRules() VoiceRules {
	rules := VoiceRules{
		muteRules: map[game.Phase]map[bool]bool{
			game.LOBBY: map[bool]bool{
				true:  false,
				false: false,
			},
			game.TASKS: map[bool]bool{
				true:  true,
				false: true,
			},
			game.DISCUSS: map[bool]bool{
				true:  false,
				false: true,
			},
		},
		deafenRules: map[game.Phase]map[bool]bool{
			game.LOBBY: map[bool]bool{
				true:  false,
				false: false,
			},
			game.TASKS: map[bool]bool{
				true:  false,
				false: false,
			},
			game.DISCUSS: map[bool]bool{
				true:  false,
				false: false,
			},
		},
	}
	return rules
}
