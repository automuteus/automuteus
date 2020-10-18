package discord

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/denverquane/amongusdiscord/game"

	"github.com/bwmarrin/discordgo"
)

const downloadURL = "https://github.com/denverquane/amonguscapture/releases/latest/download/amonguscapture.exe"
const dotNet32Url = "https://dotnet.microsoft.com/download/dotnet-core/thank-you/sdk-3.1.402-windows-x86-installer"
const dotNet64Url = "https://dotnet.microsoft.com/download/dotnet-core/thank-you/sdk-3.1.402-windows-x64-installer"

func (bot *Bot) handleGameEndMessage(guild *GuildState, s *discordgo.Session) {
	guild.AmongUsData.SetAllAlive()
	guild.AmongUsData.SetPhase(game.LOBBY)

	// apply the unmute/deafen to users who have state linked to them
	guild.handleTrackedMembers(&bot.SessionManager, 0, NoPriority)

	//clear the tracking and make sure all users are unlinked
	guild.clearGameTracking(s)

	guild.GameRunning = false

	// clear any existing game state message
	guild.AmongUsData.SetRoomRegion("", "")
}

func (bot *Bot) handleNewGameMessage(guild *GuildState, s *discordgo.Session, m *discordgo.MessageCreate, g *discordgo.Guild, room, region string) {
	initialTracking := make([]TrackingChannel, 0)

	//TODO need to send a message to the capture re-questing all the player/game states. Otherwise,
	//we don't have enough info to go off of when remaking the game...
	//if !guild.GameStateMsg.Exists() {

	//TODO don't always recreate if we're already connected...

	connectCode := generateConnectCode(guild.PersistentGuildData.GuildID)
	log.Println(connectCode)
	bot.LinkCodeLock.Lock()
	bot.LinkCodes[GameOrLobbyCode{
		gameCode:    room,
		connectCode: connectCode,
	}] = guild.PersistentGuildData.GuildID

	bot.LinkCodeLock.Unlock()

	var hyperlink string
	var minimalUrl string
	urlregex := regexp.MustCompile(`^http(?P<secure>s?)://(?P<host>[\w.-]+)(?::(?P<port>\d+))?/?$`)
	if match := urlregex.FindStringSubmatch(bot.url); match != nil {
		secure := match[urlregex.SubexpIndex("secure")] == "s"
		host := match[urlregex.SubexpIndex("host")]
		port := ":" + match[urlregex.SubexpIndex("port")]

		if port == ":" {
			if bot.extPort != "" {
				if bot.extPort == "protocol" {
					port = ""
				} else {
					port = ":" + bot.extPort
				}
			} else {
				//if no port explicitly provided via config, use the default
				port = ":" + bot.socketPort
			}
		}

		insecure := "?insecure"
		protocol := "http://"
		if secure {
			insecure = ""
			protocol = "https://"
		}

		hyperlink = fmt.Sprintf("aucapture://%s%s/%s%s", host, port, connectCode, insecure)
		minimalUrl = fmt.Sprintf("%s%s%s", protocol, host, port)
	} else {
		hyperlink = "Invalid Server URL (missing `http://`? Or do you have a trailing `/`?)"
		minimalUrl = "Invalid Server URL"
	}

	var embed = discordgo.MessageEmbed{
		URL:   "",
		Type:  "",
		Title: "You just started a game!",
		Description: fmt.Sprintf("Click the following link to link your capture: \n <%s>\n\n"+
			"Don't have the capture installed? Latest version [here](%s)\nDon't have .NET Core installed? [32-bit here](%s), [64-bit here](%s)\n\nTo link your capture manually:", hyperlink, downloadURL, dotNet32Url, dotNet64Url),
		Timestamp: "",
		Color:     3066993, //GREEN
		Image:     nil,
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{
				Name:   "URL",
				Value:  minimalUrl,
				Inline: true,
			},
			&discordgo.MessageEmbedField{
				Name:   "Code",
				Value:  connectCode,
				Inline: true,
			},
		},
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
}

func (guild *GuildState) handleGameStartMessage(s *discordgo.Session, m *discordgo.MessageCreate, room string, region string, channels []TrackingChannel, g *discordgo.Guild) {
	guild.AmongUsData.SetRoomRegion(room, region)

	guild.clearGameTracking(s)

	guild.GameRunning = true

	for _, channel := range channels {
		if channel.channelName != "" {
			guild.Tracking.AddTrackedChannel(channel.channelID, channel.channelName, channel.forGhosts)
			for _, v := range g.VoiceStates {
				if v.ChannelID == channel.channelID {
					guild.checkCacheAndAddUser(g, s, v.UserID)
				}
			}
		}
	}

	guild.GameStateMsg.CreateMessage(s, gameStateResponse(guild), m.ChannelID, m.Author.ID)

	log.Println("Added self game state message")

	if guild.AmongUsData.GetPhase() != game.MENU {
		for _, e := range guild.StatusEmojis[true] {
			guild.GameStateMsg.AddReaction(s, e.FormatForReaction())
		}
		guild.GameStateMsg.AddReaction(s, "❌")
	}
}

// sendMessage provides a single interface to send a message to a channel via discord
func sendMessage(s *discordgo.Session, channelID string, message string) *discordgo.Message {
	msg, err := s.ChannelMessageSend(channelID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

func sendMessageDM(s *discordgo.Session, userID string, message *discordgo.MessageEmbed) *discordgo.Message {
	dmChannel, err := s.UserChannelCreate(userID)
	if err != nil {
		log.Println(err)
	}
	m, err := s.ChannelMessageSendEmbed(dmChannel.ID, message)
	if err != nil {
		log.Println(err)
	}
	return m
}

func sendMessageEmbed(s *discordgo.Session, channelID string, message *discordgo.MessageEmbed) *discordgo.Message {
	msg, err := s.ChannelMessageSendEmbed(channelID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

// editMessage provides a single interface to edit a message in a channel via discord
func editMessage(s *discordgo.Session, channelID string, messageID string, message string) *discordgo.Message {
	msg, err := s.ChannelMessageEdit(channelID, messageID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

func editMessageEmbed(s *discordgo.Session, channelID string, messageID string, message *discordgo.MessageEmbed) *discordgo.Message {
	msg, err := s.ChannelMessageEditEmbed(channelID, messageID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

func deleteMessage(s *discordgo.Session, channelID string, messageID string) {
	err := s.ChannelMessageDelete(channelID, messageID)
	if err != nil {
		log.Println(err)
	}
}

func addReaction(s *discordgo.Session, channelID, messageID, emojiID string) {
	err := s.MessageReactionAdd(channelID, messageID, emojiID)
	if err != nil {
		log.Println(err)
	}
}

func removeAllReactions(s *discordgo.Session, channelID, messageID string) {
	err := s.MessageReactionsRemoveAll(channelID, messageID)
	if err != nil {
		log.Println(err)
	}
}
