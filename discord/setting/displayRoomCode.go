package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
)

func FnDisplayRoomCode(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(DisplayRoomCode)
	if sett == nil {
		return nil, false
	}
	if len(args) == 0 {
		return ConstructEmbedForSetting(fmt.Sprintf("%v", sett.GetDisplayRoomCode()), s, sett), false
	}

	val := strings.ToLower(args[0])
	valid := map[string]bool{"always": true, "spoiler": true, "never": true}
	if !valid[val] {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDisplayRoomCode.Unrecognized",
			Other: "{{.Arg}} is not an expected value. See `/settings display-room-code` for usage",
		},
			map[string]interface{}{
				"Arg": val,
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
