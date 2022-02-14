package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strconv"
)

func FnLeaderboardSize(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if len(args) == 2 {
		return ConstructEmbedForSetting(fmt.Sprintf("%d", sett.GetLeaderboardSize()), AllSettings[LeaderboardSize], sett), false
	}

	num, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardSize.Unrecognized",
			Other: "{{.Number}} is not a valid number. See `{{.CommandPrefix}} settings leaderboardSize` for usage",
		},
			map[string]interface{}{
				"Number":        args[2],
				"CommandPrefix": sett.GetCommandPrefix(),
			}), false
	}
	if num > 10 || num < 1 {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardSize.OutOfRange",
			Other: "You provided a number too high or too low. Please specify a number between [1-10]",
		}), false
	}

	sett.SetLeaderboardSize(int(num))

	return sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingLeaderboardSize.Success",
		Other: "From now on, I'll display {{.Players}} players on the leaderboard",
	},
		map[string]interface{}{
			"Players": num,
		}), true
}
