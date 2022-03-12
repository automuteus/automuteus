package setting

import (
	"fmt"
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strconv"
)

func FnMatchSummary(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	if len(args) == 2 {
		return ConstructEmbedForSetting(fmt.Sprintf("%d", sett.GetDeleteGameSummaryMinutes()), AllSettings[MatchSummary], sett), false
	}

	num, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummary.Unrecognized",
			Other: "{{.Minutes}} is not a valid number. See `{{.CommandPrefix}} settings matchSummary` for usage",
		},
			map[string]interface{}{
				"Minutes":       args[2],
				"CommandPrefix": sett.GetCommandPrefix(),
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
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	if len(args) == 2 {
		return ConstructEmbedForSetting(sett.GetMatchSummaryChannelID(), AllSettings[MatchSummaryChannel], sett), false
	}

	channelID, err := discord.ExtractChannelIDFromMention(args[2])
	if err != nil {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummaryChannel.invalidChannelID",
			Other: "{{.channelID}} is not a valid text channel ID or mention!",
		},
			map[string]interface{}{
				"channelID": args[2],
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
