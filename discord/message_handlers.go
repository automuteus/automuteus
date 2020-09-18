package discord

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func handlePlayerListMessage(guild *GuildState, s *discordgo.Session, m *discordgo.MessageCreate) {
	// if we want to keep locking we can do something like this in the handlers
	guild.UserDataLock.RLock()
	message := playerListResponse(guild.UserData)
	guild.UserDataLock.RUnlock()
	sendMessage(s, m.ChannelID, message)
}

func handleGameStartMessage(guild *GuildState, s *discordgo.Session, m *discordgo.MessageCreate) {
	// another toy example of how rw locking could look
	guild.GameStateMessageLock.Lock()
	if guild.GameStateMessage == nil {
		guild.GameStateMessage = sendMessage(s, m.ChannelID, guild.ToString())
	}
	guild.GameStateMessageLock.Unlock()
}

// this will be called every game phase update
// i don't think we will have `m` where we need it, so potentially rethink it...?
func handleGameStateMessage(guild *GuildState, s *discordgo.Session) {
	if guild.GameStateMessage == nil {
		log.Println("Game State Message is scuffed, try .au start again!")
		return
	}
	editMessage(s, guild.GameStateMessage.ChannelID, guild.GameStateMessage.ID, guild.ToString())
}

// sendMessage provides a single interface to send a message to a channel via discord
func sendMessage(s *discordgo.Session, channelID string, message string) *discordgo.Message {
	msg, err := s.ChannelMessageSend(channelID, message)
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
