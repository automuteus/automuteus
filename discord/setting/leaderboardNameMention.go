package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnLeaderboardNameMention(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if len(args) == 2 {
		return ConstructEmbedForSetting(fmt.Sprintf("%v", sett.GetLeaderboardMention()), AllSettings[LeaderboardMention], sett), false
	}

	val := args[2]
	if val != "t" && val != "true" && val != "f" && val != "false" {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardMention.Unrecognized",
			Other: "{{.Arg}} is not a true/false value. See `{{.CommandPrefix}} settings leaderboardMention` for usage",
		},
			map[string]interface{}{
				"Arg":           val,
				"CommandPrefix": sett.GetCommandPrefix(),
			}), false
	}

	newSet := val == "t" || val == "true"
	sett.SetLeaderboardMention(newSet)
	if newSet {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardMention.True",
			Other: "From now on, I'll mention players directly in the leaderboard",
		}), true
	} else {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardMention.False",
			Other: "From now on, I'll use player nicknames/usernames in the leaderboard",
		}), true
	}
}
