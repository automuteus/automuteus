package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
	"strconv"
)

func FnMatchSummary(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(MatchSummary)
	if sett == nil {
		return nil, false
	}
	if len(args) == 0 {
		return ConstructEmbedForSetting(fmt.Sprintf("%d", sett.GetDeleteGameSummaryMinutes()), s, sett), false
	}

	num, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		log.Println("error for parseint in MatchSummary: ", err)
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummary.Unrecognized",
			Other: "{{.Minutes}} is not a valid number. See `/settings match-summary` for usage",
		},
			map[string]interface{}{
				"Minutes": args[0],
			}), false
	}
	if num > 60 || num < -1 {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummary.OutOfRange",
			Other: "You provided a number too high or too low. Please specify a number between [0-60], or -1 to never delete match summaries",
		}), false
	}

	sett.SetDeleteGameSummaryMinutes(int(num))
	switch {
	case num == -1:
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummary.Success-1",
			Other: "From now on, I'll never delete match summary messages.",
		}), true
	case num == 0:
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummary.Success0",
			Other: "From now on, I'll delete match summary messages immediately.",
		}), true
	default:
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummary.Success",
			Other: "From now on, I'll delete match summary messages after {{.Minutes}} minutes.",
		},
			map[string]interface{}{
				"Minutes": num,
			}), true
	}
}

func FnMatchSummaryChannel(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(MatchSummaryChannel)
	if sett == nil {
		return nil, false
	}
	if len(args) == 0 {
		return ConstructEmbedForSetting(discord.MentionByChannelID(sett.GetMatchSummaryChannelID()), s, sett), false
	}

	channelID, err := discord.ExtractChannelIDFromText(args[0])
	if err != nil {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummaryChannel.invalidChannelID",
			Other: "{{.channelID}} is not a valid text channel ID or mention!",
		},
			map[string]interface{}{
				"channelID": args[0],
			}), false
	}

	sett.SetMatchSummaryChannelID(channelID)
	return sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingMatchSummaryChannel.withChannelID",
		Other: "Match Summary text channel ID changed to {{.channelID}}!",
	},
		map[string]interface{}{
			"channelID": discord.MentionByChannelID(channelID),
		}), true
}
