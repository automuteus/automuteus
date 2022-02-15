package setting

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnMuteSpectators(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	muteSpec := sett.GetMuteSpectator()
	if len(args) == 2 {
		current := "false"
		if muteSpec {
			current = "true"
		}
		return ConstructEmbedForSetting(current, AllSettings[MuteSpectators], sett), false
	}
	switch {
	case args[2] == "true":
		if muteSpec {
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingUnmuteDeadDuringTasks.true_noUnmuteDead",
				Other: "It's already true!",
			}), false
		} else {
			sett.SetMuteSpectator(true)
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingMuteSpectators.true_noMuteSpectators",
				Other: "I will now mute spectators just like dead players. \n**Note, this can cause delays or slowdowns when not self-hosting, or using a Premium worker bot!**",
			}), true
		}
	case args[2] == "false":
		if muteSpec {
			sett.SetMuteSpectator(false)
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingMuteSpectators.false_muteSpectators",
				Other: "I will no longer mute spectators like dead players",
			}), true
		}
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingUnmuteDeadDuringTasks.false_noUnmuteDead",
			Other: "It's already false!",
		}), false
	default:
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingUnmuteDeadDuringTasks.wrongArg",
			Other: "Sorry, `{{.Arg}}` is neither `true` nor `false`.",
		},
			map[string]interface{}{
				"Arg": args[2],
			}), false
	}
}
