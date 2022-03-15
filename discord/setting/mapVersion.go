package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
)

func FnMapVersion(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(MapVersion)
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	if len(args) == 2 {
		return ConstructEmbedForSetting(fmt.Sprintf("%T", sett.GetMapDetailed()), s, sett), false
	}

	val := strings.ToLower(args[2])
	sett.SetMapDetailed(val == "true")
	return sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingMapVersion.Success",
		Other: "From now on, map-detailed is `{{.Arg}}`",
	},
		map[string]interface{}{
			"Arg": val,
		}), true
}
