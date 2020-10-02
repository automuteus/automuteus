package discord

import (
	"github.com/denverquane/amongusdiscord/game"
	"log"

	"github.com/bwmarrin/discordgo"
)

func (guild *GuildState) handleGameEndMessage(s *discordgo.Session) {
	guild.AmongUsData.SetAllAlive()
	guild.AmongUsData.SetPhase(game.LOBBY)

	// apply the unmute/deafen to users who have state linked to them
	guild.handleTrackedMembers(s, 0, NoPriority)

	//clear the tracking and make sure all users are unlinked
	guild.clearGameTracking(s)

	// clear any existing game state message
	guild.AmongUsData.SetRoomRegion("", "")
}

func (guild *GuildState) handleGameStartMessage(s *discordgo.Session, m *discordgo.MessageCreate, room string, region string, channels []TrackingChannel) {
	guild.AmongUsData.SetRoomRegion(room, region)

	guild.clearGameTracking(s)

	for _, channel := range channels {
		if channel.channelName != "" {
			guild.Tracking.AddTrackedChannel(channel.channelID, channel.channelName, channel.forGhosts)
		}
	}

	guild.GameStateMsg.CreateMessage(s, gameStateResponse(guild), m.ChannelID)

	log.Println("Added self game state message")

	for _, e := range guild.StatusEmojis[true] {
		guild.GameStateMsg.AddReaction(s, e.FormatForReaction())
	}
	guild.GameStateMsg.AddReaction(s, "❌")
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
