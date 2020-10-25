package discord

import (
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
	},
	{
		cmdType:   Track,
		command:   "track",
		example:   "track Among Us Voice",
		shortDesc: "Track a voice channel",
		desc:      "Tell the bot which voice channel you'll be playing in",
		args:      "<voice channel name>",
		aliases:   []string{"t"},
	},
	{
		cmdType:   Link,
		command:   "link",
		example:   "link @Soup red",
		shortDesc: "Link a Discord user",
		desc:      "Manually link a Discord user to their in-game color or name",
		args:      "<discord user> <in-game color or name>",
		aliases:   []string{"l"},
	},
	{
		cmdType:   Unlink,
		command:   "unlink",
		example:   "unlink @Soup",
		shortDesc: "Unlink a Discord user",
		desc:      "Manually unlink a Discord user from their in-game player",
		args:      "<discord user>",
		aliases:   []string{"u"},
	},
	{
		cmdType:   New,
		command:   "new",
		example:   "new",
		shortDesc: "Start a new game",
		desc:      "Start a new game",
		args:      "None",
		aliases:   []string{"n"},
	},
	{
		cmdType:   End,
		command:   "end",
		example:   "end",
		shortDesc: "End the game",
		desc:      "End the current game",
		args:      "None",
		aliases:   []string{"e"},
	},
	{
		cmdType:   Force,
		command:   "force",
		example:   "force task",
		shortDesc: "Force the bot to transition",
		desc:      "Force the bot to transition to another game stage, if it doesn't transition properly",
		args:      "<phase name> (task, discuss, or lobby / t,d, or l)",
		aliases:   []string{"f"},
	},
	{
		cmdType:   Refresh,
		command:   "refresh",
		example:   "refresh",
		shortDesc: "Refresh the bot status",
		desc:      "Recreate the bot status message if it ends up too far in the chat",
		args:      "None",
		aliases:   []string{"r"},
	},
	{
		cmdType:   Settings,
		command:   "settings",
		example:   "settings commandPrefix !",
		shortDesc: "Adjust bot settings",
		desc:      "Adjust the bot settings. Type `.au settings` with no arguments to see more.",
		args:      "<setting> <value>",
		aliases:   []string{"s"},
	},
	{
		cmdType:   Pause,
		command:   "pause",
		example:   "pause",
		shortDesc: "Pause the bot",
		desc:      "Pause the bot so it doesn't automute/deafen. **Will not unmute/undeafen**",
		args:      "None",
		aliases:   []string{"p"},
	},
	{
		cmdType:   Log,
		command:   "log",
		example:   "log something bad happened",
		shortDesc: "Log a weird event",
		desc:      "Log if something bad happened, so you can find the event in your logs later",
		args:      "<message>",
		aliases:   []string{"log"},
	},
	{
		cmdType:   Null,
		command:   "",
		example:   "",
		shortDesc: "",
		desc:      "",
		args:      "",
		aliases:   []string{""},
	},
}

//TODO cache/preconstruct these (no reason to make them fresh everytime help is called, except for the prefix...)
func ConstructEmbedForCommand(prefix string, cmd Command) discordgo.MessageEmbed {
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
			if cmd.command[0] == arg[0] {
				return cmd
			}
		} else {
			if arg == cmd.command {
				return cmd
			}
		}
	}
	return AllCommands[Null]
}

func (bot *Bot) HandleCommand(sett *storage.GuildSettings, s *discordgo.Session, g *discordgo.Guild, m *discordgo.MessageCreate, args []string) {
	prefix := sett.CommandPrefix
	cmd := GetCommand(args[0])

	if cmd.cmdType != Null {
		log.Print(fmt.Sprintf("\"%s\" command typed by user %s\n", cmd.command, m.Author.ID))
	}
	dgs := bot.RedisInterface.GetDiscordGameState(m.GuildID, m.ChannelID, "", "")
	aud := bot.RedisInterface.GetAmongUsData(dgs.ConnectCode)

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

			dgs.trackChannel(channelName, channels)

			dgs.GameStateMsg.Edit(s, bot.gameStateResponse(aud, dgs))
		}
		break

	case Link:
		if len(args[1:]) < 2 {
			embed := ConstructEmbedForCommand(prefix, cmd)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		} else {
			dgs.linkPlayer(s, aud, args[1:])
			dgs.GameStateMsg.Edit(s, bot.gameStateResponse(aud, dgs))
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
				dgs.UserData.ClearPlayerData(userID)

				//make sure that any players we remove/unlink get auto-unmuted/undeafened
				dgs.verifyVoiceStateChanges(s, sett, aud.GetPhase())

				//update the state message to reflect the player leaving
				dgs.GameStateMsg.Edit(s, bot.gameStateResponse(aud, dgs))
			}
		}
		break

	case New:
		room, region := getRoomAndRegionFromArgs(args[1:])

		bot.handleNewGameMessage(dgs, aud, s, m, g, room, region)
		break

	case End:
		log.Println("User typed end to end the current game")

		bot.endGame(dgs, aud, s)

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
		dgs.GameStateMsg.Delete(s) //delete the old message

		//create a new instance of the new one
		dgs.GameStateMsg.CreateMessage(s, bot.gameStateResponse(aud, dgs), m.ChannelID, dgs.GameStateMsg.LeaderID)

		//add the emojis to the refreshed message if in the right stage
		if aud.GetPhase() != game.MENU {
			for _, e := range bot.StatusEmojis[true] {
				dgs.GameStateMsg.AddReaction(s, e.FormatForReaction())
			}
			dgs.GameStateMsg.AddReaction(s, "❌")
		}
		break

	case Settings:
		bot.HandleSettingsCommand(s, m, sett, args)
		//return // to prevent the user's message from being deleted
		break

	case Pause:
		dgs.Running = !dgs.Running
		dgs.GameStateMsg.Edit(s, bot.gameStateResponse(aud, dgs))
		break
	case Log:
		log.Println(fmt.Sprintf("\"%s\"", strings.Join(args, " ")))
		break
	default:
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, I didn't understand that command! Please see `%s help` for commands", prefix))
		break
	}

	bot.RedisInterface.SetDiscordGameState(m.GuildID, dgs)
}
