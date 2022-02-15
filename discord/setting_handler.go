package discord

import (
	"encoding/json"
	"fmt"
	"github.com/automuteus/automuteus/discord/setting"
	"github.com/automuteus/utils/pkg/settings"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"log"
	"strings"
)

func getSetting(arg string) setting.SettingType {
	for _, set := range setting.AllSettings {
		if arg == strings.ToLower(set.Name) {
			return set.SettingType
		}

		for _, alias := range set.Aliases {
			if arg == strings.ToLower(alias) {
				return set.SettingType
			}
		}
	}
	return setting.NullSetting
}

func (bot *Bot) HandleSettingsCommand(m *discordgo.MessageCreate, sett *settings.GuildSettings, args []string, prem bool) (string, interface{}) {
	if len(args) == 1 {
		return m.ChannelID, settingResponse(sett.GetCommandPrefix(), setting.AllSettings, sett, prem)
	}
	var sendMsg interface{}
	// if command invalid, no need to reapply changes to json file
	isValid := false

	settType := getSetting(args[1])
	switch settType {
	case setting.Prefix:
		sendMsg, isValid = setting.FnCommandPrefix(sett, args)
	case setting.Language:
		sendMsg, isValid = setting.FnLanguage(sett, args)
	case setting.AdminUserIDs:
		sendMsg, isValid = setting.FnAdminUserIDs(sett, args)
	case setting.RoleIDs:
		sendMsg, isValid = setting.FnPermissionRoleIDs(sett, args)
	case setting.UnmuteDead:
		sendMsg, isValid = setting.FnUnmuteDeadDuringTasks(sett, args)
	case setting.Delays:
		sendMsg, isValid = setting.FnDelays(sett, args)
	case setting.VoiceRules:
		sendMsg, isValid = setting.FnVoiceRules(sett, args)
	case setting.MapVersion:
		sendMsg, isValid = setting.FnMapVersion(sett, args)
	case setting.MatchSummary:
		if !prem {
			return m.ChannelID, nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnMatchSummary(sett, args)
	case setting.MatchSummaryChannel:
		if !prem {
			return m.ChannelID, nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnMatchSummaryChannel(sett, args)
	case setting.AutoRefresh:
		if !prem {
			return m.ChannelID, nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnAutoRefresh(sett, args)
	case setting.LeaderboardMention:
		if !prem {
			return m.ChannelID, nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnLeaderboardNameMention(sett, args)
	case setting.LeaderboardSize:
		if !prem {
			return m.ChannelID, nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnLeaderboardSize(sett, args)
	case setting.LeaderboardMin:
		if !prem {
			return m.ChannelID, nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnLeaderboardMin(sett, args)
	case setting.MuteSpectators:
		if !prem {
			return m.ChannelID, nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnMuteSpectators(sett, args)
	case setting.DisplayRoomCode:
		if !prem {
			return m.ChannelID, nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnDisplayRoomCode(sett, args)
	case setting.Show:
		jBytes, err := json.MarshalIndent(sett, "", "  ")
		if err != nil {
			log.Println(err)
			return m.ChannelID, err
		}
		// TODO need to consider if the settings are too long? Is that possible?
		return m.ChannelID, fmt.Sprintf("```JSON\n%s\n```", jBytes)
	case setting.Reset:
		sett = settings.MakeGuildSettings(os.Getenv("AUTOMUTEUS_GLOBAL_PREFIX"), os.Getenv("AUTOMUTEUS_OFFICIAL") != "")
		sendMsg = "Resetting guild settings to default values"
		isValid = true
	default:
		return m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.HandleSettingsCommand.default",
			Other: "Sorry, `{{.Arg}}` is not a valid setting!\n",
		},
			map[string]interface{}{
				"Arg": args[1],
			})
	}

	// TODO do another check of validation for roleIDs and channelIDs here
	// we have the bot/discord scope to allow querying discord and making sure they're valid
	if isValid {
		err := bot.StorageInterface.SetGuildSettings(m.GuildID, sett)
		if err != nil {
			log.Println(err)
		}
	}
	return m.ChannelID, sendMsg
}
