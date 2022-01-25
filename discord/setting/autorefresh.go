package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnAutoRefresh(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if len(args) == 2 {
		return ConstructEmbedForSetting(fmt.Sprintf("%v", sett.GetAutoRefresh()), AllSettings[AutoRefresh], sett), false
	}

	val := args[2]
	if val != "t" && val != "true" && val != "f" && val != "false" {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingAutoRefresh.Unrecognized",
			Other: "{{.Arg}} is not a true/false value. See `{{.CommandPrefix}} settings autorefresh` for usage",
		},
			map[string]interface{}{
				"Arg":           val,
				"CommandPrefix": sett.GetCommandPrefix(),
			}), false
	}

	newSet := val == "t" || val == "true"
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
