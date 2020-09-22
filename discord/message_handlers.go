package discord

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

//func handlePlayerListMessage(guild *GuildState, s *discordgo.Session, m *discordgo.MessageCreate) {
//	// if we want to keep locking we can do something like this in the handlers
//	guild.UserDataLock.RLock()
//	handleGameStateMessage(guild, s)
//	guild.UserDataLock.RUnlock()
//	//sendMessage(s, m.ChannelID, message)
//}


func (guild *GuildState) handleGameEndMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// unmute all players
	guild.handleTrackedMembers(s, false, false)
	// clear any existing game state message
	guild.Room = ""
	guild.Region = ""
	guild.clearGameTracking(s)
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
