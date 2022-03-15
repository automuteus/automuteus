package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strconv"
)

func FnLeaderboardMin(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(LeaderboardMin)
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	if len(args) == 2 {
		return ConstructEmbedForSetting(fmt.Sprintf("%d", sett.GetLeaderboardMin()), s, sett), false
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

func FnLeaderboardNameMention(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(LeaderboardMention)
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	if len(args) == 2 {
		return ConstructEmbedForSetting(fmt.Sprintf("%v", sett.GetLeaderboardMention()), s, sett), false
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

func FnLeaderboardSize(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(LeaderboardSize)
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	if len(args) == 2 {
		return ConstructEmbedForSetting(fmt.Sprintf("%d", sett.GetLeaderboardSize()), s, sett), false
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
