package setting

import (
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnAdminUserIDs(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(AdminUserIDs)
	if sett == nil {
		return nil, false
	}
	adminIDs := sett.GetAdminUserIDs()
	if len(args) == 0 || args[0] == View {
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
					listOfAdmins += discord.MentionByUserID(ID)
				case index == adminCount-1:
					listOfAdmins += " and " + discord.MentionByUserID(ID)
				default:
					listOfAdmins += ", " + discord.MentionByUserID(ID)
				}
			}
			return ConstructEmbedForSetting(listOfAdmins, s, sett), false
		}
	}

	if args[0] != Clear && args[0] != "c" {
		userName := args[0]
		ID, err := discord.ExtractUserIDFromText(userName)
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
					Other: "{{.User}} is now a bot admin!",
				},
					map[string]interface{}{
						"User": discord.MentionByUserID(ID),
					}), true
			} else {
				return sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.SettingAdminUserIDs.alreadyBotAdmin",
					Other: "{{.User}} was already a bot admin!",
				},
					map[string]interface{}{
						"User": discord.MentionByUserID(ID),
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
