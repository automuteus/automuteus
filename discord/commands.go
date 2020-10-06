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

func (guild *GuildState) HandleCommand(s *discordgo.Session, g *discordgo.Guild, storageInterface storage.StorageInterface, m *discordgo.MessageCreate, args []string) {
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

		}
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
		break

	case New:
		room, region := getRoomAndRegionFromArgs(args[1:])

		initialTracking := make([]TrackingChannel, 0)

		//TODO need to send a message to the capture re-questing all the player/game states. Otherwise,
		//we don't have enough info to go off of when remaking the game...
		//if !guild.GameStateMsg.Exists() {

		connectCode := generateConnectCode(guild.PersistentGuildData.GuildID)
		log.Println(connectCode)
		LinkCodeLock.Lock()
		LinkCodes[GameOrLobbyCode{
			gameCode:    "",
			connectCode: connectCode,
		}] = guild.PersistentGuildData.GuildID
		guild.LinkCode = connectCode
		LinkCodeLock.Unlock()

		hyperlink := fmt.Sprintf("aucapture://%s:%s/%s?insecure", BotUrl, BotPort, connectCode)

		var embed = discordgo.MessageEmbed{
			URL:         "",
			Type:        "",
			Title:       "You just started a game!",
			Description: fmt.Sprintf("Click the following link to link your capture: \n <%s>", hyperlink),
			Timestamp:   "",
			Color:       3066993, //GREEN
			Image:       nil,
			Thumbnail:   nil,
			Video:       nil,
			Provider:    nil,
			Author:      nil,
		}

		sendMessageDM(s, m.Author.ID, &embed)

		channels, err := s.GuildChannels(m.GuildID)
		if err != nil {
			log.Println(err)
		}

		for _, channel := range channels {
			if channel.Type == discordgo.ChannelTypeGuildVoice {
				if channel.ID == guild.PersistentGuildData.DefaultTrackedChannel || strings.ToLower(channel.Name) == strings.ToLower(guild.PersistentGuildData.DefaultTrackedChannel) {
					initialTracking = append(initialTracking, TrackingChannel{
						channelID:   channel.ID,
						channelName: channel.Name,
						forGhosts:   false,
					})
					log.Printf("Found initial default channel specified in config: ID %s, Name %s\n", channel.ID, channel.Name)
				}
			}
			for _, v := range g.VoiceStates {
				//if the user is detected in a voice channel
				if v.UserID == m.Author.ID {

					//once we find the channel by ID
					if channel.Type == discordgo.ChannelTypeGuildVoice {
						if channel.ID == v.ChannelID {
							initialTracking = append(initialTracking, TrackingChannel{
								channelID:   channel.ID,
								channelName: channel.Name,
								forGhosts:   false,
							})
							log.Printf("User that typed new is in the \"%s\" voice channel; using that for tracking", channel.Name)
						}
					}

				}

			}
		}

		guild.handleGameStartMessage(s, m, room, region, initialTracking, g)
		break

	case End:
		guild.handleGameEndMessage(s)

		//have to explicitly delete here, because if we use the default delete below, the channelID
		//for the game state message doesn't exist anymore...
		deleteMessage(s, m.ChannelID, m.Message.ID)
		break

	case Force:
		phase := getPhaseFromString(args[1])
		if phase == game.UNINITIALIZED {
			s.ChannelMessageSend(m.ChannelID, "Sorry, I didn't understand the game phase you tried to force")
		} else {
			//TODO this is ugly, but only for debug really
			ChannelsMapLock.RLock()
			*GamePhaseUpdateChannels[m.GuildID] <- phase
			ChannelsMapLock.RUnlock()
		}
		break

	case Refresh:
		guild.GameStateMsg.Delete(s) //delete the old message

		//create a new instance of the new one
		guild.GameStateMsg.CreateMessage(s, gameStateResponse(guild), m.ChannelID)

		//add the emojis to the refreshed message
		for _, e := range guild.StatusEmojis[true] {
			guild.GameStateMsg.AddReaction(s, e.FormatForReaction())
		}
		guild.GameStateMsg.AddReaction(s, "❌")
		break

	case Settings:
		HandleSettingsCommand(s, m, guild, storageInterface, args)
		return // to prevent the user's message from being deleted
	default:
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Sorry, I didn't understand that command! Please see `%s help` for commands", guild.PersistentGuildData.CommandPrefix))

	}
}
