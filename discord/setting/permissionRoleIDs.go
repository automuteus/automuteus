package setting

import (
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FnPermissionRoleIDs(sett *settings.GuildSettings, args []string) (interface{}, bool) {
	s := GetSettingByName(RoleIDs)
	if sett == nil {
		return nil, false
	}
	oldRoleIDs := sett.GetPermissionRoleIDs()
	if len(args) == 0 {
		adminRoleCount := len(oldRoleIDs) // caching for optimisation
		// make a nicely formatted string of all the roles: "role1, role2, role3 and role4"
		if adminRoleCount == 0 {
			return ConstructEmbedForSetting(sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingPermissionRoleIDs.noRoleAdmins",
				Other: "No Role Admins",
			}), s, sett), false
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
			return ConstructEmbedForSetting(listOfRoles, s, sett), false
		}
	}

	if args[0] != "clear" && args[0] != "c" {
		roleName := args[0]
		ID, err := discord.ExtractRoleIDFromText(roleName)
		if err != nil {
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingPermissionRoleIDs.notFound",
				Other: "Sorry, I didn't recognize the role you provided",
			}), false
		}

		if ID != "" && !contains(oldRoleIDs, ID) {
			sett.SetPermissionRoleIDs(append(oldRoleIDs, ID))
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingPermissionRoleIDs.newBotOperator",
				Other: "I successfully added that role as bot operators!",
			}), true
		} else {
			return sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingPermissionRoleIDs.alreadyBotOperator",
				Other: "That role was already a bot operator!",
			}), false
		}
	} else {
		sett.SetPermissionRoleIDs([]string{})
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingPermissionRoleIDs.clearRoles",
			Other: "Clearing all PermissionRoleIDs!",
		}), true
	}
}
