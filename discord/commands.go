package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/automuteus/utils/pkg/premium"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/metrics"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Type int

const (
	CommandEnumHelp Type = iota
	CommandEnumNew
	CommandEnumEnd
	CommandEnumPause
	CommandEnumRefresh
	CommandEnumLink
	CommandEnumUnlink
	CommandEnumUnmuteAll
	CommandEnumForce
	CommandEnumSettings
	CommandEnumMap
	CommandEnumCache
	CommandEnumPrivacy
	CommandEnumInfo
	CommandEnumDebugState
	CommandEnumASCII
	CommandEnumStats
	CommandEnumPremium
)

type Command struct {
	Aliases     []string
	Command     string
	Example     string
	Emoji       string
	CommandType Type
	ShortDesc   *i18n.Message
	Description *i18n.Message
	Arguments   *i18n.Message
	IsSecret    bool
	IsAdmin     bool
	IsOperator  bool

	fn func(
		bot *Bot,
		isAdmin bool,
		isPermissioned bool,
		sett *storage.GuildSettings,
		session *discordgo.Session,
		guild *discordgo.Guild,
		message *discordgo.MessageCreate,
		args []string,
		cmd Command,
	)
}

func getCommand(arg string) (Command, bool) {
	arg = strings.ToLower(arg)
	command, exists := commandMap[arg]
	return command, exists
}

// note, this mapping is HIERARCHICAL. If you type `l`, "link" would be used over "log"
var allCommands []Command
var commandMap = map[string]Command{}

func init() {
	allCommands = []Command{
		{
			CommandType: CommandEnumHelp,
			Command:     "help",
			Example:     "help track",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Help.shortDesc",
				Other: "Display help",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Help.desc",
				Other: "Display bot help message, or see info about a Command",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Help.args",
				Other: "None, or optional Command to see info for",
			},
			Aliases:    []string{"h"},
			IsSecret:   false,
			Emoji:      "❓",
			IsAdmin:    false,
			IsOperator: false,

			fn: commandFnHelp,
		},
		{
			CommandType: CommandEnumNew,
			Command:     "new",
			Example:     "new",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.New.shortDesc",
				Other: "Start a new game",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.New.desc",
				Other: "Start a new game",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.New.args",
				Other: "None",
			},
			Aliases:    []string{"start", "n"},
			IsSecret:   false,
			Emoji:      "🕹",
			IsAdmin:    false,
			IsOperator: true,

			fn: commandFnNew,
		},
		{
			CommandType: CommandEnumEnd,
			Command:     "end",
			Example:     "end",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.End.shortDesc",
				Other: "End the game",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.End.desc",
				Other: "End the current game",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.End.args",
				Other: "None",
			},
			Aliases:    []string{"stop", "e"},
			IsSecret:   false,
			Emoji:      "🛑",
			IsAdmin:    false,
			IsOperator: true,

			fn: commandFnEnd,
		},
		{
			CommandType: CommandEnumPause,
			Command:     "pause",
			Example:     "pause",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Pause.shortDesc",
				Other: "Pause the bot",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Pause.desc",
				Other: "Pause the bot so it doesn't automute/deafen. Will unmute/undeafen all players!",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Pause.args",
				Other: "None",
			},
			Aliases:    []string{"unpause", "p"},
			IsSecret:   false,
			Emoji:      "⏸",
			IsAdmin:    false,
			IsOperator: true,

			fn: commandFnPause,
		},
		{
			CommandType: CommandEnumRefresh,
			Command:     "refresh",
			Example:     "refresh",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Refresh.shortDesc",
				Other: "Refresh the bot status",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Refresh.desc",
				Other: "Recreate the bot status message if it ends up too far in the chat",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Refresh.args",
				Other: "None",
			},
			Aliases:    []string{"reload", "ref", "rel", "r"},
			IsSecret:   false,
			Emoji:      "♻",
			IsAdmin:    false,
			IsOperator: true,

			fn: commandFnRefresh,
		},
		{
			CommandType: CommandEnumLink,
			Command:     "link",
			Example:     "link @Soup red",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Link.shortDesc",
				Other: "Link a Discord User",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Link.desc",
				Other: "Manually link a Discord User to their in-game color or name",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Link.args",
				Other: "<discord User> <in-game color or name>",
			},
			Aliases:    []string{"l"},
			IsSecret:   false,
			Emoji:      "🔗",
			IsAdmin:    false,
			IsOperator: true,

			fn: commandFnLink,
		},
		{
			CommandType: CommandEnumUnlink,
			Command:     "unlink",
			Example:     "unlink @Soup",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Unlink.shortDesc",
				Other: "Unlink a Discord User",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Unlink.desc",
				Other: "Manually unlink a Discord User from their in-game player",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Unlink.args",
				Other: "<discord User>",
			},
			Aliases:    []string{"un", "ul", "u"},
			IsSecret:   false,
			Emoji:      "🚷",
			IsAdmin:    false,
			IsOperator: true,

			fn: commandFnUnlink,
		},
		{
			CommandType: CommandEnumUnmuteAll,
			Command:     "unmuteall",
			Example:     "unmuteall",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.UnmuteAll.shortDesc",
				Other: "Force the bot to unmute all",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.UnmuteAll.desc",
				Other: "Force the bot to unmute all linked players",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.UnmuteAll.args",
				Other: "None",
			},
			Aliases:    []string{"unmute", "ua"},
			IsSecret:   false,
			Emoji:      "🔊",
			IsAdmin:    false,
			IsOperator: true,

			fn: commandFnUnmuteAll,
		},
		{
			CommandType: CommandEnumForce,
			Command:     "force",
			Example:     "force task",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Force.shortDesc",
				Other: "Force the bot to transition",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Force.desc",
				Other: "Force the bot to transition to another game stage, if it doesn't transition properly",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Force.args",
				Other: "<phase name> (task, discuss, or lobby / t,d, or l)",
			},
			Aliases:    []string{"f"},
			IsSecret:   true, // force is broken rn, so hide it
			Emoji:      "📢",
			IsAdmin:    false,
			IsOperator: true,

			fn: func(
				bot *Bot,
				isAdmin bool,
				isPermissioned bool,
				sett *storage.GuildSettings,
				session *discordgo.Session,
				guild *discordgo.Guild,
				message *discordgo.MessageCreate,
				args []string,
				cmd Command,
			) {
			},
		},
		{
			CommandType: CommandEnumMap,
			Command:     "map",
			Example:     "map skeld",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Map.shortDesc",
				Other: "Display an in-game map",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Map.desc",
				Other: "Display an image of an in-game map in the text channel. Two supported versions: simple or detailed",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Map.args",
				Other: "<map_name> (skeld, mira_hq, polus, airship) <version> (optional, simple or detailed)",
			},
			IsSecret:   false,
			Emoji:      "🗺",
			IsAdmin:    false,
			IsOperator: false,

			fn: commandFnMap,
		},
		{
			CommandType: CommandEnumCache,
			Command:     "cache",
			Example:     "cache @Soup",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Cache.shortDesc",
				Other: "View cached usernames",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Cache.desc",
				Other: "View a player's cached in-game names, and/or clear them",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Cache.args",
				Other: "<player> (optionally, \"clear\")",
			},
			Aliases:    []string{"c"},
			IsSecret:   false,
			Emoji:      "📖",
			IsAdmin:    false,
			IsOperator: true,

			fn: commandFnCache,
		},
		{
			CommandType: CommandEnumPrivacy,
			Command:     "privacy",
			Example:     "privacy showme",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Privacy.shortDesc",
				Other: "View AutoMuteUs privacy information",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Privacy.desc",
				Other: "AutoMuteUs privacy and data collection details.\nMore details [here](https://github.com/denverquane/automuteus/blob/master/PRIVACY.md)",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Privacy.args",
				Other: "showme, optin, or optout",
			},
			Aliases:    []string{"private", "priv", "gdpr"},
			IsSecret:   false,
			Emoji:      "🔍",
			IsAdmin:    false,
			IsOperator: false,

			fn: commandFnPrivacy,
		},
		{
			CommandType: CommandEnumSettings,
			Command:     "settings",
			Example:     "settings commandPrefix !",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Settings.shortDesc",
				Other: "Adjust bot settings",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Settings.desc",
				Other: "Adjust the bot settings. Type `{{.CommandPrefix}} settings` with no arguments to see more.",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Settings.args",
				Other: "<setting> <value>",
			},
			Aliases:    []string{"sett", "set", "s"},
			IsSecret:   false,
			Emoji:      "🛠",
			IsAdmin:    true,
			IsOperator: true,

			fn: commandFnSettings,
		},
		{
			CommandType: CommandEnumPremium,
			Command:     "premium",
			Example:     "premium",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Premium.shortDesc",
				Other: "View Premium Bot Features",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Premium.desc",
				Other: "View all the features and perks of Premium AutoMuteUs membership",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Premium.args",
				Other: "None",
			},
			Aliases:    []string{"donate", "paypal", "prem", "$"},
			IsSecret:   false,
			Emoji:      "💎",
			IsAdmin:    false,
			IsOperator: false,

			fn: commandFnPremium,
		},
		{
			CommandType: CommandEnumStats,
			Command:     "stats",
			Example:     "stats @Soup",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Stats.shortDesc",
				Other: "View Player and Guild stats",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Stats.desc",
				Other: "View Player and Guild stats",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Stats.args",
				Other: "<@discord user> or \"guild\"",
			},
			Aliases:    []string{"stat", "st"},
			IsSecret:   false,
			Emoji:      "📊",
			IsAdmin:    false,
			IsOperator: false,

			fn: commandFnStats,
		},
		{
			CommandType: CommandEnumInfo,
			Command:     "info",
			Example:     "info",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Info.shortDesc",
				Other: "View Bot info",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Info.desc",
				Other: "View info about the bot, like total guild number, active games, etc",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Info.args",
				Other: "None",
			},
			Aliases:    []string{"inf", "in", "i"},
			IsSecret:   false,
			Emoji:      "📰",
			IsAdmin:    false,
			IsOperator: false,

			fn: commandFnInfo,
		},
		{
			CommandType: CommandEnumASCII,
			Command:     "ascii",
			Example:     "ascii @Soup t 10",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.Ascii.shortDesc",
				Other: "Print an ASCII crewmate",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.Ascii.desc",
				Other: "Print an ASCII crewmate",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.Ascii.args",
				Other: "<@discord user> <is imposter> (true|false) <x impostor remains> (count)",
			},
			Aliases:    []string{"asc"},
			IsSecret:   true,
			IsAdmin:    false,
			IsOperator: false,

			fn: commandFnASCII,
		},
		{
			CommandType: CommandEnumDebugState,
			Command:     "debugstate",
			Example:     "debugstate",
			ShortDesc: &i18n.Message{
				ID:    "commands.AllCommands.DebugState.shortDesc",
				Other: "View the full state of the Discord Guild Data",
			},
			Description: &i18n.Message{
				ID:    "commands.AllCommands.DebugState.desc",
				Other: "View the full state of the Discord Guild Data",
			},
			Arguments: &i18n.Message{
				ID:    "commands.AllCommands.DebugState.args",
				Other: "None",
			},
			Aliases:    []string{"debug", "ds", "state"},
			IsSecret:   true,
			IsAdmin:    false,
			IsOperator: true,

			fn: commandFnDebugState,
		},
	}

	for _, cmd := range allCommands {
		addCommand(cmd, cmd.Command)
		for _, alias := range cmd.Aliases {
			addCommand(cmd, alias)
		}
	}
}

func addCommand(command Command, key string) {
	if key == "" {
		log.Println(fmt.Sprintf("Provided a blank key for command: %s", command.Command))
		return
	}

	if _, exist := commandMap[key]; exist {
		log.Println(fmt.Sprintf("Conflict in keys: %s => %s", command.Command, key))
		return
	}

	commandMap[key] = command
}

func commandFnHelp(
	_ *Bot,
	isAdmin bool,
	isPermissioned bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	args []string,
	_ Command,
) {
	if len(args[1:]) == 0 {
		embed := helpResponse(isAdmin, isPermissioned, sett.CommandPrefix, allCommands, sett)
		session.ChannelMessageSendEmbed(message.ChannelID, &embed)
		return
	}

	cmd, exists := getCommand(args[1])
	if !exists {
		session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.HandleCommand.Help.notFound",
			Other: "I didn't recognize that command! View `help` for all available commands!",
		}))
		return
	}

	embed := ConstructEmbedForCommand(sett.CommandPrefix, cmd, sett)
	session.ChannelMessageSendEmbed(message.ChannelID, embed)
}

func commandFnNew(
	bot *Bot,
	_ bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	guild *discordgo.Guild,
	message *discordgo.MessageCreate,
	_ []string,
	_ Command,
) {
	bot.handleNewGameMessage(session, message, guild, sett)
}

func commandFnEnd(
	bot *Bot,
	_ bool,
	_ bool,
	_ *storage.GuildSettings,
	_ *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	_ []string,
	_ Command,
) {
	log.Println("User typed end to end the current game")

	gsr := GameStateRequest{
		GuildID:     message.GuildID,
		TextChannel: message.ChannelID,
	}
	dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
	if v, ok := bot.EndGameChannels[dgs.ConnectCode]; ok {
		v <- true
	}
	delete(bot.EndGameChannels, dgs.ConnectCode)

	bot.applyToAll(dgs, false, false)
}

func commandFnPause(
	bot *Bot,
	_ bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	_ []string,
	_ Command,
) {
	gsr := GameStateRequest{
		GuildID:     message.GuildID,
		TextChannel: message.ChannelID,
	}
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	if lock == nil {
		return
	}
	dgs.Running = !dgs.Running

	bot.RedisInterface.SetDiscordGameState(dgs, lock)
	if !dgs.Running {
		bot.applyToAll(dgs, false, false)
	}

	edited := dgs.Edit(session, bot.gameStateResponse(dgs, sett))
	if edited {
		metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
	}
}

func commandFnRefresh(
	bot *Bot,
	_ bool,
	_ bool,
	sett *storage.GuildSettings,
	_ *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	_ []string,
	_ Command,
) {
	gsr := GameStateRequest{
		GuildID:     message.GuildID,
		TextChannel: message.ChannelID,
	}
	bot.RefreshGameStateMessage(gsr, sett)
}

func commandFnLink(
	bot *Bot,
	_ bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	args []string,
	cmd Command,
) {
	if len(args[1:]) < 2 {
		embed := ConstructEmbedForCommand(sett.CommandPrefix, cmd, sett)
		session.ChannelMessageSendEmbed(message.ChannelID, embed)
	} else {
		gsr := GameStateRequest{
			GuildID:     message.GuildID,
			TextChannel: message.ChannelID,
		}
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		if lock == nil {
			return
		}
		bot.linkPlayer(session, dgs, args[1:])
		bot.RedisInterface.SetDiscordGameState(dgs, lock)

		edited := dgs.Edit(session, bot.gameStateResponse(dgs, sett))
		if edited {
			metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
		}
	}
}

func commandFnUnlink(
	bot *Bot,
	_ bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	args []string,
	cmd Command,
) {
	if len(args[1:]) == 0 {
		embed := ConstructEmbedForCommand(sett.CommandPrefix, cmd, sett)
		session.ChannelMessageSendEmbed(message.ChannelID, embed)
	} else {
		userID, err := extractUserIDFromMention(args[1])
		if err != nil {
			log.Println(err)
		} else {
			log.Print(fmt.Sprintf("Removing player %s", userID))
			gsr := GameStateRequest{
				GuildID:     message.GuildID,
				TextChannel: message.ChannelID,
			}
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
			if lock == nil {
				return
			}
			dgs.ClearPlayerData(userID)

			bot.RedisInterface.SetDiscordGameState(dgs, lock)

			// update the state message to reflect the player leaving
			edited := dgs.Edit(session, bot.gameStateResponse(dgs, sett))
			if edited {
				metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
			}
		}
	}
}

func commandFnUnmuteAll(
	bot *Bot,
	_ bool,
	_ bool,
	_ *storage.GuildSettings,
	_ *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	_ []string,
	_ Command,
) {
	gsr := GameStateRequest{
		GuildID:     message.GuildID,
		TextChannel: message.ChannelID,
	}
	dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
	bot.applyToAll(dgs, false, false)
}

func commandFnSettings(
	bot *Bot,
	_ bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	args []string,
	_ Command,
) {
	premStatus, days := bot.PostgresInterface.GetGuildPremiumStatus(message.GuildID)
	isPrem := !premium.IsExpired(premStatus, days)
	bot.HandleSettingsCommand(session, message, sett, args, isPrem)
}

func commandFnMap(
	_ *Bot,
	_ bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	args []string,
	cmd Command,
) {
	if len(args[1:]) == 0 {
		embed := ConstructEmbedForCommand(sett.CommandPrefix, cmd, sett)
		session.ChannelMessageSendEmbed(message.ChannelID, embed)
	} else {
		mapVersion := args[len(args)-1]

		var mapName string
		switch mapVersion {
		case "simple", detailedMapString:
			mapName = strings.Join(args[1:len(args)-1], " ")
		default:
			mapName = strings.Join(args[1:], " ")
			mapVersion = sett.GetMapVersion()
		}
		mapItem, err := NewMapItem(mapName)
		if err != nil {
			log.Println(err)
			session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.HandleCommand.Map.notFound",
				Other: "I don't have a map by that name!",
			}))
			return
		}
		switch mapVersion {
		case "simple":
			session.ChannelMessageSend(message.ChannelID, mapItem.MapImage.Simple)
		case detailedMapString:
			session.ChannelMessageSend(message.ChannelID, mapItem.MapImage.Detailed)
		default:
			log.Println("mapVersion has unexpected value for 'map' command")
		}
	}
}

func commandFnCache(
	bot *Bot,
	_ bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	args []string,
	cmd Command,
) {
	if len(args[1:]) == 0 {
		embed := ConstructEmbedForCommand(sett.CommandPrefix, cmd, sett)
		session.ChannelMessageSendEmbed(message.ChannelID, embed)
	} else {
		userID, err := extractUserIDFromMention(args[1])
		if err != nil {
			log.Println(err)
			session.ChannelMessageSend(message.ChannelID, "I couldn't find a user by that name or ID!")
			return
		}
		if len(args[2:]) == 0 {
			cached := bot.RedisInterface.GetUsernameOrUserIDMappings(message.GuildID, userID)
			if len(cached) == 0 {
				session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.HandleCommand.Cache.emptyCachedNames",
					Other: "I don't have any cached player names stored for that user!",
				}))
			} else {
				buf := bytes.NewBuffer([]byte(sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.HandleCommand.Cache.cachedNames",
					Other: "Cached in-game names:",
				})))
				buf.WriteString("\n```\n")
				for n := range cached {
					buf.WriteString(fmt.Sprintf("%s\n", n))
				}
				buf.WriteString("```")

				session.ChannelMessageSend(message.ChannelID, buf.String())
			}
		} else if strings.ToLower(args[2]) == clearArgumentString || strings.ToLower(args[2]) == "c" {
			err := bot.RedisInterface.DeleteLinksByUserID(message.GuildID, userID)
			if err != nil {
				log.Println(err)
			} else {
				session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.HandleCommand.Cache.Success",
					Other: "Successfully deleted all cached names for that user!",
				}))
			}
		}
	}
}

func commandFnPrivacy(
	bot *Bot,
	_ bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	args []string,
	cmd Command,
) {
	if message.Author != nil {
		var arg = ""
		if len(args[1:]) > 0 {
			arg = args[1]
		}
		if arg == "" || (arg != "showme" && arg != "optin" && arg != "optout") {
			embed := ConstructEmbedForCommand(sett.CommandPrefix, cmd, sett)
			session.ChannelMessageSendEmbed(message.ChannelID, embed)
		} else {
			embed := bot.privacyResponse(message.GuildID, message.Author.ID, arg, sett)
			session.ChannelMessageSendEmbed(message.ChannelID, embed)
		}
	}
}

func commandFnInfo(
	bot *Bot,
	_ bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	_ []string,
	_ Command,
) {
	embed := bot.infoResponse(message.GuildID, sett)
	_, err := session.ChannelMessageSendEmbed(message.ChannelID, embed)
	if err != nil {
		log.Println(err)
	}
}

func commandFnDebugState(
	bot *Bot,
	_ bool,
	_ bool,
	_ *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	_ []string,
	_ Command,
) {
	if message.Author != nil {
		gsr := GameStateRequest{
			GuildID:     message.GuildID,
			TextChannel: message.ChannelID,
		}
		state := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
		if state != nil {
			jBytes, err := json.MarshalIndent(state, "", "  ")
			if err != nil {
				log.Println(err)
			} else {
				for i := 0; i < len(jBytes); i += MaxDebugMessageSize {
					end := i + MaxDebugMessageSize
					if end > len(jBytes) {
						end = len(jBytes)
					}
					session.ChannelMessageSend(message.ChannelID, fmt.Sprintf("```JSON\n%s\n```", jBytes[i:end]))
				}
			}
		}
	}
}

func commandFnASCII(
	bot *Bot,
	_ bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	args []string,
	_ Command,
) {
	if len(args[1:]) == 0 {
		session.ChannelMessageSend(message.ChannelID, ASCIICrewmate)
	} else {
		id, err := extractUserIDFromMention(args[1])
		if id == "" || err != nil {
			session.ChannelMessageSend(message.ChannelID, "I couldn't find a user by that name or ID!")
		} else {
			imposter := false
			count := 1
			if len(args[2:]) > 0 {
				if args[2] == trueString || args[2] == "t" {
					imposter = true
				}
				if len(args[3:]) > 0 {
					if itCount, err := strconv.Atoi(args[3]); err == nil {
						count = itCount
					}
				}
			}
			session.ChannelMessageSend(message.ChannelID, ASCIIStarfield(sett, args[1], imposter, count))
		}
	}
}

func commandFnStats(
	bot *Bot,
	isAdmin bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	args []string,
	cmd Command,
) {
	premStatus, days := bot.PostgresInterface.GetGuildPremiumStatus(message.GuildID)
	isPrem := !premium.IsExpired(premStatus, days)
	if len(args[1:]) == 0 {
		embed := ConstructEmbedForCommand(sett.CommandPrefix, cmd, sett)
		session.ChannelMessageSendEmbed(message.ChannelID, embed)
	} else {
		userID, err := extractUserIDFromMention(args[1])
		if userID == "" || err != nil {
			arg := strings.ReplaceAll(args[1], "\"", "")
			if arg == "g" || arg == "guild" || arg == "server" {
				if len(args) > 2 && args[2] == "reset" {
					if !isAdmin {
						session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
							ID:    "message_handlers.handleResetGuild.noPerms",
							Other: "Only Admins are capable of resetting server stats",
						}))
					} else {
						if len(args) == 3 {
							_, err := session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
								ID:    "commands.StatsCommand.Reset.NoConfirm",
								Other: "Please type `{{.CommandPrefix}} stats guild reset confirm` if you are 100% certain that you wish to **completely reset** your guild's stats!",
							},
								map[string]interface{}{
									"CommandPrefix": sett.CommandPrefix,
								}))
							if err != nil {
								log.Println(err)
							}
						} else if args[3] == "confirm" {
							err := bot.PostgresInterface.DeleteAllGamesForServer(message.GuildID)
							if err != nil {
								session.ChannelMessageSend(message.ChannelID, "Encountered the following error when deleting the server's stats: "+err.Error())
							} else {
								session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
									ID:    "commands.StatsCommand.Reset.Success",
									Other: "Successfully reset your guild's stats!",
								}))
							}
						}
					}
				} else {
					_, err := session.ChannelMessageSendEmbed(message.ChannelID, bot.GuildStatsEmbed(message.GuildID, sett, isPrem))
					if err != nil {
						log.Println(err)
					}
				}
			} else {
				arg = strings.ToUpper(arg)
				log.Println(arg)
				if MatchIDRegex.MatchString(arg) {
					strs := strings.Split(arg, ":")
					if len(strs) < 2 {
						log.Println("Something very wrong with the regex for match/conn codes...")
					} else {
						session.ChannelMessageSendEmbed(message.ChannelID, bot.GameStatsEmbed(message.GuildID, strs[1], strs[0], sett, isPrem))
					}
				} else {
					session.ChannelMessageSend(message.ChannelID, "I didn't recognize that user, you mistyped 'guild', or didn't provide a valid Match ID")
				}
			}
		} else {
			if len(args) > 2 && args[2] == "reset" {
				if !isAdmin {
					session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
						ID:    "message_handlers.handleResetGuild.noPerms",
						Other: "Only Admins are capable of resetting server stats",
					}))
				} else {
					if len(args) == 3 {
						_, err := session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.StatsCommand.ResetUser.NoConfirm",
							Other: "Please type `{{.CommandPrefix}} stats `{{.User}}` reset confirm` if you are 100% certain that you wish to **completely reset** that user's stats!",
						},
							map[string]interface{}{
								"CommandPrefix": sett.CommandPrefix,
								"User":          args[1],
							}))
						if err != nil {
							log.Println(err)
						}
					} else if args[3] == "confirm" {
						err := bot.PostgresInterface.DeleteAllGamesForUser(userID)
						if err != nil {
							session.ChannelMessageSend(message.ChannelID, "Encountered the following error when deleting that user's stats: "+err.Error())
						} else {
							session.ChannelMessageSend(message.ChannelID, sett.LocalizeMessage(&i18n.Message{
								ID:    "commands.StatsCommand.ResetUser.Success",
								Other: "Successfully reset {{.User}}'s stats!",
							},
								map[string]interface{}{
									"User": args[1],
								}))
						}
					}
				}
			} else {
				session.ChannelMessageSendEmbed(message.ChannelID, bot.UserStatsEmbed(userID, message.GuildID, sett, isPrem))
			}
		}
	}
}

func commandFnPremium(
	bot *Bot,
	isAdmin bool,
	_ bool,
	sett *storage.GuildSettings,
	session *discordgo.Session,
	_ *discordgo.Guild,
	message *discordgo.MessageCreate,
	args []string,
	_ Command,
) {
	premStatus, days := bot.PostgresInterface.GetGuildPremiumStatus(message.GuildID)
	if len(args[1:]) == 0 {
		session.ChannelMessageSendEmbed(message.ChannelID, premiumEmbedResponse(message.GuildID, premStatus, days, sett))
	} else {
		tier := premium.FreeTier
		if !premium.IsExpired(premStatus, days) {
			tier = premStatus
		}
		arg := strings.ToLower(args[1])
		if isAdmin {
			if arg == "invite" || arg == "invites" || arg == "inv" {
				_, err := session.ChannelMessageSendEmbed(message.ChannelID, premiumInvitesEmbed(tier, sett))
				if err != nil {
					log.Println(err)
				}
			} else {
				session.ChannelMessageSend(message.ChannelID, "Sorry, I didn't recognize that premium command or argument!")
			}
		} else {
			session.ChannelMessageSend(message.ChannelID, "Viewing the premium invites is an Admin-only command")
		}
	}
}
