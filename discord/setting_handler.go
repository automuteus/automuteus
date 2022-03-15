package discord

import (
	"encoding/json"
	"fmt"
	"github.com/automuteus/automuteus/discord/setting"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"log"
)

func (bot *Bot) HandleSettingsCommand(guildID string, sett *settings.GuildSettings, args []string, prem bool) interface{} {
	if len(args) == 1 {
		return settingResponse(setting.AllSettings, sett, prem)
	}
	var sendMsg interface{}
	// if command invalid, no need to reapply changes to json file
	isValid := false

	settType := args[1]
	switch settType {
	case setting.Language:
		sendMsg, isValid = setting.FnLanguage(sett, args)
	case setting.AdminUserIDs:
		sendMsg, isValid = setting.FnAdminUserIDs(sett, args)
	case setting.RoleIDs:
		sendMsg, isValid = setting.FnPermissionRoleIDs(sett, args)
	case setting.UnmuteDead:
		sendMsg, isValid = setting.FnUnmuteDeadDuringTasks(sett, args)
	case setting.MapVersion:
		sendMsg, isValid = setting.FnMapVersion(sett, args)
	case setting.MatchSummary:
		if !prem {
			return nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnMatchSummary(sett, args)
	case setting.MatchSummaryChannel:
		if !prem {
			return nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnMatchSummaryChannel(sett, args)
	case setting.AutoRefresh:
		if !prem {
			return nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnAutoRefresh(sett, args)
	case setting.LeaderboardMention:
		if !prem {
			return nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnLeaderboardNameMention(sett, args)
	case setting.LeaderboardSize:
		if !prem {
			return nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnLeaderboardSize(sett, args)
	case setting.LeaderboardMin:
		if !prem {
			return nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnLeaderboardMin(sett, args)
	case setting.MuteSpectators:
		if !prem {
			return nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnMuteSpectators(sett, args)
	case setting.DisplayRoomCode:
		if !prem {
			return nonPremiumSettingResponse(sett)
		}
		sendMsg, isValid = setting.FnDisplayRoomCode(sett, args)
	case setting.Show:
		jBytes, err := json.MarshalIndent(sett, "", "  ")
		if err != nil {
			log.Println(err)
			return err
		}
		// TODO need to consider if the settings are too long? Is that possible?
		return fmt.Sprintf("```JSON\n%s\n```", jBytes)
	case setting.Reset:
		sett = settings.MakeGuildSettings()
		sendMsg = "Resetting guild settings to default values"
		isValid = true
	default:
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.HandleSettingsCommand.default",
			Other: "Sorry, `{{.Arg}}` is not a valid setting!\n",
		},
			map[string]interface{}{
				"Arg": args[1],
			})
	}

	if isValid {
		err := bot.StorageInterface.SetGuildSettings(guildID, sett)
		if err != nil {
			log.Println(err)
		}
	}
	return sendMsg
}
