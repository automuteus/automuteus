package setting

import (
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnAdminUserIDs(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(AdminUserIDs)
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	adminIDs := sett.GetAdminUserIDs()
	if len(args) == 2 {
		adminCount := len(adminIDs) // caching for optimisation
		// make a nicely formatted string of all the admins: "user1, user2, user3 and user4"
		if adminCount == 0 {
			return ConstructEmbedForSetting(sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingAdminUserIDs.noBotAdmins",
				Other: "No Bot Admins",
			}), s, sett), false
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
			return ConstructEmbedForSetting(listOfAdmins, s, sett), false
		}
	}

	if args[2] != "clear" && args[2] != "c" {
		userName := args[2]
		ID, err := discord.ExtractUserIDFromMention(userName)
		if ID == "" || err != nil {
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingAdminUserIDs.notFound",
				Other: "Sorry, I don't know who `{{.UserName}}` is. You can pass in ID or @mention",
			},
				map[string]interface{}{
					"UserName": userName,
				}), false
		} else {
			oldIDs := sett.GetAdminUserIDs()
			if ID != "" && !contains(oldIDs, ID) {
				sett.SetAdminUserIDs(append(oldIDs, ID))
				return sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.SettingAdminUserIDs.newBotAdmin",
					Other: "<@{{.UserID}}> is now a bot admin!",
				},
					map[string]interface{}{
						"UserID": ID,
					}), true
			} else {
				return sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.SettingAdminUserIDs.alreadyBotAdmin",
					Other: "<@{{.UserID}}> was already a bot admin!",
				},
					map[string]interface{}{
						"UserID": ID,
					}), false
			}
		}

	} else {
		sett.SetAdminUserIDs([]string{})
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingAdminUserIDs.clearAdmins",
			Other: "Clearing all AdminUserIDs!",
		}), true
	}
}

func contains(arr []string, elem string) bool {
	for _, v := range arr {
		if v == elem {
			return true
		}
	}
	return false
}
