package setting

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/denverquane/amongusdiscord/common"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnAdminUserIDs(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	adminIDs := sett.GetAdminUserIDs()
	if len(args) == 2 {
		adminCount := len(adminIDs) // caching for optimisation
		// make a nicely formatted string of all the admins: "user1, user2, user3 and user4"
		if adminCount == 0 {
			return ConstructEmbedForSetting(sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingAdminUserIDs.noBotAdmins",
				Other: "No Bot Admins",
			}), AllSettings[AdminUserIDs], sett), false
		} else {
			listOfAdmins := ""
			for index, ID := range adminIDs {
				switch {
				case index == 0:
					listOfAdmins += "<@" + ID + ">"
				case index == adminCount-1:
					listOfAdmins += " and <@" + ID + ">"
				default:
					listOfAdmins += ", <@" + ID + ">"
				}
			}
			return ConstructEmbedForSetting(listOfAdmins, AllSettings[AdminUserIDs], sett), false
		}
	}
	var newAdminIDs []string
	// users the User mentioned in their message
	var userIDs []string

	if args[2] != "clear" && args[2] != "c" {
		var sendMessages []string
		for _, userName := range args[2:] {
			if userName == "" || userName == " " {
				// User added a double space by accident, ignore it
				continue
			}
			ID, err := common.ExtractUserIDFromMention(userName)
			if ID == "" || err != nil {
				sendMessages = append(sendMessages, sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.SettingAdminUserIDs.notFound",
					Other: "Sorry, I don't know who `{{.UserName}}` is. You can pass in ID, username, username#XXXX, nickname or @mention",
				},
					map[string]interface{}{
						"UserName": userName,
					}))
				continue
			}
			userIDs = append(userIDs, ID)
		}

		for _, ID := range userIDs {
			if ID != "" {
				newAdminIDs = append(newAdminIDs, ID)
				sendMessages = append(sendMessages, sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.SettingAdminUserIDs.newBotAdmin",
					Other: "<@{{.UserID}}> is now a bot admin!",
				},
					map[string]interface{}{
						"UserID": ID,
					}))
			}
		}
		sett.SetAdminUserIDs(newAdminIDs)
		return sendMessages, true
	} else {
		sett.SetAdminUserIDs(newAdminIDs)
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingAdminUserIDs.clearAdmins",
			Other: "Clearing all AdminUserIDs!",
		}), true
	}
}
