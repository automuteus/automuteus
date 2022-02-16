package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
)

func FnMapVersion(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	if len(args) == 2 {
		return ConstructEmbedForSetting(fmt.Sprintf("%v", sett.GetMapVersion()), AllSettings[MapVersion], sett), false
	}

	val := strings.ToLower(args[2])
	valid := map[string]bool{"simple": true, "detailed": true}
	if !valid[val] {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMapVersion.Unrecognized",
			Other: "{{.Arg}} is not an expected value. See `{{.CommandPrefix}} settings mapversion` for usage",
		},
			map[string]interface{}{
				"Arg":           val,
				"CommandPrefix": sett.GetCommandPrefix(),
			}), false
	}

	sett.SetMapVersion(val)
	return sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingMapVersion.Success",
		Other: "From now on, I will display map images as {{.Arg}}",
	},
		map[string]interface{}{
			"Arg": val,
		}), true
}
