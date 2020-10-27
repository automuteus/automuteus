package discord

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	"log"
	"strings"
)

type CommandType int

const (
	Help CommandType = iota
	Track
	Link
	Unlink
	New
	End
	Force
	Refresh
	Settings
	Pause
	Log
	ShowMe
	ForgetMe
	DebugState
	Ascii
	Null
)

type Command struct {
	cmdType   CommandType
	command   string
	example   string
	shortDesc string
	desc      string
	args      string
	aliases   []string
	secret    bool
}

//note, this mapping is HIERARCHICAL. If you type `l`, "link" would be used over "log"
var AllCommands = []Command{
	{
		cmdType:   Help,
		command:   "help",
		example:   "help track",
		shortDesc: "Display help",
		desc:      "Display bot help message, or see info about a command",
		args:      "None, or optional command to see info for",
		aliases:   []string{"h"},
		secret:    false,
	},
	{
		cmdType:   Track,
		command:   "track",
		example:   "track Among Us Voice",
		shortDesc: "Track a voice channel",
		desc:      "Tell the bot which voice channel you'll be playing in",
		args:      "<voice channel name>",
		aliases:   []string{"t"},
		secret:    false,
	},
	{
		cmdType:   Link,
		command:   "link",
		example:   "link @Soup red",
		shortDesc: "Link a Discord User",
		desc:      "Manually link a Discord User to their in-game color or name",
		args:      "<discord User> <in-game color or name>",
		aliases:   []string{"l"},
		secret:    false,
	},
	{
		cmdType:   Unlink,
		command:   "unlink",
		example:   "unlink @Soup",
		shortDesc: "Unlink a Discord User",
		desc:      "Manually unlink a Discord User from their in-game player",
		args:      "<discord User>",
		aliases:   []string{"u"},
		secret:    false,
	},
	{
		cmdType:   New,
		command:   "new",
		example:   "new",
		shortDesc: "Start a new game",
		desc:      "Start a new game",
		args:      "None",
		aliases:   []string{"n"},
		secret:    false,
	},
	{
		cmdType:   End,
		command:   "end",
		example:   "end",
		shortDesc: "End the game",
		desc:      "End the current game",
		args:      "None",
		aliases:   []string{"e"},
		secret:    false,
	},
	{
		cmdType:   Force,
		command:   "force",
		example:   "force task",
		shortDesc: "Force the bot to transition",
		desc:      "Force the bot to transition to another game stage, if it doesn't transition properly",
		args:      "<phase name> (task, discuss, or lobby / t,d, or l)",
		aliases:   []string{"f"},
		secret:    false,
	},
	{
		cmdType:   Refresh,
		command:   "refresh",
		example:   "refresh",
		shortDesc: "Refresh the bot status",
		desc:      "Recreate the bot status message if it ends up too far in the chat",
		args:      "None",
		aliases:   []string{"r"},
		secret:    false,
	},
	{
		cmdType:   Settings,
		command:   "settings",
		example:   "settings commandPrefix !",
		shortDesc: "Adjust bot settings",
		desc:      "Adjust the bot settings. Type `.au settings` with no arguments to see more.",
		args:      "<setting> <value>",
		aliases:   []string{"s"},
		secret:    false,
	},
	{
		cmdType:   Pause,
		command:   "pause",
		example:   "pause",
		shortDesc: "Pause the bot",
		desc:      "Pause the bot so it doesn't automute/deafen. **Will not unmute/undeafen**",
		args:      "None",
		aliases:   []string{"p"},
		secret:    false,
	},
	{
		cmdType:   Log,
		command:   "log",
		example:   "log something bad happened",
		shortDesc: "Log a weird event",
		desc:      "Log if something bad happened, so you can find the event in your logs later",
		args:      "<message>",
		aliases:   []string{"log"},
		secret:    false,
	},
	{
		cmdType:   ShowMe,
		command:   "showme",
		example:   "showme",
		shortDesc: "Show player settings",
		desc:      "Send all the player data for the User issuing the command",
		args:      "None",
		aliases:   []string{"sm"},
		secret:    false,
	},
	{
		cmdType:   ForgetMe,
		command:   "forgetme",
		example:   "forgetme",
		shortDesc: "Delete player settings",
		desc:      "Delete all the data associated with the User issuing the command",
		args:      "None",
		aliases:   []string{"fm"},
		secret:    false,
	},
	{
		cmdType:   DebugState,
		command:   "debugstate",
		example:   "debugstate",
		shortDesc: "View the full state of the Discord Guild Data",
		desc:      "View the full state of the Discord Guild Data",
		args:      "None",
		aliases:   []string{"ds"},
		secret:    true,
	},
	{
		cmdType:   Ascii,
		command:   "ascii",
		example:   "ascii",
		shortDesc: "Print an ASCII crewmate",
		desc:      "Print an ASCII crewmate",
		args:      "None",
		aliases:   []string{"ascii"},
		secret:    true,
	},
	{
		cmdType:   Null,
		command:   "",
		example:   "",
		shortDesc: "",
		desc:      "",
		args:      "",
		aliases:   []string{""},
		secret:    true,
	},
}

//TODO cache/preconstruct these (no reason to make them fresh everytime help is called, except for the prefix...)
func ConstructEmbedForCommand(prefix string, cmd Command) discordgo.MessageEmbed {
	if cmd.cmdType == Settings {
		return settingResponse(prefix, AllSettings)
	}
	return discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       strings.Title(cmd.command),
		Description: cmd.desc,
		Timestamp:   "",
		Color:       15844367, //GOLD
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{
				Name:   "Example",
				Value:  "`" + fmt.Sprintf("%s %s", prefix, cmd.example) + "`",
				Inline: false,
			},
			&discordgo.MessageEmbedField{
				Name:   "Arguments",
				Value:  "`" + cmd.args + "`",
				Inline: false,
			},
			&discordgo.MessageEmbedField{
				Name:   "Aliases",
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

func (bot *Bot) HandleCommand(sett *storage.GuildSettings, s *discordgo.Session, g *discordgo.Guild, m *discordgo.MessageCreate, args []string) {
	prefix := sett.CommandPrefix
	cmd := GetCommand(args[0])

	if cmd.cmdType != Null {
		log.Print(fmt.Sprintf("\"%s\" command typed by User %s\n", cmd.command, m.Author.ID))
	}

	switch cmd.cmdType {
	case Help:
		if len(args[1:]) == 0 {
			embed := helpResponse(Version, prefix, AllCommands)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)

		} else {
			cmd = GetCommand(args[1])
			if cmd.cmdType != Null {
				embed := ConstructEmbedForCommand(prefix, cmd)
				s.ChannelMessageSendEmbed(m.ChannelID, &embed)
			} else {
				s.ChannelMessageSend(m.ChannelID, "I didn't recognize that command! View `help` for all available commands!")
			}
		}
		break

	case Track:
		if len(args[1:]) == 0 {
			embed := ConstructEmbedForCommand(prefix, cmd)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		} else {
			channelName := strings.Join(args[1:], " ")

			channels, err := s.GuildChannels(m.GuildID)
			if err != nil {
				log.Println(err)
			}

			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(m.GuildID, m.ChannelID, "", "")
			dgs.trackChannel(channelName, channels)
			bot.RedisInterface.SetDiscordGameState(dgs, lock)

			dgs.Edit(s, bot.gameStateResponse(dgs))
		}
		break

	case Link:
		if len(args[1:]) < 2 {
			embed := ConstructEmbedForCommand(prefix, cmd)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		} else {
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(m.GuildID, m.ChannelID, "", "")
			bot.linkPlayer(s, dgs, args[1:])
			bot.RedisInterface.SetDiscordGameState(dgs, lock)

			dgs.Edit(s, bot.gameStateResponse(dgs))
		}
		break

	case Unlink:
		if len(args[1:]) == 0 {
			embed := ConstructEmbedForCommand(prefix, cmd)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		} else {

			userID, err := extractUserIDFromMention(args[1])
			if err != nil {
				log.Println(err)
			} else {
				log.Print(fmt.Sprintf("Removing player %s", userID))
				lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(m.GuildID, m.ChannelID, "", "")
				dgs.ClearPlayerData(userID)

				//make sure that any players we remove/unlink get auto-unmuted/undeafened
				dgs.verifyVoiceStateChanges(s, sett, dgs.GetPhase())
				bot.RedisInterface.SetDiscordGameState(dgs, lock)

				//update the state message to reflect the player leaving
				dgs.Edit(s, bot.gameStateResponse(dgs))
			}
		}
		break

	case New:
		room, region := getRoomAndRegionFromArgs(args[1:])

		bot.handleNewGameMessage(s, m, g, room, region)
		break

	case End:
		log.Println("User typed end to end the current game")

		bot.endGame(m.GuildID, m.ChannelID, "", "", s)

		//only need read-only for deletion (the delete method is locking)
		dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(m.GuildID, m.ChannelID, "", "")
		bot.RedisInterface.DeleteDiscordGameState(dgs)

		//have to explicitly delete here, because if we use the default delete below, the ChannelID
		//for the game state message doesn't exist anymore...
		deleteMessage(s, m.ChannelID, m.Message.ID)
		break

	case Force:
		if len(args[1:]) == 0 {
			embed := ConstructEmbedForCommand(prefix, cmd)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		} else {
			phase := getPhaseFromString(args[1])
			if phase == game.UNINITIALIZED {
				s.ChannelMessageSend(m.ChannelID, "Sorry, I didn't understand the game phase you tried to force")
			} else {
				log.Print("FORCE IS BROKEN!")
			}
		}
		break

	case Refresh:
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(m.GuildID, m.ChannelID, "", "")
		dgs.Delete(s) //delete the old message

		//create a new instance of the new one
		dgs.CreateMessage(s, bot.gameStateResponse(dgs), m.ChannelID, dgs.GameStateMsg.LeaderID)

		//add the emojis to the refreshed message if in the right stage
		if dgs.GetPhase() != game.MENU {
			for _, e := range bot.StatusEmojis[true] {
				dgs.AddReaction(s, e.FormatForReaction())
			}
			dgs.AddReaction(s, "❌")
		}
		bot.RedisInterface.SetDiscordGameState(dgs, lock)
		break

	case Settings:
		bot.HandleSettingsCommand(s, m, sett, args)
		//return // to prevent the User's message from being deleted
		break

	case Pause:
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(m.GuildID, m.ChannelID, "", "")
		dgs.Running = !dgs.Running
		bot.RedisInterface.SetDiscordGameState(dgs, lock)

		dgs.Edit(s, bot.gameStateResponse(dgs))
		break
	case Log:
		log.Println(fmt.Sprintf("\"%s\"", strings.Join(args, " ")))
		break
	case ShowMe:
		if m.Author != nil {
			sett := bot.StorageInterface.GetUserSettings(m.Author.ID)
			if sett == nil {
				s.ChannelMessageSend(m.ChannelID, "I don't have any data stored for you!")
			} else {
				embed := sett.ToEmbed()
				sendMessageDM(s, m.Author.ID, embed)
			}
		}
		break
	case ForgetMe:
		if m.Author != nil {
			err := bot.StorageInterface.DeleteUserSettings(m.Author.ID)
			if err != nil {
				log.Println(err)
			} else {
				s.ChannelMessageSend(m.ChannelID, "Successfully deleted all player data for <@"+m.Author.ID+">")
			}
		}
		break

	case DebugState:
		if m.Author != nil {
			state := bot.RedisInterface.GetReadOnlyDiscordGameState(m.GuildID, m.ChannelID, "", "")
			if state != nil {
				jBytes, err := json.MarshalIndent(state, "", "  ")
				if err != nil {
					log.Println(err)
				}
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```JSON\n%s\n```", jBytes))
			}
		}
		break
	case Ascii:
		s.ChannelMessageSend(m.ChannelID, AsciiCrewmate)
		break
	default:
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, I didn't understand that command! Please see `%s help` for commands", prefix))
		break
	}
}
