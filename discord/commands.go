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
	Null
)

var CommandTypeStringMapping = map[string]CommandType{
	"help":     Help,
	"track":    Track,
	"link":     Link,
	"unlink":   Unlink,
	"new":      New,
	"end":      End,
	"force":    Force,
	"refresh":  Refresh,
	"settings": Settings,
	"pause":    Pause,
	"":         Null,
}

func GetCommandType(arg string) CommandType {
	for str, cmd := range CommandTypeStringMapping {
		if len(arg) == 1 && cmd != Null {
			if str[0] == arg[0] {
				return cmd
			}
		} else {
			if strings.ToLower(arg) == str {
				return cmd
			}
		}
	}

	return Null
}

func (bot *Bot) HandleCommand(guild *GuildState, s *discordgo.Session, g *discordgo.Guild, storageInterface storage.StorageInterface, m *discordgo.MessageCreate, args []string) {
	switch GetCommandType(args[0]) {

	case Help:
		s.ChannelMessageSend(m.ChannelID, helpResponse(Version, guild.PersistentGuildData.CommandPrefix))
		break

	case Track:
		if len(args[1:]) == 0 {
			//TODO print usage of this command specifically
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You used this command incorrectly! Please refer to `%s help` for proper command usage", guild.PersistentGuildData.CommandPrefix))
		} else {
			// have to explicitly check for true. Otherwise, processing the 2-word VC names gets really ugly...
			forGhosts := false
			endIdx := len(args)
			if args[len(args)-1] == "true" || args[len(args)-1] == "t" {
				forGhosts = true
				endIdx--
			}

			channelName := strings.Join(args[1:endIdx], " ")

			channels, err := s.GuildChannels(m.GuildID)
			if err != nil {
				log.Println(err)
			}

			guild.trackChannelResponse(channelName, channels, forGhosts)

			guild.GameStateMsg.Edit(s, gameStateResponse(guild))
		}
		break

	case Link:
		if len(args[1:]) < 2 {
			//TODO print usage of this command specifically
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You used this command incorrectly! Please refer to `%s help` for proper command usage", guild.PersistentGuildData.CommandPrefix))
		} else {
			guild.linkPlayerResponse(s, m.GuildID, args[1:])

			guild.GameStateMsg.Edit(s, gameStateResponse(guild))
		}
		break

	case Unlink:
		if len(args[1:]) == 0 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You used this command incorrectly! Please refer to `%s help` for proper command usage", guild.PersistentGuildData.CommandPrefix))
		} else {

			userID, err := extractUserIDFromMention(args[1])
			if err != nil {
				log.Println(err)
			} else {
				log.Printf("Removing player %s", userID)
				guild.UserData.ClearPlayerData(userID)

				//make sure that any players we remove/unlink get auto-unmuted/undeafened
				guild.verifyVoiceStateChanges(s)

				//update the state message to reflect the player leaving
				guild.GameStateMsg.Edit(s, gameStateResponse(guild))
			}
		}
		break

	case New:
		room, region := getRoomAndRegionFromArgs(args[1:])

		bot.handleNewGameMessage(guild, s, m, g, room, region)
		break

	case End:
		log.Println("User typed end to end the current game")

		bot.handleGameEndMessage(guild, s)

		//have to explicitly delete here, because if we use the default delete below, the channelID
		//for the game state message doesn't exist anymore...
		deleteMessage(s, m.ChannelID, m.Message.ID)
		break

	case Force:
		if len(args[1:]) < 1 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You used this command incorrectly! Please refer to `%s help` for proper command usage", guild.PersistentGuildData.CommandPrefix))
		}
		phase := getPhaseFromString(args[1])
		if phase == game.UNINITIALIZED {
			s.ChannelMessageSend(m.ChannelID, "Sorry, I didn't understand the game phase you tried to force")
		} else {
			//TODO this is ugly, but only for debug really
			bot.PushGuildPhaseUpdate(m.GuildID, phase)
		}
		break

	case Refresh:
		guild.GameStateMsg.Delete(s) //delete the old message

		//create a new instance of the new one
		guild.GameStateMsg.CreateMessage(s, gameStateResponse(guild), m.ChannelID, guild.GameStateMsg.leaderID)

		//add the emojis to the refreshed message if in the right stage
		if guild.AmongUsData.GetPhase() != game.MENU {
			for _, e := range guild.StatusEmojis[true] {
				guild.GameStateMsg.AddReaction(s, e.FormatForReaction())
			}
			guild.GameStateMsg.AddReaction(s, "❌")
		}
		break

	case Settings:
		HandleSettingsCommand(s, m, guild, storageInterface, args)
		return // to prevent the user's message from being deleted

	case Pause:
		guild.GameRunning = !guild.GameRunning
		guild.GameStateMsg.Edit(s, gameStateResponse(guild))
		break
	default:
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, I didn't understand that command! Please see `%s help` for commands", guild.PersistentGuildData.CommandPrefix))

	}
}
