package setting

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strconv"
)

func FnMatchSummaryChannel(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if len(args) == 2 {
		return ConstructEmbedForSetting(sett.GetMatchSummaryChannelID(), AllSettings[MatchSummaryChannel], sett), false
	}

	channelID := args[2]
	// TODO snowflake validation
	_, err := strconv.ParseInt(channelID, 10, 64)
	if err != nil {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummaryChannel.invalidChannelID",
			Other: "`{{.channelID}}` is not a valid channel ID!",
		},
			map[string]interface{}{
				"channelID": args[2],
			}), false
	}

	sett.SetMatchSummaryChannelID(channelID)
	return sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingMatchSummaryChannel.withChannelID",
		Other: "Match Summary text channel ID changed to `{{.channelID}}`!",
	},
		map[string]interface{}{
			"channelName": channelID,
		}), true
}
