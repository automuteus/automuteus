package setting

import (
	"github.com/j0nas500/automuteus/v8/pkg/game"
	"github.com/j0nas500/automuteus/v8/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strconv"
)

func FnDelays(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if sett == nil {
		return nil, false
	}
	// User passes phase name, phase name and new delay value
	if len(args) < 2 {
		// User didn't pass 2 phases, tell them the list of game phases
		return sett.LocalizeMessage(&i18n.Message{
			ID: "settings.SettingDelays.missingPhases",
			Other: "The list of game phases are `Lobby`, `Tasks` and `Discussion`.\n" +
				"You need to type both phases the game is transitioning from and to to change the delay.",
		}), false // find a better wording for this at some point
	}
	// now to find the actual game state from the string they passed
	var gamePhase1 = game.GetPhaseFromString(args[0])
	var gamePhase2 = game.GetPhaseFromString(args[1])
	if gamePhase1 == game.UNINITIALIZED {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDelays.Phase.UNINITIALIZED",
			Other: "I don't know what `{{.PhaseName}}` is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.",
		},
			map[string]interface{}{
				"PhaseName": args[0],
			}), false
	} else if gamePhase2 == game.UNINITIALIZED {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDelays.Phase.UNINITIALIZED",
			Other: "I don't know what `{{.PhaseName}}` is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.",
		},
			map[string]interface{}{
				"PhaseName": args[1],
			}), false
	}

	oldDelay := sett.GetDelay(gamePhase1, gamePhase2)
	if len(args) == 2 {
		// no number was passed, User was querying the delay
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDelays.delayBetweenPhases",
			Other: "Currently, the delay when passing from `{{.PhaseA}}` to `{{.PhaseB}}` is {{.OldDelay}}.",
		},
			map[string]interface{}{
				"PhaseA":   args[0],
				"PhaseB":   args[1],
				"OldDelay": oldDelay,
			}), false
	}

	newDelay, err := strconv.Atoi(args[2])
	if err != nil || newDelay < 0 {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDelays.wrongNumber",
			Other: "`{{.Number}}` is not a valid number! Please try again",
		},
			map[string]interface{}{
				"Number": args[2],
			}), false
	}

	sett.SetDelay(gamePhase1, gamePhase2, newDelay)
	return sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingDelays.setDelayBetweenPhases",
		Other: "The delay when passing from `{{.PhaseA}}` to `{{.PhaseB}}` changed from {{.OldDelay}} to {{.NewDelay}}.",
	},
		map[string]interface{}{
			"PhaseA":   args[0],
			"PhaseB":   args[1],
			"OldDelay": oldDelay,
			"NewDelay": newDelay,
		}), true
}
