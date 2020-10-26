package discord

import (
	"github.com/bwmarrin/discordgo"
	"sync"
	"time"
)

//TODO up this for a full public rollout
const EditDelaySeconds = 1

type GameStateMessage struct {
	MessageID        string `json:"messageID"`
	MessageChannelID string `json:"messageChannelID"`
	MessageAuthorID  string `json:"messageAuthorID"`
	LeaderID         string `json:"leaderID"`
	lock             sync.RWMutex

	deferredEdit *discordgo.MessageEmbed
}

func MakeGameStateMessage() GameStateMessage {
	return GameStateMessage{
		MessageID:        "",
		MessageChannelID: "",
		LeaderID:         "",
		lock:             sync.RWMutex{},
	}
}

func (gsm *GameStateMessage) Exists() bool {
	gsm.lock.RLock()
	defer gsm.lock.RUnlock()
	return gsm.MessageID != ""
}

func (gsm *GameStateMessage) AddReaction(s *discordgo.Session, emoji string) {
	gsm.lock.Lock()
	if gsm.MessageID != "" {
		addReaction(s, gsm.MessageChannelID, gsm.MessageID, emoji)
	}
	gsm.lock.Unlock()
}

func (gsm *GameStateMessage) RemoveAllReactions(s *discordgo.Session) {
	gsm.lock.Lock()
	if gsm.MessageID != "" {
		removeAllReactions(s, gsm.MessageChannelID, gsm.MessageID)
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
	if gsm.MessageID != "" {
		go deleteMessage(s, gsm.MessageChannelID, gsm.MessageID)
		gsm.MessageID = ""
		gsm.MessageChannelID = ""
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
	if gsm.MessageID != "" {
		editMessageEmbed(s, gsm.MessageChannelID, gsm.MessageID, gsm.deferredEdit)
	}
	gsm.deferredEdit = nil
	gsm.lock.Unlock()
}

func (gsm *GameStateMessage) CreateMessage(s *discordgo.Session, me *discordgo.MessageEmbed, channelID string, authorID string) {
	gsm.lock.Lock()
	gsm.LeaderID = authorID
	msg := sendMessageEmbed(s, channelID, me)
	if msg != nil {
		gsm.MessageAuthorID = msg.Author.ID
		gsm.MessageChannelID = msg.ChannelID
		gsm.MessageID = msg.ID
	}

	gsm.lock.Unlock()
}

func (gsm *GameStateMessage) SameChannel(channelID string) bool {
	gsm.lock.RLock()
	defer gsm.lock.RUnlock()
	if gsm.MessageID != "" {
		return gsm.MessageChannelID == channelID
	}
	return false
}

func (gsm *GameStateMessage) IsReactionTo(m *discordgo.MessageReactionAdd) bool {
	gsm.lock.RLock()
	defer gsm.lock.RUnlock()
	if !gsm.Exists() {
		return false
	}

	return m.ChannelID == gsm.MessageChannelID && m.MessageID == gsm.MessageID && m.UserID != gsm.MessageAuthorID
}
