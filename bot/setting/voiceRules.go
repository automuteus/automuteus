package setting

import (
	"github.com/j0nas500/automuteus/v8/pkg/game"
	"github.com/j0nas500/automuteus/v8/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnVoiceRules(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if sett == nil {
		return nil, false
	}

	// now for a bunch of input checking
	if len(args) < 3 {
		// User didn't pass enough args
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.enoughArgs",
			Other: "You didn't pass enough arguments! Correct syntax is: `voiceRules [muted/deafened] [game phase] [alive/dead] [true/false]`",
		}), false
	}

	gamePhase := game.GetPhaseFromString(args[1])
	if gamePhase == game.UNINITIALIZED {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.Phase.UNINITIALIZED",
			Other: "I don't know what {{.PhaseName}} is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.",
		},
			map[string]interface{}{
				"PhaseName": args[1],
			}), false
	}

	if args[2] != "alive" && args[2] != "dead" {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.neitherAliveDead",
			Other: "`{{.Arg}}` is neither `alive` or `dead`!",
		},
			map[string]interface{}{
				"Arg": args[2],
			}), false
	}

	oldValue := sett.GetVoiceRule(args[0] == "muted", gamePhase, args[2])

	if len(args) == 3 {
		// User was only querying
		if oldValue {
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingVoiceRules.queryingCurrentlyOldValues",
				Other: "When in `{{.PhaseName}}` phase, {{.PlayerGameState}} players are currently {{.PlayerDiscordState}}.",
			},
				map[string]interface{}{
					"PhaseName":          args[1],
					"PlayerGameState":    args[2],
					"PlayerDiscordState": args[0],
				}), false
		} else {
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingVoiceRules.queryingCurrentlyValues",
				Other: "When in `{{.PhaseName}}` phase, {{.PlayerGameState}} players are currently NOT {{.PlayerDiscordState}}.",
			},
				map[string]interface{}{
					"PhaseName":          args[1],
					"PlayerGameState":    args[2],
					"PlayerDiscordState": args[0],
				}), false
		}
	}
	newValue := args[3] == "true"

	if newValue == oldValue {
		if newValue {
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingVoiceRules.queryingAlreadyValues",
				Other: "When in `{{.PhaseName}}` phase, {{.PlayerGameState}} players are already {{.PlayerDiscordState}}!",
			},
				map[string]interface{}{
					"PhaseName":          args[1],
					"PlayerGameState":    args[2],
					"PlayerDiscordState": args[0],
				}), false
		} else {
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingVoiceRules.queryingAlreadyUnValues",
				Other: "When in `{{.PhaseName}}` phase, {{.PlayerGameState}} players are already un{{.PlayerDiscordState}}!",
			},
				map[string]interface{}{
					"PhaseName":          args[1],
					"PlayerGameState":    args[2],
					"PlayerDiscordState": args[0],
				}), false
		}
	}

	if args[0] == "muted" {
		sett.SetVoiceRule(true, gamePhase, args[2], newValue)
	} else {
		sett.SetVoiceRule(false, gamePhase, args[2], newValue)
	}

	if newValue {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.setValues",
			Other: "From now on, when in `{{.PhaseName}}` phase, {{.PlayerGameState}} players will be {{.PlayerDiscordState}}.",
		},
			map[string]interface{}{
				"PhaseName":          args[1],
				"PlayerGameState":    args[2],
				"PlayerDiscordState": args[0],
			}), true
	} else {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.setUnValues",
			Other: "From now on, when in `{{.PhaseName}}` phase, {{.PlayerGameState}} players will be un{{.PlayerDiscordState}}.",
		},
			map[string]interface{}{
				"PhaseName":          args[1],
				"PlayerGameState":    args[2],
				"PlayerDiscordState": args[0],
			}), true
	}
}
