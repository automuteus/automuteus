package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnAutoRefresh(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(AutoRefresh)
	if sett == nil {
		return nil, false
	}
	if len(args) == 0 {
		return ConstructEmbedForSetting(fmt.Sprintf("%v", sett.GetAutoRefresh()), s, sett), false
	}

	val := args[0]
	if val != "t" && val != "true" && val != "f" && val != "false" {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingAutoRefresh.Unrecognized",
			Other: "{{.Arg}} is not a true/false value. See `/settings auto-refresh` for usage",
		},
			map[string]interface{}{
				"Arg": val,
			}), false
	}

	newSet := val == "t" || val == "true"
	if sett.GetAutoRefresh() == newSet {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingAutoRefresh.Noop",
			Other: "AutoRefresh was already set to `{{.Value}}`; not doing anything",
		},
			map[string]interface{}{
				"Value": newSet,
			}), false
	}
	sett.SetAutoRefresh(newSet)
	if newSet {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingAutoRefresh.True",
			Other: "From now on, I'll AutoRefresh the game status message",
		}), true
	} else {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingAutoRefresh.False",
			Other: "From now on, I will not AutoRefresh the game status message",
		}), true
	}
}
