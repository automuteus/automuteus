package setting

import (
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnPermissionRoleIDs(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	if sett == nil || len(args) < 2 {
		return nil, false
	}
	oldRoleIDs := sett.GetPermissionRoleIDs()
	if len(args) == 2 {
		adminRoleCount := len(oldRoleIDs) // caching for optimisation
		// make a nicely formatted string of all the roles: "role1, role2, role3 and role4"
		if adminRoleCount == 0 {
			return ConstructEmbedForSetting(sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingPermissionRoleIDs.noRoleAdmins",
				Other: "No Role Admins",
			}), AllSettings[RoleIDs], sett), false
		} else {
			listOfRoles := ""
			for index, ID := range oldRoleIDs {
				switch {
				case index == 0:
					listOfRoles += "<@&" + ID + ">"
				case index == adminRoleCount-1:
					listOfRoles += " and <@&" + ID + ">"
				default:
					listOfRoles += ", <@&" + ID + ">"
				}
			}
			return ConstructEmbedForSetting(listOfRoles, AllSettings[RoleIDs], sett), false
		}
	}

	if args[2] != "clear" && args[2] != "c" {
		var newRoleIDs []string
		// roles the User mentioned in their message
		var roleIDs []string
		var sendMessages []string

		for _, roleName := range args[2:] {
			if roleName == "" || roleName == " " {
				// User added a double space by accident, ignore it
				continue
			}
			ID, err := discord.ExtractRoleIDFromText(roleName)
			if err != nil {
				sendMessages = append(sendMessages, sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.SettingPermissionRoleIDs.notFound",
					Other: "Sorry, I don't know the role `{{.RoleName}}` is. Please use @role or the roleID",
				},
					map[string]interface{}{
						"RoleName": roleName,
					}))
				continue
			} else {
				roleIDs = append(roleIDs, ID)
			}
		}

		for _, ID := range roleIDs {
			if ID != "" {
				newRoleIDs = append(newRoleIDs, ID)
				sendMessages = append(sendMessages, sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.SettingPermissionRoleIDs.newBotAdmins",
					Other: "<@&{{.UserID}}>s are now bot admins!",
				},
					map[string]interface{}{
						"UserID": ID,
					}))
			}
		}
		if len(newRoleIDs) > 0 {
			sett.SetPermissionRoleIDs(newRoleIDs)
		}
		return sendMessages, len(newRoleIDs) > 0
	} else {
		sett.SetPermissionRoleIDs([]string{})
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingPermissionRoleIDs.clearRoles",
			Other: "Clearing all PermissionRoleIDs!",
		}), true
	}
}
