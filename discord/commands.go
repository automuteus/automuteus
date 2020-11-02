package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type CommandType int

const (
	Help CommandType = iota
	New
	End
	Pause
	Refresh
	Link
	Unlink
	Track
	UnmuteAll
	Force
	Settings
	Log
	Cache
	ShowMe
	ForgetMe
	DebugState
	Ascii
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
		aliases:           []string{"n"},
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
		aliases:           []string{"e"},
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
			Other: "Pause the bot so it doesn't automute/deafen. **Will not unmute/undeafen**",
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
		aliases:           []string{"r"},
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
		aliases:           []string{"u"},
		secret:            false,
		emoji:             "🚷",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: Track,
		command: "track",
		example: "track Among Us Voice",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.Track.shortDesc",
			Other: "Track a voice channel",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.Track.desc",
			Other: "Tell the bot which voice channel you'll be playing in",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Track.args",
			Other: "<voice channel name>",
		},
		aliases:           []string{"t"},
		secret:            false,
		emoji:             "📌",
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
		secret:            false,
		emoji:             "📢",
		adminSetting:      false,
		permissionSetting: true,
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
			Other: "Adjust the bot settings. Type `.au settings` with no arguments to see more.",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.Settings.args",
			Other: "<setting> <value>",
		},
		aliases:           []string{"s"},
		secret:            false,
		emoji:             "⚙",
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
		secret:            false,
		emoji:             "⁉",
		adminSetting:      false,
		permissionSetting: true,
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
		aliases:           []string{"cache"},
		secret:            false,
		emoji:             "📖",
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: ShowMe,
		command: "showme",
		example: "showme",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.ShowMe.shortDesc",
			Other: "Show player data",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.ShowMe.desc",
			Other: "Send all the player data for the User issuing the command",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.ShowMe.args",
			Other: "None",
		},
		aliases:           []string{"sm"},
		secret:            false,
		emoji:             "🔍",
		adminSetting:      false,
		permissionSetting: false,
	},
	{
		cmdType: ForgetMe,
		command: "forgetme",
		example: "forgetme",
		shortDesc: &i18n.Message{
			ID:    "commands.AllCommands.ForgetMe.shortDesc",
			Other: "Delete player data",
		},
		desc: &i18n.Message{
			ID:    "commands.AllCommands.ForgetMe.desc",
			Other: "Delete all the data associated with the User issuing the command",
		},
		args: &i18n.Message{
			ID:    "commands.AllCommands.ForgetMe.args",
			Other: "None",
		},
		aliases:           []string{"fm"},
		secret:            false,
		emoji:             "🗑",
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
		aliases:           []string{"ds"},
		secret:            true,
		adminSetting:      false,
		permissionSetting: true,
	},
	{
		cmdType: Ascii,
		command: "ascii",
		example: "ascii",
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
			Other: "None",
		},
		aliases:           []string{"ascii"},
		secret:            true,
		adminSetting:      false,
		permissionSetting: false,
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
	if cmd.cmdType == Settings {
		return settingResponse(prefix, AllSettings, sett)
	}
	return &discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       cmd.emoji + " " + strings.Title(cmd.command),
		Description: sett.LocalizeMessage(cmd.desc),
		Timestamp:   "",
		Color:       15844367, //GOLD
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
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
		if len(arg) == 1 {
			if cmd.cmdType != Null && cmd.command[0] == arg[0] {
				return cmd
			}
		} else {
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
		switch cmd.cmdType {
		case Help:
			if len(args[1:]) == 0 {
				embed := helpResponse(isAdmin, isPermissioned, Version, prefix, AllCommands, sett)
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

			deleteMessage(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)

			break

		case Pause:
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
			if lock == nil {
				break
			}
			dgs.Running = !dgs.Running
			bot.RedisInterface.SetDiscordGameState(dgs, lock)

			dgs.Edit(s, bot.gameStateResponse(dgs, sett))
			break

		case Refresh:
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
			if lock == nil {
				break
			}
			dgs.DeleteGameStateMsg(s) //delete the old message

			//create a new instance of the new one
			dgs.CreateMessage(s, bot.gameStateResponse(dgs, sett), m.ChannelID, dgs.GameStateMsg.LeaderID)

			bot.RedisInterface.SetDiscordGameState(dgs, lock)
			//add the emojis to the refreshed message if in the right stage
			if dgs.AmongUsData.GetPhase() != game.MENU {
				for _, e := range bot.StatusEmojis[true] {
					dgs.AddReaction(s, e.FormatForReaction())
				}
				dgs.AddReaction(s, "❌")
			}
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

				dgs.Edit(s, bot.gameStateResponse(dgs, sett))
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
					dgs.Edit(s, bot.gameStateResponse(dgs, sett))
				}
			}
			break

		case Track:
			if len(args[1:]) == 0 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {
				channelName := strings.Join(args[1:], " ")

				channels, err := s.GuildChannels(m.GuildID)
				if err != nil {
					log.Println(err)
				}

				lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
				if lock == nil {
					break
				}
				dgs.trackChannel(channelName, channels, sett)
				bot.RedisInterface.SetDiscordGameState(dgs, lock)

				dgs.Edit(s, bot.gameStateResponse(dgs, sett))
			}
			break
		case UnmuteAll:
			dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
			bot.applyToAll(dgs, false, false)
			break

		case Force:
			if len(args[1:]) == 0 {
				embed := ConstructEmbedForCommand(prefix, cmd, sett)
				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			} else {
				phase := getPhaseFromString(args[1])
				if phase == game.UNINITIALIZED {
					s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.HandleCommand.Force.UNINITIALIZED",
						Other: "Sorry, I didn't understand the game phase you tried to force",
					}))
				} else {
					dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
					if dgs.ConnectCode != "" {
						i := strconv.FormatInt(int64(phase), 10)
						bot.RedisInterface.PublishPhaseUpdate(dgs.ConnectCode, i)
					}
				}
			}
			break

		case Settings:
			bot.HandleSettingsCommand(s, m, sett, args)
			break

		case Log:
			log.Println(fmt.Sprintf("\"%s\"", strings.Join(args, " ")))
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

		case ShowMe:
			if m.Author != nil {
				if settUser := bot.StorageInterface.GetUserSettings(m.Author.ID); settUser != nil {
					embed := settUser.ToEmbed(sett)
					sendMessageDM(s, m.Author.ID, embed)
				} else {
					s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.HandleCommand.ShowMe.emptySettings",
						Other: "I don't have any settings stored for you!",
					}))
				}

				cached := bot.RedisInterface.GetUsernameOrUserIDMappings(m.GuildID, m.Author.ID)
				if len(cached) == 0 {
					s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.HandleCommand.ShowMe.emptyCachedNames",
						Other: "I don't have any cached player names stored for you!",
					}))
				} else {
					buf := bytes.NewBuffer([]byte(sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.HandleCommand.ShowMe.cachedNames",
						Other: "Here's your cached in-game names:",
					})))
					buf.WriteString("\n```\n")
					for n := range cached {
						buf.WriteString(fmt.Sprintf("%s\n", n))
					}
					buf.WriteString("```")
					s.ChannelMessageSend(m.ChannelID, buf.String())
				}
			}
			break
		case ForgetMe:
			if m.Author != nil {
				err := bot.StorageInterface.DeleteUserSettings(m.Author.ID)
				if err != nil {
					log.Println(err)
				} else {
					err := bot.RedisInterface.DeleteLinksByUserID(m.GuildID, m.Author.ID)
					if err != nil {
						log.Println(err)
					} else {
						s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.HandleCommand.ForgetMe.Success",
							Other: "Successfully deleted all player data for <@{{.AuthorID}}>",
						},
							map[string]interface{}{
								"AuthorID": m.Author.ID,
							}))
					}
				}
			}
			break

		case DebugState:
			if m.Author != nil {
				state := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
				if state != nil {
					jBytes, err := json.MarshalIndent(state, "", "  ")
					if len(jBytes) > 1980 {
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```JSON\n%s", jBytes[0:1980]))
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s\n```", jBytes[1980:]))
					} else {
						if err != nil {
							log.Println(err)
						}
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```JSON\n%s\n```", jBytes))
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
					if len(args[2:]) > 0 {
						if args[2] == "true" || args[2] == "t" {
							imposter = true
						}
					}
					s.ChannelMessageSend(m.ChannelID, AsciiStarfield(args[1], imposter))
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
