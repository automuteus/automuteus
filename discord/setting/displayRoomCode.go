package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
)

func FnDisplayRoomCode(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	if len(args) == 2 {
		return ConstructEmbedForSetting(fmt.Sprintf("%v", sett.GetDisplayRoomCode()), AllSettings[DisplayRoomCode], sett), false
	}

	val := strings.ToLower(args[2])
	valid := map[string]bool{"always": true, "spoiler": true, "never": true}
	if !valid[val] {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDisplayRoomCode.Unrecognized",
			Other: "{{.Arg}} is not an expected value. See `{{.CommandPrefix}} settings displayRoomCode` for usage",
		},
			map[string]interface{}{
				"Arg":           val,
				"CommandPrefix": sett.GetCommandPrefix(),
			}), false
	}

	sett.SetDisplayRoomCode(val)
	if val == "spoiler" {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDisplayRoomCode.Spoiler",
			Other: "From now on, I will mark the room code as spoiler in the message",
		},
			map[string]interface{}{
				"Arg": val,
			}), true
	} else {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDisplayRoomCode.AlwaysOrNever",
			Other: "From now on, I will {{.Arg}} display the room code in the message",
		},
			map[string]interface{}{
				"Arg": val,
			}), true
	}
}
