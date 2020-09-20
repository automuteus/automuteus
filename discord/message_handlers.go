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

func (guild *GuildState) handleGameStartMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// another toy example of how rw locking could look
	if guild.GameStateMessage == nil {
		guild.GameStateMessage = sendMessage(s, m.ChannelID, gameStateResponse(guild))
		log.Println("Added self game state message")
	}

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
	editMessage(s, guild.GameStateMessage.ChannelID, guild.GameStateMessage.ID, gameStateResponse(guild))
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

func guildMemberMove(session *discordgo.Session, guildID, userID string, channelID *string) (err error) {
	log.Println("Issuing move channel request to discord")
	data := struct {
		ChannelID *string `json:"channel_id"`
	}{channelID}

	_, err = session.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), data, discordgo.EndpointGuildMember(guildID, ""))
	return
}

func guildMemberMute(session *discordgo.Session, guildID, userID string, mute bool) (err error) {
	log.Printf("Issuing mute=%v request to discord\n", mute)
	data := struct {
		Mute bool `json:"mute"`
	}{mute}

	_, err = session.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), data, discordgo.EndpointGuildMember(guildID, ""))
	return
}
