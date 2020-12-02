package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/denverquane/amongusdiscord/metrics"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const MaxDebugMessageSize = 1980

type CommandType int

const (
	Help CommandType = iota
	New
	End
	Pause
	Refresh
	Link
	Unlink
	//Track
	UnmuteAll
	Force
	Map
	Settings
	Log
	Cache
	Privacy
	Info
	DebugState
	Ascii
	Stats
	Premium
	Null
)

type Command struct {
	cmdType           CommandType
	command           string
	example           string
	shortDesc         *i18n.Message
	desc              *i18n.Message
	args              *i18n.Message
	aliases           []string
	secret            bool
	emoji             string
	adminSetting      bool
	permissionSetting bool
}

//note, this mapping is HIERARCHICAL. If you type `l`, "link" would be used over "log"
var AllCommands = []Command{
	{
		cmdType: Help,
		command: "help",
		example: "help track",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Help.shortDesc",
			Other: "Display help",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Help.desc",
			Other: "Display bot help message, or see info about a command",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Help.args",
			Other: "None, or optional command to see info for",
		},
		aliases:           []string{"h"},
		secret:            false,
		emoji:             "❓",
		adminSetting:      false,
		permissionSetting: false,
	},
	{
		cmdType: New,
		command: "new",
		example: "new",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.New.shortDesc",
			Other: "Start a new game",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.New.desc",
			Other: "Start a new game",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.New.args",
			Other: "None",
		},
		aliases:           []string{"start", "n"},
		secret:            false,
		emoji:             "🕹",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: End,
		command: "end",
		example: "end",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.End.shortDesc",
			Other: "End the game",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.End.desc",
			Other: "End the current game",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.End.args",
			Other: "None",
		},
		aliases:           []string{"stop", "e"},
		secret:            false,
		emoji:             "🛑",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: Pause,
		command: "pause",
		example: "pause",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Pause.shortDesc",
			Other: "Pause the bot",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Pause.desc",
			Other: "Pause the bot so it doesn't automute/deafen. Will unmute/undeafen all players!",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Pause.args",
			Other: "None",
		},
		aliases:           []string{"p"},
		secret:            false,
		emoji:             "⏸",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: Refresh,
		command: "refresh",
		example: "refresh",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Refresh.shortDesc",
			Other: "Refresh the bot status",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Refresh.desc",
			Other: "Recreate the bot status message if it ends up too far in the chat",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Refresh.args",
			Other: "None",
		},
		aliases:           []string{"reload", "r"},
		secret:            false,
		emoji:             "♻",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: Link,
		command: "link",
		example: "link @Soup red",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Link.shortDesc",
			Other: "Link a Discord User",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Link.desc",
			Other: "Manually link a Discord User to their in-game color or name",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Link.args",
			Other: "<discord User> <in-game color or name>",
		},
		aliases:           []string{"l"},
		secret:            false,
		emoji:             "🔗",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: Unlink,
		command: "unlink",
		example: "unlink @Soup",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Unlink.shortDesc",
			Other: "Unlink a Discord User",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Unlink.desc",
			Other: "Manually unlink a Discord User from their in-game player",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Unlink.args",
			Other: "<discord User>",
		},
		aliases:           []string{"u", "ul"},
		secret:            false,
		emoji:             "🚷",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: UnmuteAll,
		command: "unmuteall",
		example: "unmuteall",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.UnmuteAll.shortDesc",
			Other: "Force the bot to unmute all",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.UnmuteAll.desc",
			Other: "Force the bot to unmute all linked players",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.UnmuteAll.args",
			Other: "None",
		},
		aliases:           []string{"unmute", "ua"},
		secret:            false,
		emoji:             "🔊",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: Force,
		command: "force",
		example: "force task",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Force.shortDesc",
			Other: "Force the bot to transition",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Force.desc",
			Other: "Force the bot to transition to another game stage, if it doesn't transition properly",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Force.args",
			Other: "<phase name> (task, discuss, or lobby / t,d, or l)",
		},
		aliases:           []string{"f"},
		secret:            true, //force is broken rn, so hide it
		emoji:             "📢",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: Map,
		command: "map",
		example: "map skeld",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Map.shortDesc",
			Other: "Display an in-game map",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Map.desc",
			Other: "Display an image of an in-game map in the text channel. Two supported versions: simple or detailed",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Map.args",
			Other: "<map_name> (skeld, mira_hq, polus) <version> (optional, simple or detailed)",
		},
		aliases:           []string{"map"},
		secret:            false,
		emoji:             "🗺",
		adminSetting:      false,
		permissionSetting: false,
	},
	{
		cmdType: Cache,
		command: "cache",
		example: "cache @Soup",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Cache.shortDesc",
			Other: "View cached usernames",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Cache.desc",
			Other: "View a player's cached in-game names, and/or clear them",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Cache.args",
			Other: "<player> (optionally, \"clear\")",
		},
		aliases:           []string{"c"},
		secret:            false,
		emoji:             "📖",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: Privacy,
		command: "privacy",
		example: "privacy showme",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Privacy.shortDesc",
			Other: "View AutoMuteUs privacy information",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Privacy.desc",
			Other: "AutoMuteUs privacy and data collection details.\nMore details [here](https://github.com/denverquane/automuteus/blob/master/PRIVACY.md)",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Privacy.args",
			Other: "showme, optin, or optout",
		},
		aliases:           []string{"private", "priv"},
		secret:            false,
		emoji:             "🔍",
		adminSetting:      false,
		permissionSetting: false,
	},
	{
		cmdType: Settings,
		command: "settings",
		example: "settings commandPrefix !",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Settings.shortDesc",
			Other: "Adjust bot settings",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Settings.desc",
			Other: "Adjust the bot settings. Type `{{.CommandPrefix}} settings` with no arguments to see more.",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Settings.args",
			Other: "<setting> <value>",
		},
		aliases:           []string{"s"},
		secret:            false,
		emoji:             "🛠",
		adminSetting:      true,
		permissionSetting: true,
	},
	{
		cmdType: Log,
		command: "log",
		example: "log something bad happened",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Log.shortDesc",
			Other: "Log a weird event",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Log.desc",
			Other: "Log if something bad happened, so you can find the event in your logs later",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Log.args",
			Other: "<message>",
		},
		aliases:           []string{"log"},
		secret:            true,
		emoji:             "⁉",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: Premium,
		command: "premium",
		example: "premium",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Premium.shortDesc",
			Other: "View Premium Bot Features",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Premium.desc",
			Other: "View all the features and perks of Premium AutoMuteUs membership",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Premium.args",
			Other: "None",
		},
		aliases:           []string{"donate", "prem"},
		secret:            false,
		emoji:             "💎",
		adminSetting:      false,
		permissionSetting: false,
	},
	{
		cmdType: Stats,
		command: "stats",
		example: "stats @Soup",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Stats.shortDesc",
			Other: "View Player and Guild stats",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Stats.desc",
			Other: "View Player and Guild stats",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Stats.args",
			Other: "<@discord user> or \"guild\"",
		},
		aliases:           []string{"stat"},
		secret:            false,
		emoji:             "📊",
		adminSetting:      false,
		permissionSetting: false,
	},
	{
		cmdType: Info,
		command: "info",
		example: "info",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Info.shortDesc",
			Other: "View Bot info",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Info.desc",
			Other: "View info about the bot, like total guild number, active games, etc",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Info.args",
			Other: "None",
		},
		aliases:           []string{"info", "i"},
		secret:            false,
		emoji:             "📰",
		adminSetting:      false,
		permissionSetting: false,
	},
	{
		cmdType: Ascii,
		command: "ascii",
		example: "ascii @Soup t 10",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Ascii.shortDesc",
			Other: "Print an ASCII crewmate",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Ascii.desc",
			Other: "Print an ASCII crewmate",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Ascii.args",
			Other: "<@discord user> <is imposter> (true|false) <x impostor remains> (count)",
		},
		aliases:           []string{"ascii", "asc"},
		secret:            true,
		adminSetting:      false,
		permissionSetting: false,
	},
	{
		cmdType: DebugState,
		command: "debugstate",
		example: "debugstate",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.DebugState.shortDesc",
			Other: "View the full state of the Discord Guild Data",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.DebugState.desc",
			Other: "View the full state of the Discord Guild Data",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.DebugState.args",
			Other: "None",
		},
		aliases:           []string{"debug", "ds", "state"},
		secret:            true,
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType:           Null,
		command:           "",
		example:           "",
		shortDesc:         nil,
		desc:              nil,
		args:              nil,
		aliases:           []string{""},
		secret:            true,
		adminSetting:      true,
		permissionSetting: true,
	},
}

//TODO cache/preconstruct these (no reason to make them fresh everytime help is called, except for the prefix...)
func ConstructEmbedForCommand(prefix string, cmd Command, sett *storage.GuildSettings) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		URL:   "",
		Type:  "",
		Title: cmd.emoji + " " + strings.Title(cmd.command),
		Description: sett.LocalizeMessage(cmd.desc,
			map[string]interface{}{
				"CommandPrefix": sett.CommandPrefix,
			}),
		Timestamp: "",
		Color:     15844367, //GOLD
		Image:     nil,
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.ConstructEmbedForCommand.Fields.Example",
					Other: "Example",
				}),
				Value:  "`" + fmt.Sprintf("%s %s", prefix, cmd.example) + "`",
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.ConstructEmbedForCommand.Fields.Arguments",
					Other: "Arguments",
				}),
				Value:  "`" + sett.LocalizeMessage(cmd.args) + "`",
				Inline: false,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.ConstructEmbedForCommand.Fields.Aliases",
					Other: "Aliases",
				}),
				Value:  strings.Join(cmd.aliases, ", "),
				Inline: false,
			},
		},
	}
}

func GetCommand(arg string) Command {
	arg = strings.ToLower(arg)
	for _, cmd := range AllCommands {
		if arg == cmd.command {
			return cmd
		} else {
			for _, al := range cmd.aliases {
				if arg == al {
					return cmd
				}
			}
		}
	}
	return AllCommands[Null]
}

func (bot *Bot) HandleCommand(isAdmin, isPermissioned bool, sett *storage.GuildSettings, s *discordgo.Session, g *discordgo.Guild, m *discordgo.MessageCreate, args []string) {
	prefix := sett.CommandPrefix
	cmd := GetCommand(args[0])

	gsr := GameStateRequest{
		GuildID:     m.GuildID,
		TextChannel: m.ChannelID,
	}

	if cmd.cmdType != Null {
		log.Print(fmt.Sprintf("\"%s\" command typed by User %s\n", cmd.command, m.Author.ID))
	}

	//only allow admins to execute admin commands
	if cmd.adminSetting && !isAdmin {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "message_handlers.handleMessageCreate.noPerms",
			Other: "User does not have the required permissions to execute this command!",
		}))
	} else if cmd.permissionSetting && (!isPermissioned && !isAdmin) {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "message_handlers.handleMessageCreate.noPerms",
			Other: "User does not have the required permissions to execute this command!",
		}))
	} else {
		//broadly speaking, most commands issue at minimum 1 discord request, and delete a user's message.
		//Very approximately, at least
		bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, 2)

		switch cmd.cmdType {
		case Help:
			if len(args[1:]) == 0 {
				embed := helpResponse(isAdmin, isPermissioned, prefix, AllCommands, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, &embed)
			} else {
				cmd = GetCommand(args[1])
				if cmd.cmdType != Null {
					embed := ConstructEmbedForCommand(prefix, cmd, sett)
					s.ChannelMessageSendEmbed(m.ChannelID, embed)
				} else {
					s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.HandleCommand.Help.notFound",
						Other: "I didn't recognize that command! View `help` for all available commands!",
					}))
				}
			}
			break

		case New:
			room, region := getRoomAndRegionFromArgs(args[1:], sett)

			bot.handleNewGameMessage(s, m, g, sett, room, region)
			break

		case End:
			log.Println("User typed end to end the current game")

			dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
			if v, ok := bot.EndGameChannels[dgs.ConnectCode]; ok {
				v <- EndGameMessage{EndGameType: EndAndWipe}
			}
			delete(bot.EndGameChannels, dgs.ConnectCode)

			bot.applyToAll(dgs, false, false)

			break

		case Pause:
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
			if lock == nil {
				break
			}
			dgs.Running = !dgs.Running

			bot.RedisInterface.SetDiscordGameState(dgs, lock)
			if !dgs.Running {
				bot.applyToAll(dgs, false, false)
			}

			edited := dgs.Edit(s, bot.gameStateResponse(dgs, sett))
			if edited {
				bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
			}
			break

		case Refresh:
			bot.RefreshGameStateMessage(gsr, sett)
			break
		case Link:
			if len(args[1:]) < 2 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {
				lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
				if lock == nil {
					break
				}
				bot.linkPlayer(s, dgs, args[1:])
				bot.RedisInterface.SetDiscordGameState(dgs, lock)

				edited := dgs.Edit(s, bot.gameStateResponse(dgs, sett))
				if edited {
					bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
				}
			}
			break

		case Unlink:
			if len(args[1:]) == 0 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {

				userID, err := extractUserIDFromMention(args[1])
				if err != nil {
					log.Println(err)
				} else {
					log.Print(fmt.Sprintf("Removing player %s", userID))
					lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
					if lock == nil {
						break
					}
					dgs.ClearPlayerData(userID)

					bot.RedisInterface.SetDiscordGameState(dgs, lock)

					//update the state message to reflect the player leaving
					edited := dgs.Edit(s, bot.gameStateResponse(dgs, sett))
					if edited {
						bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
					}
				}
			}
			break
		case UnmuteAll:
			dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
			bot.applyToAll(dgs, false, false)
			break

		case Settings:
			premStatus := bot.PostgresInterface.GetGuildPremiumStatus(m.GuildID)
			bot.HandleSettingsCommand(s, m, sett, args, premStatus != 0)
			break

		case Log:
			log.Println(fmt.Sprintf("\"%s\"", strings.Join(args, " ")))
			break

		case Map:
			if len(args[1:]) == 0 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {
				mapVersion := args[len(args)-1]

				var mapName string
				switch mapVersion {
				case "simple", "detailed":
					mapName = strings.Join(args[1:len(args)-1], " ")
				default:
					mapName = strings.Join(args[1:], " ")
					mapVersion = sett.GetMapVersion()
				}
				mapItem, err := NewMapItem(mapName)
				if err != nil {
					log.Println(err)
					s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.HandleCommand.Map.notFound",
						Other: "I don't have a map by that name!",
					}))
					break
				}
				switch mapVersion {
				case "simple":
					s.ChannelMessageSend(m.ChannelID, mapItem.MapImage.Simple)
				case "detailed":
					s.ChannelMessageSend(m.ChannelID, mapItem.MapImage.Detailed)
				default:
					log.Println("mapVersion has unexpected value for 'map' command")
				}
			}
			break

		case Cache:
			if len(args[1:]) == 0 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {
				userID, err := extractUserIDFromMention(args[1])
				if err != nil {
					log.Println(err)
					s.ChannelMessageSend(m.ChannelID, "I couldn't find a user by that name or ID!")
					break
				}
				if len(args[2:]) == 0 {
					cached := bot.RedisInterface.GetUsernameOrUserIDMappings(m.GuildID, userID)
					if len(cached) == 0 {
						s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
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

						s.ChannelMessageSend(m.ChannelID, buf.String())
					}
				} else if strings.ToLower(args[2]) == "clear" || strings.ToLower(args[2]) == "c" {
					err := bot.RedisInterface.DeleteLinksByUserID(m.GuildID, userID)
					if err != nil {
						log.Println(err)
					} else {
						s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.HandleCommand.Cache.Success",
							Other: "Successfully deleted all cached names for that user!",
						}))
					}
				}
			}
			break

		case Privacy:
			if m.Author != nil {
				var arg = ""
				if len(args[1:]) > 0 {
					arg = args[1]
				}
				if arg == "" || (arg != "showme" && arg != "optin" && arg != "optout") {
					embed := ConstructEmbedForCommand(prefix, cmd, sett)
					s.ChannelMessageSendEmbed(m.ChannelID, embed)
				} else {
					embed := bot.privacyResponse(m.GuildID, m.Author.ID, arg, sett)
					s.ChannelMessageSendEmbed(m.ChannelID, embed)
				}
			}
			break

		case Info:
			embed := bot.infoResponse(sett)
			_, err := s.ChannelMessageSendEmbed(m.ChannelID, embed)
			if err != nil {
				log.Println(err)
			}
			break

		case DebugState:
			if m.Author != nil {
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
							s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```JSON\n%s\n```", jBytes[i:end]))
						}
					}
				}
			}
			break

		case Ascii:
			if len(args[1:]) == 0 {
				s.ChannelMessageSend(m.ChannelID, AsciiCrewmate)
			} else {
				id, err := extractUserIDFromMention(args[1])
				if id == "" || err != nil {
					s.ChannelMessageSend(m.ChannelID, "I couldn't find a user by that name or ID!")
				} else {
					imposter := false
					count := 1
					if len(args[2:]) > 0 {
						if args[2] == "true" || args[2] == "t" {
							imposter = true
						}
						if len(args[3:]) > 0 {
							if itCount, err := strconv.Atoi(args[3]); err == nil {
								count = itCount
							}
						}
					}
					s.ChannelMessageSend(m.ChannelID, AsciiStarfield(sett, args[1], imposter, count))
				}
			}
			break
		case Stats:
			premStatus := bot.PostgresInterface.GetGuildPremiumStatus(m.GuildID)
			if len(args[1:]) == 0 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {
				userID, err := extractUserIDFromMention(args[1])
				if userID == "" || err != nil {
					if strings.ReplaceAll(strings.ToLower(args[1]), "\"", "") == "guild" {
						_, err := s.ChannelMessageSendEmbed(m.ChannelID, bot.GuildStatsEmbed(m.GuildID, sett, premStatus))
						if err != nil {
							log.Println(err)
						}
					} else {
						s.ChannelMessageSend(m.ChannelID, "I didn't recognize that user, or you mistyped 'guild'...")
					}

				} else {
					s.ChannelMessageSendEmbed(m.ChannelID, bot.UserStatsEmbed(userID, m.GuildID, sett, premStatus))
				}

			}
			break
		case Premium:
			premStatus := bot.PostgresInterface.GetGuildPremiumStatus(m.GuildID)
			if len(args[1:]) == 0 {
				s.ChannelMessageSendEmbed(m.ChannelID, premiumEmbedResponse(premStatus, sett))
			} else {
				arg := strings.ToLower(args[1])
				if isAdmin {
					if arg == "invite" || arg == "inv" {
						_, err := s.ChannelMessageSendEmbed(m.ChannelID, premiumInvitesEmbed(premStatus, sett))
						if err != nil {
							log.Println(err)
						}
					}
				} else {
					s.ChannelMessageSend(m.ChannelID, "Viewing the premium invites is an Admin command")
				}
			}
			break
		default:
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.HandleCommand.default",
				Other: "Sorry, I didn't understand that command! Please see `{{.CommandPrefix}} help` for commands",
			},
				map[string]interface{}{
					"CommandPrefix": prefix,
				}))
			break
		}
	}

	deleteMessage(s, m.ChannelID, m.Message.ID)
}
