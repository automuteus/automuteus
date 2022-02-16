package setting

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnUnmuteDeadDuringTasks(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	unmuteDead := sett.GetUnmuteDeadDuringTasks()
	if len(args) == 2 {
		current := "false"
		if unmuteDead {
			current = "true"
		}
		return ConstructEmbedForSetting(current, AllSettings[UnmuteDead], sett), false
	}
	switch {
	case args[2] == "true":
		if unmuteDead {
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingUnmuteDeadDuringTasks.true_unmuteDead",
				Other: "It's already true!",
			}), false
		} else {
			sett.SetUnmuteDeadDuringTasks(true)
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingUnmuteDeadDuringTasks.true_noUnmuteDead",
				Other: "I will now unmute the dead people immediately after they die. Careful, this reveals who died during the match!",
			}), true
		}
	case args[2] == "false":
		if unmuteDead {
			sett.SetUnmuteDeadDuringTasks(false)
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingUnmuteDeadDuringTasks.false_unmuteDead",
				Other: "I will no longer immediately unmute dead people. Good choice!",
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
