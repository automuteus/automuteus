package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
)

func FnMapVersion(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(MapVersion)
	if sett == nil {
		return nil, false
	}
	if len(args) == 0 {
		return ConstructEmbedForSetting(fmt.Sprintf("%t", sett.GetMapDetailed()), s, sett), false
	}

	val := strings.ToLower(args[0]) == "true"
	sett.SetMapDetailed(val)
	return sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingMapVersion.Success",
		Other: "From now on, detailed map setting is `{{.Arg}}`",
	},
		map[string]interface{}{
			"Arg": val,
		}), true
}
