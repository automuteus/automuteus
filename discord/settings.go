package discord

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	"log"
	"strconv"
	"strings"
)

type SettingType int

const (
	Prefix SettingType = iota
	TrackedChannel
	AdminUserIDs
	RoleIDs
	Nicknames
	UnmuteDead
	Delays
	VoiceRules
)

type Setting struct {
	settingType SettingType
	name        string
	example     string
	shortDesc   string
	desc        string
	args        string
	aliases     []string
}

var AllSettings = []Setting{
	{
		settingType: Prefix,
		name:        "commandPrefix",
		example:     "commandPrefix !",
		shortDesc:   "Bot prefix",
		desc:        "Change the prefix that the bot uses to detect commands",
		args:        "<prefix>",
		aliases:     []string{"prefix", "cp"},
	},
	{
		settingType: TrackedChannel,
		name:        "defaultTrackedChannel",
		example:     "defaultTrackedChannel Among Us Voice",
		shortDesc:   "Default tracked voice channel",
		desc:        "Change the default tracked voice channel",
		args:        "<voice channel name>",
		aliases:     []string{"channel", "vc", "dtc"},
	},
	{
		settingType: AdminUserIDs,
		name:        "adminUserIDs",
		example:     "adminUserIDs @Soup @Bob",
		shortDesc:   "Bot Admins",
		desc:        "Specify which individual users have permissions to invoke the bot",
		args:        "<user @ mentions>...",
		aliases:     []string{"admins", "admin", "auid", "aui", "a"},
	},
	{
		settingType: RoleIDs,
		name:        "permissionRoleIDs",
		example:     "permissionRoleIDs @Bot Admins @Bot Mods",
		shortDesc:   "Bot Admins by Role",
		desc:        "Specify which roles have permissions to invoke the bot",
		args:        "<role @ mentions>...",
		aliases:     []string{"roles", "role", "prid", "pri", "r"},
	},
	{
		settingType: Nicknames,
		name:        "applyNicknames",
		example:     "applyNicknames false",
		shortDesc:   "Bot renames Discord users",
		desc:        "Specify if the bot should rename Discord users to match their in-game names or not",
		args:        "<true/false>",
		aliases:     []string{"nicknames", "nickname", "an"},
	},
	{
		settingType: UnmuteDead,
		name:        "unmuteDeadDuringTasks",
		example:     "unmuteDeadDuringTasks false",
		shortDesc:   "Bot unmutes players on death",
		desc:        "Specify if the bot should immediately unmute players when they die. **CAUTION. Leaks information!**",
		args:        "<true/false>",
		aliases:     []string{"unmute", "uddt"},
	},
	{
		settingType: Delays,
		name:        "delays",
		example:     "delays lobby tasks 5",
		shortDesc:   "Delays between stages",
		desc:        "Specify the delays for automute/deafen between stages of the game, like lobby->tasks",
		args:        "<start phase> <end phase> <delay>",
		aliases:     []string{"delays", "d"},
	},
	{
		settingType: VoiceRules,
		name:        "voiceRules",
		example:     "voiceRules mute tasks dead true",
		shortDesc:   "Mute/deafen rules",
		desc:        "Specify mute/deafen rules for the game, depending on the stage and the alive/deadness of players. Example given would mute dead players during the tasks stage",
		args:        "<mute/deaf> <game phase> <dead/alive> <true/false>",
		aliases:     []string{"vr"},
	},
}

func ConstructEmbedForSetting(value string, setting Setting) discordgo.MessageEmbed {
	return discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       setting.name,
		Description: setting.desc,
		Timestamp:   "",
		Color:       15844367, //GOLD
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{
				Name:   "Current Value",
				Value:  value,
				Inline: false,
			},
			&discordgo.MessageEmbedField{
				Name:   "Example",
				Value:  "`" + setting.example + "`",
				Inline: false,
			},
			&discordgo.MessageEmbedField{
				Name:   "Arguments",
				Value:  "`" + setting.args + "`",
				Inline: false,
			},
			&discordgo.MessageEmbedField{
				Name:   "Aliases",
				Value:  strings.Join(setting.aliases, ", "),
				Inline: false,
			},
		},
	}
}

func HandleSettingsCommand(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, storageInterface storage.StorageInterface, args []string) {
	// if no arg passed, send them list of possible settings to change
	if len(args) == 1 {
		embed := settingResponse(AllSettings)
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return
	}
	// if command invalid, no need to reapply changes to json file
	isValid := false
	switch args[1] {
	case "commandprefix":
		fallthrough
	case "prefix":
		fallthrough
	case "cp":
		isValid = CommandPrefixSetting(s, m, guild, args)
		break
	case "defaulttrackedchannel":
		fallthrough
	case "channel":
		fallthrough
	case "vc":
		fallthrough
	case "dtc":
		isValid = SettingDefaultTrackedChannel(s, m, guild, args)
		break
	case "adminuserids":
		fallthrough
	case "admins":
		fallthrough
	case "admin":
		fallthrough
	case "auid":
		fallthrough
	case "aui":
		fallthrough
	case "a":
		isValid = SettingAdminUserIDs(s, m, guild, args)
		break
	case "permissionroleids":
		fallthrough
	case "roles":
		fallthrough
	case "role":
		fallthrough
	case "prid":
		fallthrough
	case "pri":
		fallthrough
	case "r":
		isValid = SettingPermissionRoleIDs(s, m, guild, args)
		break
	case "applynicknames":
		fallthrough
	case "nicknames":
		fallthrough
	case "nickname":
		fallthrough
	case "an":
		isValid = SettingApplyNicknames(s, m, guild, args)
		break
	case "unmutedeadduringtasks":
		fallthrough
	case "unmute":
		fallthrough
	case "uddt":
		isValid = SettingUnmuteDeadDuringTasks(s, m, guild, args)
		break
	case "delays":
		fallthrough
	case "d":
		isValid = SettingDelays(s, m, guild, args)
		break
	case "voicerules":
		fallthrough
	case "vr":
		isValid = SettingVoiceRules(s, m, guild, args)
		break
	default:
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, `%s` is not a valid setting!\n", args[1]))
	}
	if isValid {
		err := storageInterface.WriteGuildSettings(m.GuildID, guild.guildSettings)
		if err != nil {
			log.Println(err)
		}
	}
}

func CommandPrefixSetting(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting(guild.guildSettings.GetCommandPrefix(), AllSettings[Prefix])
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}
	if len(args[2]) > 10 {
		// prevent someone from setting something ridiculous lol
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, the prefix `%s` is too long (%d characters, max 10). Try something shorter.", args[2], len(args[2])))
		return false
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Guild prefix changed from `%s` to `%s`. Use that from now on!",
		guild.CommandPrefix(), args[2]))
	guild.guildSettings.SetCommandPrefix(args[2])
	return true
}

func SettingDefaultTrackedChannel(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	if len(args) == 2 {
		// give them both command syntax and current voice channel
		channelList, _ := s.GuildChannels(m.GuildID)
		for _, c := range channelList {
			if c.ID == guild.GetDefaultTrackedChannel() {
				embed := ConstructEmbedForSetting(guild.guildSettings.GetDefaultTrackedChannel(), AllSettings[TrackedChannel])
				s.ChannelMessageSendEmbed(m.ChannelID, &embed)
				return false
			}
		}
		embed := ConstructEmbedForSetting("No default tracked voice channel", AllSettings[TrackedChannel])
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}
	// now to find the channel they are referencing
	channelID := ""
	channelName := "" // we track name to confirm to the user they selected the right channel
	channelList, _ := s.GuildChannels(m.GuildID)
	for _, c := range channelList {
		// Check if channel is a voice channel
		if c.Type != discordgo.ChannelTypeGuildVoice {
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
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Could not find the voice channel `%s`! Pass in the name or the ID, and make sure the bot can see it.", args[2]))
		return false
	} else {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Default voice channel changed to `%s`. Use that from now on!",
			channelName))
		guild.guildSettings.SetDefaultTrackedChannel(channelID)
		return true
	}
}

func SettingAdminUserIDs(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	adminIDs := guild.guildSettings.GetAdminUserIDs()
	if len(args) == 2 {
		adminCount := len(adminIDs) // caching for optimisation
		// make a nicely formatted string of all the admins: "user1, user2, user3 and user4"
		if adminCount == 0 {
			embed := ConstructEmbedForSetting("No Bot Admins", AllSettings[AdminUserIDs])
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		} else {
			listOfAdmins := ""
			for index, ID := range adminIDs {
				if index == 0 {
					listOfAdmins += "<@" + ID + ">"
				} else if index == adminCount-1 {
					listOfAdmins += " and <@" + ID + ">"
				} else {
					listOfAdmins += ", <@" + ID + ">"
				}
			}
			embed := ConstructEmbedForSetting(listOfAdmins, AllSettings[AdminUserIDs])
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		}
		return false
	}
	newAdminIDs := []string{}
	// users the user mentioned in their message
	var userIDs []string

	for _, userName := range args[2:] {
		if userName == "" || userName == " " {
			// user added a double space by accident, ignore it
			continue
		}
		ID := getMemberFromString(s, m.GuildID, userName)
		if ID == "" {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, I don't know who `%s` is. You can pass in ID, username, username#XXXX, nickname or @mention", userName))
			continue
		}
		userIDs = append(userIDs, ID)
	}

	for _, ID := range userIDs {
		if ID != "" {
			newAdminIDs = append(newAdminIDs, ID)
			// mention user without pinging
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content:         fmt.Sprintf("<@%s> is now a bot admin!", ID),
				AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
			})
		}
	}

	guild.guildSettings.SetAdminUserIDs(newAdminIDs)
	return true
}

func SettingPermissionRoleIDs(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	oldRoleIDs := guild.guildSettings.GetPermissionRoleIDs()
	if len(args) == 2 {
		adminRoleCount := len(oldRoleIDs) // caching for optimisation
		// make a nicely formatted string of all the roles: "role1, role2, role3 and role4"
		if adminRoleCount == 0 {
			embed := ConstructEmbedForSetting("No Role Admins", AllSettings[RoleIDs])
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		} else {
			listOfRoles := ""
			for index, ID := range oldRoleIDs {
				if index == 0 {
					listOfRoles += "<&" + ID + ">"
				} else if index == adminRoleCount-1 {
					listOfRoles += " and <&" + ID + ">"
				} else {
					listOfRoles += ", <&" + ID + ">"
				}
			}
			embed := ConstructEmbedForSetting(listOfRoles, AllSettings[RoleIDs])
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		}
		return false
	}

	newRoleIDs := []string{}
	// roles the user mentioned in their message
	var roleIDs []string

	for _, roleName := range args[2:] {
		if roleName == "" || roleName == " " {
			// user added a double space by accident, ignore it
			continue
		}
		ID := getRoleFromString(s, m.GuildID, roleName)
		if ID == "" {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, I don't know the role `%s` is. You can pass the role ID, role name or @role", roleName))
			continue
		}
		roleIDs = append(roleIDs, ID)
	}

	for _, ID := range roleIDs {
		if ID != "" {
			newRoleIDs = append(newRoleIDs, ID)
			// mention user without pinging
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content:         fmt.Sprintf("<@&%s>s are now bot admins!", ID),
				AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
			})
		}
	}

	guild.guildSettings.SetPermissionRoleIDs(newRoleIDs)
	return true
}

func SettingApplyNicknames(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	applyNicknames := guild.guildSettings.GetApplyNicknames()
	if len(args) == 2 {
		current := "false"
		if applyNicknames {
			current = "true"
		}
		embed := ConstructEmbedForSetting(current, AllSettings[Nicknames])
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}

	if args[2] == "true" {
		if applyNicknames {
			s.ChannelMessageSend(m.ChannelID, "It's already true!")
		} else {
			s.ChannelMessageSend(m.ChannelID, "I will now rename the players in the voice chat.")
			guild.guildSettings.SetApplyNicknames(true)
			return true
		}
	} else if args[2] == "false" {
		if applyNicknames {
			s.ChannelMessageSend(m.ChannelID, "I will no longer rename the players in the voice chat.")
			guild.guildSettings.SetApplyNicknames(false)
			return true
		} else {
			s.ChannelMessageSend(m.ChannelID, "It's already false!")
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, `%s` is neither `true` nor `false`.", args[2]))
	}
	return false
}

func SettingUnmuteDeadDuringTasks(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	unmuteDead := guild.guildSettings.GetUnmuteDeadDuringTasks()
	if len(args) == 2 {
		current := "false"
		if unmuteDead {
			current = "true"
		}
		embed := ConstructEmbedForSetting(current, AllSettings[UnmuteDead])
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}
	if args[2] == "true" {
		if unmuteDead {
			s.ChannelMessageSend(m.ChannelID, "It's already true!")
		} else {
			s.ChannelMessageSend(m.ChannelID, "I will now unmute the dead people immediately after they die. Careful, this reveals who died during the match!")
			guild.guildSettings.SetUnmuteDeadDuringTasks(true)
			return true
		}
	} else if args[2] == "false" {
		if unmuteDead {
			s.ChannelMessageSend(m.ChannelID, "I will no longer immediately unmute dead people. Good choice!")
			guild.guildSettings.SetUnmuteDeadDuringTasks(false)
			return true
		} else {
			s.ChannelMessageSend(m.ChannelID, "It's already false!")
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, `%s` is neither `true` nor `false`.", args[2]))
	}
	return false
}

func SettingDelays(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting("N/A", AllSettings[Delays])
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}
	// user passes phase name, phase name and new delay value
	if len(args) < 4 {
		// user didn't pass 2 phases, tell them the list of game phases
		s.ChannelMessageSend(m.ChannelID, "The list of game phases are `Lobby`, `Tasks` and `Discussion`.\n"+
			"You need to type both phases the game is transitioning from and to to change the delay.") // find a better wording for this at some point
		return false
	}
	// now to find the actual game state from the string they passed
	var gamePhase1 = getPhaseFromString(args[2])
	var gamePhase2 = getPhaseFromString(args[3])
	if gamePhase1 == game.UNINITIALIZED {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("I don't know what `%s` is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.", args[2]))
		return false
	} else if gamePhase2 == game.UNINITIALIZED {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("I don't know what `%s` is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.", args[3]))
		return false
	}
	oldDelay := guild.guildSettings.GetDelay(gamePhase1, gamePhase2)
	if len(args) == 4 {
		// no number was passed, user was querying the delay
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Currently, the delay when passing from `%s` to `%s` is %d.", args[2], args[3], oldDelay))
		return false
	}
	newDelay, err := strconv.Atoi(args[4])
	if err != nil || newDelay < 0 {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%s` is not a valid number! Please try again", args[4]))
		return false
	}
	guild.guildSettings.SetDelay(gamePhase1, gamePhase2, newDelay)
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("The delay when passing from `%s` to `%s` changed from %d to %d.", args[2], args[3], oldDelay, newDelay))
	return true
}

func SettingVoiceRules(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	if len(args) == 2 {
		embed := ConstructEmbedForSetting("N/A", AllSettings[VoiceRules])
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		return false
	}
	// now for a bunch of input checking
	if len(args) < 5 {
		// user didn't pass enough args
		s.ChannelMessageSend(m.ChannelID, "You didn't pass enough arguments! Correct syntax is: `voiceRules [mute/deaf] [game phase] [alive/dead] [true/false]`")
		return false
	}
	if args[2] == "deaf" {
		args[2] = "deafened" // for formatting later on
	} else if args[2] == "mute" {
		args[2] = "muted" // same here
	} else {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%s` is neither `mute` nor `deaf`!", args[2]))
		return false
	}
	gamePhase := getPhaseFromString(args[3])
	if gamePhase == game.UNINITIALIZED {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("I don't know what %s is. The list of game phases are `Lobby`, `Tasks` and `Discussion`.", args[3]))
		return false
	}
	if args[4] != "alive" && args[4] != "dead" {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%s` is neither `alive` or `dead`!", args[4]))
		return false
	}
	var oldValue bool
	if args[2] == "muted" {
		oldValue = guild.guildSettings.GetVoiceRule(true, gamePhase, args[4])
	} else {
		oldValue = guild.guildSettings.GetVoiceRule(false, gamePhase, args[4])
	}
	if len(args) == 5 {
		// user was only querying
		if oldValue {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("When in `%s` phase, %s players are currently %s.", args[3], args[4], args[2]))
		} else {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("When in `%s` phase, %s players are currently NOT %s.", args[3], args[4], args[2]))
		}
		return false
	}
	var newValue bool
	if args[5] == "true" {
		newValue = true
	} else if args[5] == "false" {
		newValue = false
	} else {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%s` is neither `true` or `false`!", args[5]))
		return false
	}
	if newValue == oldValue {
		if newValue {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("When in `%s` phase, %s players are already %s!", args[3], args[4], args[2]))
		} else {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("When in `%s` phase, %s players are already un%s!", args[3], args[4], args[2]))
		}
		return false
	}
	if args[2] == "muted" {
		guild.guildSettings.SetVoiceRule(true, gamePhase, args[4], newValue)
	} else {
		guild.guildSettings.SetVoiceRule(false, gamePhase, args[4], newValue)
	}
	if newValue {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("From now on, when in `%s` phase, %s players will be %s.", args[3], args[4], args[2]))
	} else {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("From now on, when in `%s` phase, %s players will be un%s.", args[3], args[4], args[2]))
	}
	return true
}
