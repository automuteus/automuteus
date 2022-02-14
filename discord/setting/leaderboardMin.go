package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strconv"
)

func FnLeaderboardMin(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if len(args) == 2 {
		return ConstructEmbedForSetting(fmt.Sprintf("%d", sett.GetLeaderboardMin()), AllSettings[LeaderboardMin], sett), false
	}

	num, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardMin.Unrecognized",
			Other: "{{.Number}} is not a valid number. See `{{.CommandPrefix}} settings leaderboardMin` for usage",
		},
			map[string]interface{}{
				"Number":        args[2],
				"CommandPrefix": sett.GetCommandPrefix(),
			}), false
	}
	if num > 100 || num < 1 {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardMin.OutOfRange",
			Other: "You provided a number too high or too low. Please specify a number between [1-100]",
		}), false
	}

	sett.SetLeaderboardMin(int(num))

	return sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingLeaderboardMin.Success",
		Other: "From now on, I'll display only players with {{.Games}}+ qualifying games on the leaderboard",
	},
		map[string]interface{}{
			"Games": num,
		}), true
}
