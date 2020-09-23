package discord

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func (guild *GuildState) handleGameEndMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	//clear the tracking and make sure all users are unlinked (means always unmute/undeafen)
	guild.clearGameTracking(s)

	// actually unmute/undeafen all based on the state assigned above
	guild.handleTrackedMembers(s)
	// clear any existing game state message
	guild.Room = ""
	guild.Region = ""
}

func (guild *GuildState) handlePlayerRemove(s *discordgo.Session, userID string) {
	log.Printf("Removing player %s", userID)
	guild.UserDataLock.RLock()
	if v, ok := guild.UserData[userID]; ok {
		guild.UserDataLock.RUnlock()
		v.auData = nil
		guild.updateUserInMap(userID, v)
	} else {
		guild.UserDataLock.RUnlock()
	}
}

func (guild *GuildState) handleGameStartMessage(s *discordgo.Session, m *discordgo.MessageCreate, room string, region string) {
	guild.Room = room
	guild.Region = region

	guild.clearGameTracking(s)

	guild.GameStateMessage = sendMessageEmbed(s, m.ChannelID, gameStateResponse(guild))
	log.Println("Added self game state message")

	for _, e := range guild.StatusEmojis[true] {
		addReaction(s, guild.GameStateMessage.ChannelID, guild.GameStateMessage.ID, e.FormatForReaction())
	}
	addReaction(s, guild.GameStateMessage.ChannelID, guild.GameStateMessage.ID, "❌")
}

// this will be called every game phase update
// i don't think we will have `m` where we need it, so potentially rethink it...?
func (guild *GuildState) handleGameStateMessage(s *discordgo.Session) {
	//guild.UserDataLock.Lock()
	//defer guild.UserDataLock.Unlock()

	if guild.GameStateMessage == nil {
		//log.Println("Game State Message is scuffed, try .au start again!")
		return
	}
	editMessageEmbed(s, guild.GameStateMessage.ChannelID, guild.GameStateMessage.ID, gameStateResponse(guild))
}

// sendMessage provides a single interface to send a message to a channel via discord
func sendMessage(s *discordgo.Session, channelID string, message string) *discordgo.Message {
	msg, err := s.ChannelMessageSend(channelID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
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
