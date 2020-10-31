package discord

import (
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

const DeferredEditSeconds = 1

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

func (dgs *DiscordGameState) Exists() bool {
	return dgs.GameStateMsg.MessageID != ""
}

func (dgs *DiscordGameState) AddReaction(s *discordgo.Session, emoji string) {
	if dgs.GameStateMsg.MessageID != "" {
		addReaction(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID, emoji)
	}
}

func (dgs *DiscordGameState) RemoveAllReactions(s *discordgo.Session) {
	if dgs.GameStateMsg.MessageID != "" {
		removeAllReactions(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
	}
}

func (dgs *DiscordGameState) AddAllReactions(s *discordgo.Session, emojis []Emoji) {
	for _, e := range emojis {
		dgs.AddReaction(s, e.FormatForReaction())
	}
	dgs.AddReaction(s, "❌")
}

func (dgs *DiscordGameState) DeleteGameStateMsg(s *discordgo.Session) {
	if dgs.GameStateMsg.MessageID != "" {
		go deleteMessage(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
		dgs.GameStateMsg.MessageID = ""
		dgs.GameStateMsg.MessageChannelID = ""
	}
}

var DeferredEdits = make(map[string]*discordgo.MessageEmbed)
var DeferredEditsLock = sync.Mutex{}

func (dgs *DiscordGameState) Edit(s *discordgo.Session, me *discordgo.MessageEmbed) {
	DeferredEditsLock.Lock()

	//if it isn't found, then start the worker to wait to start it
	if _, ok := DeferredEdits[dgs.GameStateMsg.MessageID]; !ok {
		go deferredEditWorker(s, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
	}
	//whether or not it's found, replace the contents with the new message
	DeferredEdits[dgs.GameStateMsg.MessageID] = me
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

func (dgs *DiscordGameState) CreateMessage(s *discordgo.Session, me *discordgo.MessageEmbed, channelID string, authorID string) {
	dgs.GameStateMsg.LeaderID = authorID
	msg := sendMessageEmbed(s, channelID, me)
	if msg != nil {
		dgs.GameStateMsg.MessageAuthorID = msg.Author.ID
		dgs.GameStateMsg.MessageChannelID = msg.ChannelID
		dgs.GameStateMsg.MessageID = msg.ID
	}
}

func (dgs *DiscordGameState) SameChannel(channelID string) bool {
	if dgs.GameStateMsg.MessageID != "" {
		return dgs.GameStateMsg.MessageChannelID == channelID
	}
	return false
}

func (dgs *DiscordGameState) IsReactionTo(m *discordgo.MessageReactionAdd) bool {
	if !dgs.Exists() {
		return false
	}

	return m.ChannelID == dgs.GameStateMsg.MessageChannelID && m.MessageID == dgs.GameStateMsg.MessageID && m.UserID != dgs.GameStateMsg.MessageAuthorID
}
