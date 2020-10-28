package discord

import (
	"fmt"
	"log"
	"regexp"

	"github.com/denverquane/amongusdiscord/game"

	"github.com/bwmarrin/discordgo"
)

const downloadURL = "https://github.com/denverquane/amonguscapture/releases/latest/download/amonguscapture.exe"

func (bot *Bot) endGame(guildID, channelID, voiceChannel, connCode string, s *discordgo.Session) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(guildID, channelID, voiceChannel, connCode)

	dgs.SetAllAlive()
	dgs.UpdatePhase(game.LOBBY)
	dgs.SetRoomRegion("", "")

	sett := bot.StorageInterface.GetGuildSettings(dgs.GuildID)

	// apply the unmute/deafen to users who have state linked to them
	dgs.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, game.LOBBY)

	//clear the Tracking and make sure all users are unlinked
	dgs.clearGameTracking(s)

	dgs.Running = false
	bot.RedisInterface.SetDiscordGameState(dgs, lock)
}

var urlregex = regexp.MustCompile(`^http(?P<secure>s?)://(?P<host>[\w.-]+)(?::(?P<port>\d+))/?$`)

func (bot *Bot) handleNewGameMessage(s *discordgo.Session, m *discordgo.MessageCreate, g *discordgo.Guild, room, region string) {
	initialTracking := make([]TrackingChannel, 0)

	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(m.GuildID, m.ChannelID, "", "")
	//TODO need to send a message to the capture re-questing all the player/game states. Otherwise,
	//we don't have enough info to go off of when remaking the game...
	//if !guild.GameStateMsg.Exists() {
	if dgs.GameStateMsg.MessageChannelID != "" {
		if v, ok := bot.RedisSubscriberKillChannels[dgs.ConnectCode]; ok {
			v <- true
		}
		bot.RedisInterface.DeleteDiscordGameState(dgs)
		dgs.Reset()
	}

	connectCode := generateConnectCode(m.GuildID)

	dgs.ConnectCode = connectCode

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	killChan := make(chan bool)

	bot.ChannelsMapLock.Lock()
	bot.RedisSubscriberKillChannels[connectCode] = killChan
	bot.ChannelsMapLock.Unlock()

	go bot.SubscribeToGameByConnectCode(m.GuildID, connectCode, killChan)

	var hyperlink string
	var minimalUrl string

	if match := urlregex.FindStringSubmatch(bot.url); match != nil {
		secure := match[urlregex.SubexpIndex("secure")] == "s"
		host := match[urlregex.SubexpIndex("host")]
		port := ":" + match[urlregex.SubexpIndex("port")]

		if port == ":" {
			port = ":" + bot.internalPort
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
		hyperlink = "Invalid HOST provided (should resemble something like `http://localhost:8123`)"
		minimalUrl = "Invalid HOST provided"
	}

	var embed = discordgo.MessageEmbed{
		URL:   "",
		Type:  "",
		Title: "You just started a game!",
		Description: fmt.Sprintf("Click the following link to link your capture: \n <%s>\n\n"+
			"Don't have the capture installed? Latest version [here](%s)\n\nTo link your capture manually:", hyperlink, downloadURL),
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

	log.Println("Generated URL for connection: " + hyperlink)

	sendMessageDM(s, m.Author.ID, &embed)

	channels, err := s.GuildChannels(m.GuildID)
	if err != nil {
		log.Println(err)
	}

	//defaultTracked := guild.guildSettings.GetDefaultTrackedChannel()
	for _, channel := range channels {
		//if channel.Type == discordgo.ChannelTypeGuildVoice {
		//	if channel.ID == defaultTracked || strings.ToLower(channel.Name) == strings.ToLower(defaultTracked) {
		//		initialTracking = append(initialTracking, TrackingChannel{
		//			ChannelID:   channel.ID,
		//			ChannelName: channel.Name,
		//			forGhosts:   false,
		//		})
		//		guild.Log(fmt.Sprintf("Found initial default channel specified in config: ID %s, Name %s\n", channel.ID, channel.Name))
		//	}
		//}
		for _, v := range g.VoiceStates {
			//if the User is detected in a voice channel
			if v.UserID == m.Author.ID {

				//once we find the channel by ID
				if channel.Type == discordgo.ChannelTypeGuildVoice {
					if channel.ID == v.ChannelID {
						initialTracking = append(initialTracking, TrackingChannel{
							ChannelID:   channel.ID,
							ChannelName: channel.Name,
						})
						log.Print(fmt.Sprintf("User that typed new is in the \"%s\" voice channel; using that for Tracking", channel.Name))
					}
				}
			}
		}
	}

	bot.handleGameStartMessage(s, m, room, region, initialTracking, g, connectCode)
}

func (bot *Bot) handleGameStartMessage(s *discordgo.Session, m *discordgo.MessageCreate, room string, region string, channels []TrackingChannel, g *discordgo.Guild, connCode string) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(m.GuildID, m.ChannelID, "", connCode)
	dgs.SetRoomRegion(room, region)

	dgs.clearGameTracking(s)

	dgs.Running = true

	for _, channel := range channels {
		if channel.ChannelName != "" {
			dgs.Tracking = TrackingChannel{
				ChannelID:   channel.ChannelID,
				ChannelName: channel.ChannelName,
			}
			for _, v := range g.VoiceStates {
				if v.ChannelID == channel.ChannelID {
					dgs.checkCacheAndAddUser(g, s, v.UserID)
				}
			}
		}
	}

	dgs.CreateMessage(s, bot.gameStateResponse(dgs), m.ChannelID, m.Author.ID)

	log.Println("Added self game state message")

	if dgs.GetPhase() != game.MENU {
		for _, e := range bot.StatusEmojis[true] {
			dgs.AddReaction(s, e.FormatForReaction())
		}
		dgs.AddReaction(s, "❌")
	}
	bot.RedisInterface.SetDiscordGameState(dgs, lock)
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
