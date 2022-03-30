package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
	"strconv"
)

func FnLeaderboardMin(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(LeaderboardMin)
	if sett == nil {
		return nil, false
	}
	if len(args) == 0 {
		return ConstructEmbedForSetting(fmt.Sprintf("%d", sett.GetLeaderboardMin()), s, sett), false
	}

	num, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		log.Println("error for parseint in LeaderboardMin: ", err)
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardMin.Unrecognized",
			Other: "{{.Number}} is not a valid number. See `/settings leaderboard-min` for usage",
		},
			map[string]interface{}{
				"Number": args[0],
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
	if sett == nil {
		return nil, false
	}
	if len(args) == 0 {
		return ConstructEmbedForSetting(fmt.Sprintf("%v", sett.GetLeaderboardMention()), s, sett), false
	}

	val := args[0]

	newSet := val == "true"
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
	if sett == nil {
		return nil, false
	}
	if len(args) == 0 {
		return ConstructEmbedForSetting(fmt.Sprintf("%d", sett.GetLeaderboardSize()), s, sett), false
	}

	num, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		log.Println("error for parseint in LeaderboardSize: ", err)
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardSize.Unrecognized",
			Other: "{{.Number}} is not a valid number. See `/settings leaderboard-size` for usage",
		},
			map[string]interface{}{
				"Number": args[0],
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
