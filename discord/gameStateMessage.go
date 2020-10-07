package discord

import (
	"github.com/bwmarrin/discordgo"
	"sync"
)

type GameStateMessage struct {
	message *discordgo.Message
	lock    sync.RWMutex
}

type PrivateStateMessage struct {
	message *discordgo.Message
	lock sync.RWMutex
	idUsernameMap map[string]string
	printedUsers []string
	privateChannelID string
	currentUserID string
}

func MakeGameStateMessage() GameStateMessage {
	return GameStateMessage{
		message: nil,
		lock:    sync.RWMutex{},
	}
}

func MakePrivateStateMessage() PrivateStateMessage {
	return PrivateStateMessage{
		message: nil,
		lock:    sync.RWMutex{},
		idUsernameMap: make(map[string]string),
		printedUsers: make([]string, 0),
		privateChannelID: "000000000000000000", // Default Value
		currentUserID: "",
	}
}

func (psm *PrivateStateMessage) Initialize(){
	psm.lock.Lock();
	defer psm.lock.Unlock()

	psm.idUsernameMap = make(map[string]string)
	psm.printedUsers = make([]string, 0)
	psm.currentUserID = ""
	psm.message = nil;
}

func (psm *PrivateStateMessage) getIDUsernameMap() map[string]string{
	psm.lock.RLock()
	defer psm.lock.RUnlock()

	return psm.idUsernameMap
}

func (gsm *GameStateMessage) Exists() bool {
	gsm.lock.RLock()
	defer gsm.lock.RUnlock()
	return gsm.message != nil
}

func (psm *PrivateStateMessage) Exists() bool {
	psm.lock.RLock()
	defer psm.lock.RUnlock()
	return psm.message != nil
}

func (gsm *GameStateMessage) AddReaction(s *discordgo.Session, emoji string) {
	gsm.lock.Lock()
	if gsm.message != nil {
		addReaction(s, gsm.message.ChannelID, gsm.message.ID, emoji)
	}
	gsm.lock.Unlock()
}

func (psm *PrivateStateMessage) AddReaction(s *discordgo.Session, emoji string) {
	psm.lock.Lock()
	if psm.message != nil {
		addReaction(s, psm.message.ChannelID, psm.message.ID, emoji)
	}
	psm.lock.Unlock()
}

func (gsm *GameStateMessage) Delete(s *discordgo.Session) {
	gsm.lock.Lock()
	if gsm.message != nil {
		go deleteMessage(s, gsm.message.ChannelID, gsm.message.ID)
		gsm.message = nil
	}
	gsm.lock.Unlock()
}

func (gsm *GameStateMessage) Edit(s *discordgo.Session, me *discordgo.MessageEmbed) {
	gsm.lock.Lock()
	if gsm.message != nil {
		editMessageEmbed(s, gsm.message.ChannelID, gsm.message.ID, me)
	}
	gsm.lock.Unlock()
}

func (gsm *GameStateMessage) CreateMessage(s *discordgo.Session, me *discordgo.MessageEmbed, channelID string) {
	gsm.lock.Lock()
	gsm.message = sendMessageEmbed(s, channelID, me)
	gsm.lock.Unlock()
}

func (psm *PrivateStateMessage) CreateMessage(s *discordgo.Session, me *discordgo.MessageEmbed, channelID string) *discordgo.Message {
	psm.lock.Lock()
	psm.message = sendMessageEmbed(s, channelID, me)
	psm.lock.Unlock()
	return psm.message;
}

func (gsm *GameStateMessage) SameChannel(channelID string) bool {
	gsm.lock.RLock()
	defer gsm.lock.RUnlock()
	if gsm.message != nil {
		return gsm.message.ChannelID == channelID
	}
	return false
}

func (gsm *GameStateMessage) IsReactionTo(m *discordgo.MessageReactionAdd) bool {
	gsm.lock.RLock()
	defer gsm.lock.RUnlock()
	if gsm.message == nil {
		return false
	}

	return m.ChannelID == gsm.message.ChannelID && m.MessageID == gsm.message.ID && m.UserID != gsm.message.Author.ID
}

func (psm *PrivateStateMessage) IsReactionTo(m *discordgo.MessageReactionAdd) bool {
	psm.lock.RLock()
	defer psm.lock.RUnlock()
	if psm.message == nil {
		return false
	}

	return m.ChannelID == psm.message.ChannelID && m.MessageID == psm.message.ID && m.UserID != psm.message.Author.ID}

func (psm *PrivateStateMessage) privateMapResponse(uID string, uName string) *discordgo.MessageEmbed {


	gameInfoFields := make([]*discordgo.MessageEmbedField, 2)
	gameInfoFields[0] = &discordgo.MessageEmbedField{
		Name:   "User ID",
		Value:  uID,
		Inline: false,
	}
	gameInfoFields[1] = &discordgo.MessageEmbedField{
		Name: "Username",
		Value: uName,
		Inline: false,
	}





	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       "What color is " + uName + "?",
		Description: "",
		Timestamp:   "",
		Footer: &discordgo.MessageEmbedFooter{
			Text:         "React to this message with " + uName + "'s color! (or ❌ to skip)",
			IconURL:      "",
			ProxyIconURL: "",
		},
		Color:     3066993, //GREEN
		Image:     nil,
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
		Fields:    gameInfoFields,
	}
	return &msg
}
