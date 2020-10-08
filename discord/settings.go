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

func HandleSettingsCommand(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, storageInterface storage.StorageInterface, args []string) {
	// if no arg passed, send them list of possible settings to change
	if len(args) == 1 {
		s.ChannelMessageSend(m.ChannelID, "The list of possible settings are:\n"+
			"•`CommandPrefix [prefix]`: Change the bot's prefix in this server\n"+
			"•`DefaultTrackedChannel [voiceChannel]`: Change the voice channel the bot tracks by default\n"+
			"•`AdminUserIDs [user 1] [user 2] [etc]`: Add or remove bot admins a.k.a users that can use commands with the bot\n"+
			"•`PermissionRoleIDs [role 1] [role 2] [etc]`: Add or remove bot admin roles, a.k.a roles that can use commands with the bot.\n"+
			"•`ApplyNicknames [true/false]`: Whether the bot should change the nicknames of the players to reflect the player's color\n"+
			"•`UnmuteDeadDuringTasks [true/false]`: Whether the bot should unmute dead players immediately when they die (**WARNING**: reveals information)\n"+
			"•`Delays [old game phase] [new game phase] [delay]`: Change the delay between changing game phase and muting/unmuting players\n"+
			"•`VoiceRules [mute/deaf] [game phase] [alive/dead] [true/false]`: Whether to mute/deafen alive/dead players during that game phase")
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
	case "defaulttrackedchannel":
		fallthrough
	case "channel":
		fallthrough
	case "vc":
		fallthrough
	case "dtc":
		isValid = SettingDefaultTrackedChannel(s, m, guild, args)
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
	case "applynicknames":
		fallthrough
	case "nicknames":
		fallthrough
	case "nickname":
		fallthrough
	case "an":
		isValid = SettingApplyNicknames(s, m, guild, args)
	case "unmutedeadduringtasks":
		fallthrough
	case "unmute":
		fallthrough
	case "uddt":
		isValid = SettingUnmuteDeadDuringTasks(s, m, guild, args)
	case "delays":
		fallthrough
	case "d":
		isValid = SettingDelays(s, m, guild, args)
	case "voicerules":
		fallthrough
	case "vr":
		isValid = SettingVoiceRules(s, m, guild, args)
	default:
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, `%s` is not a valid setting!\n"+
			"Valid settings include `CommandPrefix`, `DefaultTrackedChannel`, `AdminUserIDs`, `ApplyNicknames`, `UnmuteDeadDuringTasks`, `Delays` and `VoiceRules`.", args[1]))
	}
	if isValid {
		data, err := guild.PersistentGuildData.ToData()
		if err != nil {
			log.Println(err)
		} else {
			err := storageInterface.WriteGuildData(m.GuildID, data)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func CommandPrefixSetting(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	if len(args) == 2 {
		s.ChannelMessageSend(m.ChannelID, "`CommandPrefix [prefix]`: Change the bot's prefix in this server.")
		return false
	}
	if len(args[2]) > 10 {
		// prevent someone from setting something ridiculous lol
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, the prefix `%s` is too long (%d characters, max 10). Try something shorter.", args[2], len(args[2])))
		return false
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Guild prefix changed from `%s` to `%s`. Use that from now on!",
		guild.PersistentGuildData.CommandPrefix, args[2]))
	guild.PersistentGuildData.CommandPrefix = args[2]
	return true
}

func SettingDefaultTrackedChannel(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	if len(args) == 2 {
		// give them both command syntax and current voice channel
		channelList, _ := s.GuildChannels(m.GuildID)
		for _, c := range channelList {
			if c.ID == guild.PersistentGuildData.DefaultTrackedChannel {
				s.ChannelMessageSend(m.ChannelID, "`DefaultTrackedChannel [voiceChannel]`: Change the voice channel the bot tracks by default.\n"+
					fmt.Sprintf("Currently, I'm tracking the `%s` voice channel", c.Name))
				return false
			}
		}
		s.ChannelMessageSend(m.ChannelID, "`DefaultTrackedChannel [voiceChannel]`: Change the voice channel the bot tracks by default.\n"+
			"Currently, I'm not tracking any voice channel. Either the ID is invalid or you didn't give me one.")
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
		guild.PersistentGuildData.DefaultTrackedChannel = channelID
		return true
	}
}

func SettingAdminUserIDs(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	if len(args) == 2 {
		adminCount := len(guild.PersistentGuildData.AdminUserIDs) // caching for optimisation
		// make a nicely formatted string of all the admins: "user1, user2, user3 and user4"
		if adminCount == 0 {
			s.ChannelMessageSend(m.ChannelID, "`AdminUserIDs [user 1] [user 2] [etc]`: Add or remove bot admins, a.k.a users that can use commands with the bot.\n"+
				"Currently, there are no bot admins.")
		} else if adminCount == 1 {
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content: "`AdminUserIDs [user 1] [user 2] [etc]`: Add or remove bot admins, a.k.a users that can use commands with the bot.\n" +
					fmt.Sprintf("Currently, the only admin is <@%s>.", guild.PersistentGuildData.AdminUserIDs[0]),
				AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
			})
		} else {
			listOfAdmins := ""
			for index, ID := range guild.PersistentGuildData.AdminUserIDs {
				if index == 0 {
					listOfAdmins += "<@" + ID + ">"
				} else if index == adminCount-1 {
					listOfAdmins += " and <@" + ID + ">"
				} else {
					listOfAdmins += ", <@" + ID + ">"
				}
			}
			// mention users without pinging
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content: "`AdminUserIDs [user 1] [user 2] [etc]`: Add or remove bot admins, a.k.a users that can use commands with the bot.\n" +
					fmt.Sprintf("Currently, the admins are %s.", listOfAdmins),
				AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
			})
		}
		return false
	}
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
		// check if id is already in array
		for _, IDinSlice := range userIDs {
			if ID == IDinSlice {
				// this user is mentioned more than once, ignore it
				ID = "already in list"
				break
			}
		}
		if ID != "already in list" {
			userIDs = append(userIDs, ID)
		}
	}

	// the index of admins to remove from AdminUserIDs
	var removeAdmins []int
	isValid := false

	for _, ID := range userIDs {
		// can't use guild.HasAdminPermissions() because we also need index
		for index, adminID := range guild.PersistentGuildData.AdminUserIDs {
			if ID == adminID {
				// add ID to IDs to be deleted
				removeAdmins = append(removeAdmins, index)
				ID = "" // indicate to other loop this ID has been dealt with
				break
			}
		}
		if ID != "" {
			guild.PersistentGuildData.AdminUserIDs = append(guild.PersistentGuildData.AdminUserIDs, ID)
			// mention user without pinging
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content:         fmt.Sprintf("<@%s> is now a bot admin!", ID),
				AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
			})
			isValid = true
		}
	}

	if len(removeAdmins) == 0 {
		return isValid
	}

	// remove the values from guild.PersistentGuildData.AdminUserIDs by creating a
	// new array with only the admins the user didn't remove, and replacing the
	// current array with that one
	var newAdminList []string
	currentIndex := 0
	nextIndexToRemove := removeAdmins[0]
	currentIndexInRemoveAdmins := 0

	for currentIndex < len(guild.PersistentGuildData.AdminUserIDs) {
		if currentIndex != nextIndexToRemove {
			// user didn't remove this admin, add it to the list
			newAdminList = append(newAdminList, guild.PersistentGuildData.AdminUserIDs[currentIndex])
		} else {
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content:         fmt.Sprintf("<@%s> is no longer a bot admin, RIP", guild.PersistentGuildData.AdminUserIDs[currentIndex]),
				AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
			})
			currentIndexInRemoveAdmins++
			if currentIndexInRemoveAdmins < len(removeAdmins) {
				nextIndexToRemove = removeAdmins[currentIndexInRemoveAdmins]
			} else {
				// reached the end of removeAdmins
				newAdminList = append(newAdminList, guild.PersistentGuildData.AdminUserIDs[currentIndex+1:]...)
				break
			}
		}
		currentIndex++
	}

	guild.PersistentGuildData.AdminUserIDs = newAdminList
	return true
}

func SettingPermissionRoleIDs(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	if len(args) == 2 {
		adminRoleCount := len(guild.PersistentGuildData.PermissionedRoleIDs) // caching for optimisation
		// make a nicely formatted string of all the roles: "role1, role2, role3 and role4"
		if adminRoleCount == 0 {
			s.ChannelMessageSend(m.ChannelID, "`PermissionRoleIDs [role 1] [role 2] [etc]`: Add or remove bot admin roles, a.k.a roles that can use commands with the bot.\n"+
				"Currently, there are no bot admin roles.")
		} else if adminRoleCount == 1 {
			// mention role without pinging
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content: "`PermissionRoleIDs [role 1] [role 2] [etc]`: Add or remove bot admin roles, a.k.a roles that can use commands with the bot.\n" +
					fmt.Sprintf("Currently, the only admin role is <&%s>.", guild.PersistentGuildData.PermissionedRoleIDs[0]),
				AllowedMentions: &discordgo.MessageAllowedMentions{Roles: nil},
			})
		} else {
			listOfRoles := ""
			for index, ID := range guild.PersistentGuildData.PermissionedRoleIDs {
				if index == 0 {
					listOfRoles += "<&" + ID + ">"
				} else if index == adminRoleCount-1 {
					listOfRoles += " and <&" + ID + ">"
				} else {
					listOfRoles += ", <&" + ID + ">"
				}
			}
			// mention roles without pinging
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content: "`PermissionRoleIDs [role 1] [role 2] [etc]`: Add or remove bot admin roles, a.k.a roles that can use commands with the bot\n" +
					fmt.Sprintf("Currently, the admin roles are %s.", listOfRoles),
				AllowedMentions: &discordgo.MessageAllowedMentions{Roles: nil},
			})
		}
		return false
	}

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
		// check if id is already in array
		for _, IDinSlice := range roleIDs {
			if ID == IDinSlice {
				// this role is mentioned more than once, ignore it
				ID = "already in list"
				break
			}
		}
		if ID != "already in list" {
			roleIDs = append(roleIDs, ID)
		}
	}

	// index of roles to get adminated (or is it admin-ed...)
	var removeRoles []int
	isValid := false

	for _, ID := range roleIDs {
		// can't use guild.HasRolePermissions() because we also need index
		for index, adminRoleID := range guild.PersistentGuildData.PermissionedRoleIDs {
			if ID == adminRoleID {
				// add ID to IDs to be deleted
				removeRoles = append(removeRoles, index)
				ID = "" // indicate to other loop this ID has been dealt with
				break
			}
		}
		if ID != "" {
			guild.PersistentGuildData.PermissionedRoleIDs = append(guild.PersistentGuildData.PermissionedRoleIDs, ID)
			// mention user without pinging
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content:         fmt.Sprintf("<@&%s>s are now bot admins!", ID),
				AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
			})
			isValid = true
		}
	}

	if len(removeRoles) == 0 {
		return isValid
	}

	// same process as removeAdminIDs
	var newAdminRoleList []string
	currentIndex := 0
	nextIndexToRemove := removeRoles[0]
	currentIndexInRemoveAdminRoles := 0

	for currentIndex < len(guild.PersistentGuildData.PermissionedRoleIDs) {
		if currentIndex != nextIndexToRemove {
			// user didn't remove this role, add it to the list
			newAdminRoleList = append(newAdminRoleList, guild.PersistentGuildData.PermissionedRoleIDs[currentIndex])
		} else {
			s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
				Content:         fmt.Sprintf("<@&%s>s are no longer a bot admins.", guild.PersistentGuildData.PermissionedRoleIDs[currentIndex]),
				AllowedMentions: &discordgo.MessageAllowedMentions{Users: nil},
			})
			currentIndexInRemoveAdminRoles++
			if currentIndexInRemoveAdminRoles < len(removeRoles) {
				nextIndexToRemove = removeRoles[currentIndexInRemoveAdminRoles]
			} else {
				// reached the end of removeRoles
				newAdminRoleList = append(newAdminRoleList, guild.PersistentGuildData.PermissionedRoleIDs[currentIndex+1:]...)
				break
			}
		}
		currentIndex++
	}

	guild.PersistentGuildData.PermissionedRoleIDs = newAdminRoleList
	return true
}

func SettingApplyNicknames(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	if len(args) == 2 {
		if guild.PersistentGuildData.ApplyNicknames {
			s.ChannelMessageSend(m.ChannelID, "`ApplyNicknames [true/false]`: Whether the bot should change the nicknames of the players to reflect the player's color.\n"+
				"Currently the bot does change nicknames.")
		} else {
			s.ChannelMessageSend(m.ChannelID, "`ApplyNicknames [true/false]`: Whether the bot should change the nicknames of the players to reflect the player's color.\n"+
				"Currently the bot does **not** change nicknames.")
		}
		return false
	}
	if args[2] == "true" {
		if guild.PersistentGuildData.ApplyNicknames {
			s.ChannelMessageSend(m.ChannelID, "It's already true!")
		} else {
			s.ChannelMessageSend(m.ChannelID, "I will now rename the players in the voice chat.")
			guild.PersistentGuildData.ApplyNicknames = true
			return true
		}
	} else if args[2] == "false" {
		if guild.PersistentGuildData.ApplyNicknames {
			s.ChannelMessageSend(m.ChannelID, "I will no longer  rename the players in the voice chat.")
			guild.PersistentGuildData.ApplyNicknames = false
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
	if len(args) == 2 {
		if guild.PersistentGuildData.UnmuteDeadDuringTasks {
			s.ChannelMessageSend(m.ChannelID, "`UnmuteDeadDuringTasks [true/false]`: Whether the bot should unmute dead players immediately when they die. "+
				"**WARNING**: reveals who died before discussion begins! Use at your own risk.\n"+
				"Currently the bot does unmute the players immediately after dying.")
		} else {
			s.ChannelMessageSend(m.ChannelID, "`UnmuteDeadDuringTasks [true/false]`: Whether the bot should unmute dead players immediately when they die. "+
				"**WARNING**: reveals who died before discussion begins! Use at your own risk.\n"+
				"Currently the bot does **not** unmute the players immediately after dying.")
		}
		return false
	}
	if args[2] == "true" {
		if guild.PersistentGuildData.UnmuteDeadDuringTasks {
			s.ChannelMessageSend(m.ChannelID, "It's already true!")
		} else {
			s.ChannelMessageSend(m.ChannelID, "I will now unmute the dead people immediately after they die. Careful, this reveals who died during the match!")
			guild.PersistentGuildData.UnmuteDeadDuringTasks = true
			return true
		}
	} else if args[2] == "false" {
		if guild.PersistentGuildData.UnmuteDeadDuringTasks {
			s.ChannelMessageSend(m.ChannelID, "I will no longer immediately unmute dead people. Good choice!")
			guild.PersistentGuildData.UnmuteDeadDuringTasks = false
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
		s.ChannelMessageSend(m.ChannelID, "`Delays [old game phase] [new game phase] [delay]`: Change the delay between changing game phase and muting/unmuting players.")
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
	oldDelay := guild.PersistentGuildData.Delays.GetDelay(gamePhase1, gamePhase2)
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
	guild.PersistentGuildData.Delays.Delays[game.PhaseNames[gamePhase1]][game.PhaseNames[gamePhase2]] = newDelay
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("The delay when passing from `%s` to `%s` changed from %d to %d.", args[2], args[3], oldDelay, newDelay))
	return true
}

func SettingVoiceRules(s *discordgo.Session, m *discordgo.MessageCreate, guild *GuildState, args []string) bool {
	if len(args) == 2 {
		s.ChannelMessageSend(m.ChannelID, "`VoiceRules [mute/deaf] [game phase] [alive/dead] [true/false]`: Whether to mute/deafen alive/dead players during that game phase.")
		return false
	}
	// now for a bunch of input checking
	if len(args) < 5 {
		// user didn't pass enough args
		s.ChannelMessageSend(m.ChannelID, "You didn't pass enough arguments! Correct syntax is: `VoiceRules [mute/deaf] [game phase] [alive/dead] [true/false]`")
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
		oldValue = guild.PersistentGuildData.VoiceRules.MuteRules[game.PhaseNames[gamePhase]][args[4]]
	} else {
		oldValue = guild.PersistentGuildData.VoiceRules.DeafRules[game.PhaseNames[gamePhase]][args[4]]
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
		guild.PersistentGuildData.VoiceRules.MuteRules[game.PhaseNames[gamePhase]][args[4]] = newValue
	} else {
		guild.PersistentGuildData.VoiceRules.DeafRules[game.PhaseNames[gamePhase]][args[4]] = newValue
	}
	if newValue {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("From now on, when in `%s` phase, %s players will be %s.", args[3], args[4], args[2]))
	} else {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("From now on, when in `%s` phase, %s players will be un%s.", args[3], args[4], args[2]))
	}
	return true
}
