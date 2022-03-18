package setting

import (
	"github.com/automuteus/automuteus/amongus"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/settings"
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
			Other: "You didn't pass enough arguments! Correct syntax is: `voiceRules [mute/deaf] [game phase] [alive/dead] [true/false]`",
		}), false
	}

	switch {
	case args[0] == "deaf":
		args[0] = "deafened"
	case args[0] == "mute":
		args[0] = "muted"
	default:
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.neitherMuteDeaf",
			Other: "`{{.Arg}}` is neither `mute` nor `deaf`!",
		},
			map[string]interface{}{
				"Arg": args[0],
			}), false
	}

	gamePhase := amongus.GetPhaseFromString(args[1])
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

	var oldValue bool
	if args[2] == "muted" {
		oldValue = sett.GetVoiceRule(true, gamePhase, args[2])
	} else {
		oldValue = sett.GetVoiceRule(false, gamePhase, args[2])
	}

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

	var newValue bool
	switch {
	case args[3] == "true":
		newValue = true
	case args[3] == "false":
		newValue = false
	default:
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.neitherTrueFalse",
			Other: "`{{.Arg}}` is neither `true` or `false`!",
		},
			map[string]interface{}{
				"Arg": args[3],
			}), false
	}

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
