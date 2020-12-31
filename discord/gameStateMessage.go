package discord

import (
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// bumped for public rollout. Don't need to update the status message more than once every 2 secs prob
const DeferredEditSeconds = 2

type GameStateMessage struct {
	MessageID        string `json:"messageID"`
	MessageChannelID string `json:"messageChannelID"`
	MessageAuthorID  string `json:"messageAuthorID"`
	LeaderID         string `json:"leaderID"`
}

func MakeGameStateMessage() GameStateMessage {
	return GameStateMessage{
		MessageID:        "",
		MessageChannelID: "",
		LeaderID:         "",
	}
}

func (dgs *GameState) Exists() bool {
	return dgs.GameStateMsg.MessageID != ""
}

func (dgs *GameState) AddReaction(s *discordgo.Session, emoji string) {
	if dgs.GameStateMsg.MessageID != "" {
		addReaction(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID, emoji)
	}
}

func (dgs *GameState) RemoveAllReactions(s *discordgo.Session) {
	if dgs.GameStateMsg.MessageID != "" {
		removeAllReactions(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
	}
}

func (dgs *GameState) AddAllReactions(s *discordgo.Session, emojis []Emoji) {
	for _, e := range emojis {
		dgs.AddReaction(s, e.FormatForReaction())
	}
	dgs.AddReaction(s, "❌")
}

func (dgs *GameState) DeleteGameStateMsg(s *discordgo.Session) {
	if dgs.GameStateMsg.MessageID != "" {
		deleteMessage(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
		dgs.GameStateMsg.MessageID = ""
	}
}

var DeferredEdits = make(map[string]*discordgo.MessageEmbed)
var DeferredEditsLock = sync.Mutex{}

// Note this is not a pointer; we never expect the underlying DGS to change on an edit
func (dgs GameState) Edit(s *discordgo.Session, me *discordgo.MessageEmbed) bool {
	newEdit := false

	if !ValidFields(me) {
		return false
	}

	DeferredEditsLock.Lock()

	// if it isn't found, then start the worker to wait to start it (this is a UNIQUE edit)
	if _, ok := DeferredEdits[dgs.GameStateMsg.MessageID]; !ok {
		go deferredEditWorker(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
		newEdit = true
	}
	// whether or not it's found, replace the contents with the new message
	DeferredEdits[dgs.GameStateMsg.MessageID] = me
	DeferredEditsLock.Unlock()
	return newEdit
}

func ValidFields(me *discordgo.MessageEmbed) bool {
	for _, v := range me.Fields {
		if v == nil {
			return false
		}
		if v.Name == "" || v.Value == "" {
			return false
		}
	}
	return true
}

func RemovePendingDGSEdit(messageID string) {
	DeferredEditsLock.Lock()
	delete(DeferredEdits, messageID)
	DeferredEditsLock.Unlock()
}

func deferredEditWorker(s *discordgo.Session, channelID, messageID string) {
	time.Sleep(time.Second * time.Duration(DeferredEditSeconds))

	DeferredEditsLock.Lock()
	me := DeferredEdits[messageID]
	delete(DeferredEdits, messageID)
	DeferredEditsLock.Unlock()

	if me != nil {
		editMessageEmbed(s, channelID, messageID, me)
	}
}

func (dgs *GameState) CreateMessage(s *discordgo.Session, me *discordgo.MessageEmbed, channelID string, authorID string) {
	dgs.GameStateMsg.LeaderID = authorID
	msg := sendMessageEmbed(s, channelID, me)
	if msg != nil {
		dgs.GameStateMsg.MessageAuthorID = msg.Author.ID
		dgs.GameStateMsg.MessageChannelID = msg.ChannelID
		dgs.GameStateMsg.MessageID = msg.ID
	}
}

func (dgs *GameState) SameChannel(channelID string) bool {
	if dgs.GameStateMsg.MessageID != "" {
		return dgs.GameStateMsg.MessageChannelID == channelID
	}
	return false
}

func (dgs *GameState) IsReactionTo(m *discordgo.MessageReactionAdd) bool {
	if !dgs.Exists() {
		return false
	}

	return m.ChannelID == dgs.GameStateMsg.MessageChannelID && m.MessageID == dgs.GameStateMsg.MessageID && m.UserID != dgs.GameStateMsg.MessageAuthorID
}
