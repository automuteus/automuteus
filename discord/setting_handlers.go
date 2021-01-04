package discord

import (
	"encoding/json"
	"fmt"
	"github.com/automuteus/utils/pkg/game"
	"github.com/denverquane/amongusdiscord/discord/setting"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/locale"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"log"
	"strconv"
	"strings"
)

func ConstructEmbedForSetting(value string, setting setting.Setting, sett *storage.GuildSettings) discordgo.MessageEmbed {
	title := setting.Name
	if setting.Premium {
		title = "💎 " + title
	}
	if value == "" {
		value = "null"
	}

	desc := sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.ConstructEmbedForSetting.StarterDesc",
		Other: "Type `{{.CommandPrefix}} settings {{.Command}}` to change this setting.\n\n",
	}, map[string]interface{}{
		"CommandPrefix": sett.GetCommandPrefix(),
		"Command":       setting.Name,
	})
	return discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       setting.Name,
		Description: desc + sett.LocalizeMessage(setting.Description),
		Timestamp:   "",
		Color:       15844367, // GOLD
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.ConstructEmbedForSetting.Fields.CurrentValue",
					Other: "Current Value",
				}),
				Value:  value,
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.ConstructEmbedForSetting.Fields.Example",
					Other: "Example",
				}),
				Value:  "`" + setting.Example + "`",
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.ConstructEmbedForSetting.Fields.Arguments",
					Other: "Arguments",
				}),
				Value:  "`" + sett.LocalizeMessage(setting.Arguments) + "`",
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.ConstructEmbedForSetting.Fields.Aliases",
					Other: "Aliases",
				}),
				Value:  strings.Join(setting.Aliases, ", "),
				Inline: false,
			},
		},
	}
}

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

func (bot *Bot) HandleSettingsCommand(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string, prem bool) {
	if len(args) == 1 {
		s.ChannelMessageSendEmbed(m.ChannelID, settingResponse(sett.CommandPrefix, setting.AllSettings, sett, prem))
		return
	}
	// if command invalid, no need to reapply changes to json file
	isValid := false

	settType := getSetting(args[1])
	switch settType {
	case setting.Prefix:
		isValid = CommandPrefixSetting(s, m, sett, args)
	case setting.Language:
		isValid = SettingLanguage(s, m, sett, args)
	case setting.AdminUserIDs:
		isValid = SettingAdminUserIDs(s, m, sett, args)
	case setting.RoleIDs:
		isValid = SettingPermissionRoleIDs(s, m, sett, args)
	case setting.UnmuteDead:
		isValid = SettingUnmuteDeadDuringTasks(s, m, sett, args)
	case setting.Delays:
		isValid = SettingDelays(s, m, sett, args)
	case setting.VoiceRules:
		isValid = SettingVoiceRules(s, m, sett, args)
	case setting.MapVersion:
		isValid = SettingMapVersion(s, m, sett, args)
	case setting.MatchSummary:
		if !prem {
			s.ChannelMessageSend(m.ChannelID, nonPremiumSettingResponse(sett))
			break
		}
		isValid = SettingMatchSummary(s, m, sett, args)
	case setting.MatchSummaryChannel:
		if !prem {
			s.ChannelMessageSend(m.ChannelID, nonPremiumSettingResponse(sett))
			break
		}
		isValid = SettingMatchSummaryChannel(s, m, sett, args)
	case setting.AutoRefresh:
		if !prem {
			s.ChannelMessageSend(m.ChannelID, nonPremiumSettingResponse(sett))
			break
		}
		isValid = SettingAutoRefresh(s, m, sett, args)
	case setting.LeaderboardMention:
		if !prem {
			s.ChannelMessageSend(m.ChannelID, nonPremiumSettingResponse(sett))
			break
		}
		isValid = SettingLeaderboardNameMention(s, m, sett, args)
	case setting.LeaderboardSize:
		if !prem {
			s.ChannelMessageSend(m.ChannelID, nonPremiumSettingResponse(sett))
			break
		}
		isValid = SettingLeaderboardSize(s, m, sett, args)
	case setting.LeaderboardMin:
		if !prem {
			s.ChannelMessageSend(m.ChannelID, nonPremiumSettingResponse(sett))
			break
		}
		isValid = SettingLeaderboardMin(s, m, sett, args)
	case setting.MuteSpectators:
		if !prem {
			s.ChannelMessageSend(m.ChannelID, nonPremiumSettingResponse(sett))
			break
		}
		isValid = SettingMuteSpectators(s, m, sett, args)
	case setting.Show:
		jBytes, err := json.MarshalIndent(sett, "", "  ")
		if err != nil {
			log.Println(err)
			return
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```JSON\n%s\n```", jBytes))
	case setting.Reset:
		sett = storage.MakeGuildSettings()
		s.ChannelMessageSend(m.ChannelID, "Resetting guild settings to default values")
		isValid = true
	default:
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.HandleSettingsCommand.default",
			Other: "Sorry, `{{.Arg}}` is not a valid setting!\n",
		},
			map[string]interface{}{
				"Arg": args[1],
			}))
	}

	if isValid {
		err := bot.StorageInterface.SetGuildSettings(m.GuildID, sett)
		if err != nil {
			log.Println(err)
		}
	}
}

func CommandPrefixSetting(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(sett.GetCommandPrefix(), setting.AllSettings[setting.Prefix], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}
	if len(args[2]) > 10 {
		// prevent someone from setting something ridiculous lol
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.CommandPrefixSetting.tooLong",
			Other: "Sorry, the prefix `{{.CommandPrefix}}` is too long ({{.Length}} characters, max 10). Try something shorter.",
		},
			map[string]interface{}{
				"CommandPrefix": args[2],
				"Length":        len(args[2]),
			}))
		return false
	}

	s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.CommandPrefixSetting.changes",
		Other: "Guild prefix changed from `{{.From}}` to `{{.To}}`. Use that from now on!",
	},
		map[string]interface{}{
			"From": sett.GetCommandPrefix(),
			"To":   args[2],
		}))

	sett.SetCommandPrefix(args[2])
	return true
}

func SettingLanguage(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(sett.GetLanguage(), setting.AllSettings[setting.Language], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}

	/* strLangs := ""
	for id, data := range locale.GetLanguages() {
		strLangs += id + " - " + data + "\n"
	}
	println(strLangs) */

	if args[2] == "reload" {
		locale.LoadTranslations()

		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLanguage.reloaded",
			Other: "Localization files are reloaded ({{.Count}}). Available language codes: {{.Langs}}",
		},
			map[string]interface{}{
				"Langs": locale.GetBundle().LanguageTags(),
				"Count": len(locale.GetBundle().LanguageTags()),
			}))
		return false
	} else if args[2] == "list" {
		// locale.LoadTranslations()

		strLangs := ""
		for langCode, langName := range locale.GetLanguages() {
			strLangs += fmt.Sprintf("\n[%s] - %s", langCode, langName)
		}

		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLanguage.list",
			Other: "Available languages: {{.Langs}}",
		},
			map[string]interface{}{
				"Langs": strLangs,
			}))
		return false
	}

	if len(args[2]) < 2 {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLanguage.tooShort",
			Other: "Sorry, the language code is short. Available language codes: {{.Langs}}.",
		},
			map[string]interface{}{
				"Langs": locale.GetBundle().LanguageTags(),
			}))
		return false
	}

	if len(locale.GetBundle().LanguageTags()) < 2 {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLanguage.notLoaded",
			Other: "Localization files were not loaded! {{.Langs}}",
		},
			map[string]interface{}{
				"Langs": locale.GetBundle().LanguageTags(),
			}))

		return false
	}

	langName := locale.GetLanguages()[args[2]]
	if langName == "" {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLanguage.notFound",
			Other: "Language not found! Available language codes: {{.Langs}}",
		},
			map[string]interface{}{
				"Langs": locale.GetBundle().LanguageTags(),
			}))

		return false
	}

	sett.SetLanguage(args[2])

	s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingLanguage.set",
		Other: "Localization is set to {{.LangName}}",
	},
		map[string]interface{}{
			"LangName": langName,
		}))
	return true
}

func SettingAdminUserIDs(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	adminIDs := sett.GetAdminUserIDs()
	if len(args) == 2 {
		adminCount := len(adminIDs) // caching for optimisation
		// make a nicely formatted string of all the admins: "user1, user2, user3 and user4"
		if adminCount == 0 {
			embed := ConstructEmbedForSetting(sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingAdminUserIDs.noBotAdmins",
				Other: "No Bot Admins",
			}), setting.AllSettings[setting.AdminUserIDs], sett)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
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
			embed := ConstructEmbedForSetting(listOfAdmins, setting.AllSettings[setting.AdminUserIDs], sett)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		}
		return false
	}
	newAdminIDs := []string{}
	// users the User mentioned in their message
	var userIDs []string

	if args[2] != "clear" && args[2] != "c" {
		for _, userName := range args[2:] {
			if userName == "" || userName == " " {
				// User added a double space by accident, ignore it
				continue
			}
			ID, err := extractUserIDFromMention(userName)
			if ID == "" || err != nil {
				s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
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
				// mention User without pinging
				s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
					Content: sett.LocalizeMessage(&i18n.Message{
						ID:    "settings.SettingAdminUserIDs.newBotAdmin",
						Other: "<@{{.UserID}}> is now a bot admin!",
					},
						map[string]interface{}{
							"UserID": ID,
						}),
					AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
				})
			}
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingAdminUserIDs.clearAdmins",
			Other: "Clearing all AdminUserIDs!",
		}))
	}

	sett.SetAdminUserIDs(newAdminIDs)
	return true
}

func SettingPermissionRoleIDs(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	oldRoleIDs := sett.GetPermissionRoleIDs()
	if len(args) == 2 {
		adminRoleCount := len(oldRoleIDs) // caching for optimisation
		// make a nicely formatted string of all the roles: "role1, role2, role3 and role4"
		if adminRoleCount == 0 {
			embed := ConstructEmbedForSetting(sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingPermissionRoleIDs.noRoleAdmins",
				Other: "No Role Admins",
			}), setting.AllSettings[setting.RoleIDs], sett)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
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
			embed := ConstructEmbedForSetting(listOfRoles, setting.AllSettings[setting.RoleIDs], sett)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		}
		return false
	}

	newRoleIDs := []string{}
	// roles the User mentioned in their message
	var roleIDs []string

	if args[2] != "clear" && args[2] != "c" {
		for _, roleName := range args[2:] {
			if roleName == "" || roleName == " " {
				// User added a double space by accident, ignore it
				continue
			}
			ID := getRoleFromString(s, m.GuildID, roleName)
			if ID == "" {
				s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
					ID:    "settings.SettingPermissionRoleIDs.notFound",
					Other: "Sorry, I don't know the role `{{.RoleName}}` is. Please use @role",
				},
					map[string]interface{}{
						"RoleName": roleName,
					}))
				continue
			}
			roleIDs = append(roleIDs, ID)
		}

		for _, ID := range roleIDs {
			if ID != "" {
				newRoleIDs = append(newRoleIDs, ID)
				// mention User without pinging
				s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
					Content: sett.LocalizeMessage(&i18n.Message{
						ID:    "settings.SettingPermissionRoleIDs.newBotAdmins",
						Other: "<@&{{.UserID}}>s are now bot admins!",
					},
						map[string]interface{}{
							"UserID": ID,
						}),
					AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
				})
			}
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingPermissionRoleIDs.clearRoles",
			Other: "Clearing all PermissionRoleIDs!",
		}))
	}

	sett.SetPermissionRoleIDs(newRoleIDs)
	return true
}

func SettingUnmuteDeadDuringTasks(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	unmuteDead := sett.GetUnmuteDeadDuringTasks()
	if len(args) == 2 {
		current := "false"
		if unmuteDead {
			current = "true"
		}
		embed := ConstructEmbedForSetting(current, setting.AllSettings[setting.UnmuteDead], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}
	switch {
	case args[2] == "true":
		if unmuteDead {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingUnmuteDeadDuringTasks.true_unmuteDead",
				Other: "It's already true!",
			}))
		} else {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingUnmuteDeadDuringTasks.true_noUnmuteDead",
				Other: "I will now unmute the dead people immediately after they die. Careful, this reveals who died during the match!",
			}))
			sett.SetUnmuteDeadDuringTasks(true)
			return true
		}
	case args[2] == "false":
		if unmuteDead {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingUnmuteDeadDuringTasks.false_unmuteDead",
				Other: "I will no longer immediately unmute dead people. Good choice!",
			}))
			sett.SetUnmuteDeadDuringTasks(false)
			return true
		}
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingUnmuteDeadDuringTasks.false_noUnmuteDead",
			Other: "It's already false!",
		}))
	default:
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingUnmuteDeadDuringTasks.wrongArg",
			Other: "Sorry, `{{.Arg}}` is neither `true` nor `false`.",
		},
			map[string]interface{}{
				"Arg": args[2],
			}))
	}
	return false
}

func SettingDelays(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting("N/A", setting.AllSettings[setting.Delays], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}
	// User passes phase name, phase name and new delay value
	if len(args) < 4 {
		// User didn't pass 2 phases, tell them the list of game phases
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID: "settings.SettingDelays.missingPhases",
			Other: "The list of game phases are `Lobby`, `Tasks` and `Discussion`.\n" +
				"You need to type both phases the game is transitioning from and to to change the delay.",
		})) // find a better wording for this at some point
		return false
	}
	// now to find the actual game state from the string they passed
	var gamePhase1 = getPhaseFromString(args[2])
	var gamePhase2 = getPhaseFromString(args[3])
	if gamePhase1 == game.UNINITIALIZED {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDelays.Phase.UNINITIALIZED",
			Other: "I don't know what `{{.PhaseName}}` is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.",
		},
			map[string]interface{}{
				"PhaseName": args[2],
			}))
		return false
	} else if gamePhase2 == game.UNINITIALIZED {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDelays.Phase.UNINITIALIZED",
			Other: "I don't know what `{{.PhaseName}}` is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.",
		},
			map[string]interface{}{
				"PhaseName": args[3],
			}))
		return false
	}

	oldDelay := sett.GetDelay(gamePhase1, gamePhase2)
	if len(args) == 4 {
		// no number was passed, User was querying the delay
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDelays.delayBetweenPhases",
			Other: "Currently, the delay when passing from `{{.PhaseA}}` to `{{.PhaseB}}` is {{.OldDelay}}.",
		},
			map[string]interface{}{
				"PhaseA":   args[2],
				"PhaseB":   args[3],
				"OldDelay": oldDelay,
			}))
		return false
	}

	newDelay, err := strconv.Atoi(args[4])
	if err != nil || newDelay < 0 {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingDelays.wrongNumber",
			Other: "`{{.Number}}` is not a valid number! Please try again",
		},
			map[string]interface{}{
				"Number": args[4],
			}))
		return false
	}

	sett.SetDelay(gamePhase1, gamePhase2, newDelay)
	s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingDelays.setDelayBetweenPhases",
		Other: "The delay when passing from `{{.PhaseA}}` to `{{.PhaseB}}` changed from {{.OldDelay}} to {{.NewDelay}}.",
	},
		map[string]interface{}{
			"PhaseA":   args[2],
			"PhaseB":   args[3],
			"OldDelay": oldDelay,
			"NewDelay": newDelay,
		}))
	return true
}

func SettingVoiceRules(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.NA",
			Other: "N/A",
		}), setting.AllSettings[setting.VoiceRules], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}

	// now for a bunch of input checking
	if len(args) < 5 {
		// User didn't pass enough args
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.enoughArgs",
			Other: "You didn't pass enough arguments! Correct syntax is: `voiceRules [mute/deaf] [game phase] [alive/dead] [true/false]`",
		}))
		return false
	}

	switch {
	case args[2] == "deaf":
		args[2] = "deafened"
	case args[2] == "mute":
		args[2] = "muted"
	default:
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.neitherMuteDeaf",
			Other: "`{{.Arg}}` is neither `mute` nor `deaf`!",
		},
			map[string]interface{}{
				"Arg": args[2],
			}))
		return false
	}

	gamePhase := getPhaseFromString(args[3])
	if gamePhase == game.UNINITIALIZED {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.Phase.UNINITIALIZED",
			Other: "I don't know what {{.PhaseName}} is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.",
		},
			map[string]interface{}{
				"PhaseName": args[3],
			}))
		return false
	}

	if args[4] != "alive" && args[4] != "dead" {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.neitherAliveDead",
			Other: "`{{.Arg}}` is neither `alive` or `dead`!",
		},
			map[string]interface{}{
				"Arg": args[4],
			}))
		return false
	}

	var oldValue bool
	if args[2] == "muted" {
		oldValue = sett.GetVoiceRule(true, gamePhase, args[4])
	} else {
		oldValue = sett.GetVoiceRule(false, gamePhase, args[4])
	}

	if len(args) == 5 {
		// User was only querying
		if oldValue {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingVoiceRules.queryingCurrentlyOldValues",
				Other: "When in `{{.PhaseName}}` phase, {{.PlayerGameState}} players are currently {{.PlayerDiscordState}}.",
			},
				map[string]interface{}{
					"PhaseName":          args[3],
					"PlayerGameState":    args[4],
					"PlayerDiscordState": args[2],
				}))
		} else {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingVoiceRules.queryingCurrentlyValues",
				Other: "When in `{{.PhaseName}}` phase, {{.PlayerGameState}} players are currently NOT {{.PlayerDiscordState}}.",
			},
				map[string]interface{}{
					"PhaseName":          args[3],
					"PlayerGameState":    args[4],
					"PlayerDiscordState": args[2],
				}))
		}
		return false
	}

	var newValue bool
	switch {
	case args[5] == "true":
		newValue = true
	case args[5] == "false":
		newValue = false
	default:
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.neitherTrueFalse",
			Other: "`{{.Arg}}` is neither `true` or `false`!",
		},
			map[string]interface{}{
				"Arg": args[5],
			}))
		return false
	}

	if newValue == oldValue {
		if newValue {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingVoiceRules.queryingAlreadyValues",
				Other: "When in `{{.PhaseName}}` phase, {{.PlayerGameState}} players are already {{.PlayerDiscordState}}!",
			},
				map[string]interface{}{
					"PhaseName":          args[3],
					"PlayerGameState":    args[4],
					"PlayerDiscordState": args[2],
				}))
		} else {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingVoiceRules.queryingAlreadyUnValues",
				Other: "When in `{{.PhaseName}}` phase, {{.PlayerGameState}} players are already un{{.PlayerDiscordState}}!",
			},
				map[string]interface{}{
					"PhaseName":          args[3],
					"PlayerGameState":    args[4],
					"PlayerDiscordState": args[2],
				}))
		}
		return false
	}

	if args[2] == "muted" {
		sett.SetVoiceRule(true, gamePhase, args[4], newValue)
	} else {
		sett.SetVoiceRule(false, gamePhase, args[4], newValue)
	}

	if newValue {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.setValues",
			Other: "From now on, when in `{{.PhaseName}}` phase, {{.PlayerGameState}} players will be {{.PlayerDiscordState}}.",
		},
			map[string]interface{}{
				"PhaseName":          args[3],
				"PlayerGameState":    args[4],
				"PlayerDiscordState": args[2],
			}))
	} else {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingVoiceRules.setUnValues",
			Other: "From now on, when in `{{.PhaseName}}` phase, {{.PlayerGameState}} players will be un{{.PlayerDiscordState}}.",
		},
			map[string]interface{}{
				"PhaseName":          args[3],
				"PlayerGameState":    args[4],
				"PlayerDiscordState": args[2],
			}))
	}
	return true
}

func SettingMatchSummary(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(fmt.Sprintf("%d", sett.GetDeleteGameSummaryMinutes()), setting.AllSettings[setting.MatchSummary], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}

	num, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummary.Unrecognized",
			Other: "{{.Minutes}} is not a valid number. See `{{.CommandPrefix}} settings matchSummary` for usage",
		},
			map[string]interface{}{
				"Minutes":       args[2],
				"CommandPrefix": sett.CommandPrefix,
			}))
		return false
	}
	if num > 60 || num < -1 {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummary.OutOfRange",
			Other: "You provided a number too high or too low. Please specify a number between [0-60], or -1 to never delete match summaries",
		}))
		return false
	}

	sett.SetDeleteGameSummaryMinutes(int(num))
	switch {
	case num == -1:
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummary.Success-1",
			Other: "From now on, I'll never delete match summary messages.",
		}))
	case num == 0:
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummary.Success0",
			Other: "From now on, I'll delete match summary messages immediately.",
		}))
	default:
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummary.Success",
			Other: "From now on, I'll delete match summary messages after {{.Minutes}} minutes.",
		},
			map[string]interface{}{
				"Minutes": num,
			}))
	}

	return true
}

func SettingMatchSummaryChannel(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(sett.GetMatchSummaryChannelID(), setting.AllSettings[setting.MatchSummaryChannel], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}

	// now to find the channel they are referencing
	channelID := ""
	channelName := "" // we track name to confirm to the User they selected the right channel
	channelList, _ := s.GuildChannels(m.GuildID)
	for _, c := range channelList {
		// Check if channel is a text channel
		if c.Type != discordgo.ChannelTypeGuildText {
			continue
		}
		// check if this is the right channel
		if strings.ToLower(c.Name) == args[2] || c.ID == args[2] {
			channelID = c.ID
			channelName = c.Name
			break
		}
	}

	// check if channel was found
	if channelID == "" {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummaryChannel.withoutChannelID",
			Other: "Could not find the text channel `{{.channelName}}`! Pass in the name or the ID, and make sure the bot can see it.",
		},
			map[string]interface{}{
				"channelName": args[2],
			}))
		return false
	} else {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMatchSummaryChannel.withChannelName",
			Other: "Match Summary text channel changed to `{{.channelName}}`!",
		},
			map[string]interface{}{
				"channelName": channelName,
			}))
		sett.SetMatchSummaryChannelID(channelID)
		return true
	}
}

func SettingAutoRefresh(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(fmt.Sprintf("%v", sett.GetAutoRefresh()), setting.AllSettings[setting.AutoRefresh], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}

	val := args[2]
	if val != "t" && val != "true" && val != "f" && val != "false" {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingAutoRefresh.Unrecognized",
			Other: "{{.Arg}} is not a true/false value. See `{{.CommandPrefix}} settings autorefresh` for usage",
		},
			map[string]interface{}{
				"Arg":           val,
				"CommandPrefix": sett.CommandPrefix,
			}))
		return false
	}

	newSet := val == "t" || val == "true"
	sett.SetAutoRefresh(newSet)
	if newSet {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingAutoRefresh.True",
			Other: "From now on, I'll AutoRefresh the game status message",
		}))
	} else {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingAutoRefresh.False",
			Other: "From now on, I will not AutoRefresh the game status message",
		}))
	}

	return true
}

func SettingMapVersion(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(fmt.Sprintf("%v", sett.GetMapVersion()), setting.AllSettings[setting.MapVersion], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}

	val := strings.ToLower(args[2])
	valid := map[string]bool{"simple": true, "detailed": true}
	if !valid[val] {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingMapVersion.Unrecognized",
			Other: "{{.Arg}} is not an expected value. See `{{.CommandPrefix}} settings mapversion` for usage",
		},
			map[string]interface{}{
				"Arg":           val,
				"CommandPrefix": sett.CommandPrefix,
			}))
		return false
	}

	sett.SetMapVersion(val)
	s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingMapVersion.Success",
		Other: "From now on, I will display map images as {{.Arg}}",
	},
		map[string]interface{}{
			"Arg": val,
		}))

	return true
}

func SettingLeaderboardNameMention(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(fmt.Sprintf("%v", sett.GetLeaderboardMention()), setting.AllSettings[setting.LeaderboardMention], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}

	val := args[2]
	if val != "t" && val != "true" && val != "f" && val != "false" {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardMention.Unrecognized",
			Other: "{{.Arg}} is not a true/false value. See `{{.CommandPrefix}} settings leaderboardMention` for usage",
		},
			map[string]interface{}{
				"Arg":           val,
				"CommandPrefix": sett.CommandPrefix,
			}))
		return false
	}

	newSet := val == "t" || val == "true"
	sett.SetLeaderboardMention(newSet)
	if newSet {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardMention.True",
			Other: "From now on, I'll mention players directly in the leaderboard",
		}))
	} else {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardMention.False",
			Other: "From now on, I'll use player nicknames/usernames in the leaderboard",
		}))
	}

	return true
}

func SettingLeaderboardSize(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(fmt.Sprintf("%d", sett.GetLeaderboardSize()), setting.AllSettings[setting.LeaderboardSize], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}

	num, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardSize.Unrecognized",
			Other: "{{.Number}} is not a valid number. See `{{.CommandPrefix}} settings leaderboardSize` for usage",
		},
			map[string]interface{}{
				"Number":        args[2],
				"CommandPrefix": sett.CommandPrefix,
			}))
		return false
	}
	if num > 10 || num < 1 {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardSize.OutOfRange",
			Other: "You provided a number too high or too low. Please specify a number between [1-10]",
		}))
		return false
	}

	sett.SetLeaderboardSize(int(num))

	s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingLeaderboardSize.Success",
		Other: "From now on, I'll display {{.Players}} players on the leaderboard",
	},
		map[string]interface{}{
			"Players": num,
		}))

	return true
}

func SettingLeaderboardMin(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(fmt.Sprintf("%d", sett.GetLeaderboardMin()), setting.AllSettings[setting.LeaderboardMin], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}

	num, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardMin.Unrecognized",
			Other: "{{.Number}} is not a valid number. See `{{.CommandPrefix}} settings leaderboardMin` for usage",
		},
			map[string]interface{}{
				"Number":        args[2],
				"CommandPrefix": sett.CommandPrefix,
			}))
		return false
	}
	if num > 100 || num < 1 {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingLeaderboardMin.OutOfRange",
			Other: "You provided a number too high or too low. Please specify a number between [1-100]",
		}))
		return false
	}

	sett.SetLeaderboardMin(int(num))

	s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
		ID:    "settings.SettingLeaderboardMin.Success",
		Other: "From now on, I'll display only players with {{.Games}}+ qualifying games on the leaderboard",
	},
		map[string]interface{}{
			"Games": num,
		}))

	return true
}

func SettingMuteSpectators(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, args []string) bool {
	muteSpec := sett.GetMuteSpectator()
	if len(args) == 2 {
		current := "false"
		if muteSpec {
			current = "true"
		}
		embed := ConstructEmbedForSetting(current, setting.AllSettings[setting.MuteSpectators], sett)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}
	switch {
	case args[2] == "true":
		if muteSpec {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingUnmuteDeadDuringTasks.true_noUnmuteDead",
				Other: "It's already true!",
			}))
		} else {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingMuteSpectators.true_noMuteSpectators",
				Other: "I will now mute spectators just like dead players. \n**Note, this can cause delays or slowdowns when not self-hosting, or using a Premium worker bot!**",
			}))
			sett.SetMuteSpectator(true)
			return true
		}
	case args[2] == "false":
		if muteSpec {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "settings.SettingMuteSpectators.false_muteSpectators",
				Other: "I will no longer mute spectators like dead players",
			}))
			sett.SetMuteSpectator(false)
			return true
		}
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingUnmuteDeadDuringTasks.false_noUnmuteDead",
			Other: "It's already false!",
		}))
	default:
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "settings.SettingUnmuteDeadDuringTasks.wrongArg",
			Other: "Sorry, `{{.Arg}}` is neither `true` nor `false`.",
		},
			map[string]interface{}{
				"Arg": args[2],
			}))
	}
	return false
}
