package discord

import (
	"github.com/bwmarrin/discordgo"
	"sync"
	"time"
)

//TODO up this for a full public rollout
const EditDelaySeconds = 1

type GameStateMessage struct {
	message  *discordgo.Message
	leaderID string //who started the game
	lock     sync.RWMutex

	deferredEdit *discordgo.MessageEmbed
}

func MakeGameStateMessage() GameStateMessage {
	return GameStateMessage{
		message:  nil,
		leaderID: "",
		lock:     sync.RWMutex{},
	}
}

func (gsm *GameStateMessage) Exists() bool {
	gsm.lock.RLock()
	defer gsm.lock.RUnlock()
	return gsm.message != nil
}

func (gsm *GameStateMessage) AddReaction(s *discordgo.Session, emoji string) {
	gsm.lock.Lock()
	if gsm.message != nil {
		addReaction(s, gsm.message.ChannelID, gsm.message.ID, emoji)
	}
	gsm.lock.Unlock()
}

func (gsm *GameStateMessage) RemoveAllReactions(s *discordgo.Session) {
	gsm.lock.Lock()
	if gsm.message != nil {
		removeAllReactions(s, gsm.message.ChannelID, gsm.message.ID)
	}
	gsm.lock.Unlock()
}

func (gsm *GameStateMessage) AddAllReactions(s *discordgo.Session, emojis []Emoji) {
	for _, e := range emojis {
		gsm.AddReaction(s, e.FormatForReaction())
	}
	gsm.AddReaction(s, "❌")
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
	//the worker is already waiting to update the message, so just swap the message in-place
	if gsm.deferredEdit != nil {
		gsm.deferredEdit = me //swap with the newer message
	} else {
		gsm.deferredEdit = me
		//the edit is empty, so there isn't a worker waiting to update it
		go gsm.EditWorker(s, EditDelaySeconds)
	}
	gsm.lock.Unlock()
}

func (gsm *GameStateMessage) EditWorker(s *discordgo.Session, delay int) {
	time.Sleep(time.Duration(delay) * time.Second)

	gsm.lock.Lock()
	if gsm.message != nil {
		editMessageEmbed(s, gsm.message.ChannelID, gsm.message.ID, gsm.deferredEdit)
	}
	gsm.deferredEdit = nil
	gsm.lock.Unlock()
}

func (gsm *GameStateMessage) CreateMessage(s *discordgo.Session, me *discordgo.MessageEmbed, channelID string, authorID string) {
	gsm.lock.Lock()
	gsm.leaderID = authorID
	gsm.message = sendMessageEmbed(s, channelID, me)
	gsm.lock.Unlock()
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
