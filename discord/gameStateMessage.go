package discord

import (
	"github.com/denverquane/amongusdiscord/pkg/galactus_client"
	"log"
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

func (dgs *GameState) AddReaction(galactus *galactus_client.GalactusClient, emoji string) {
	if dgs.GameStateMsg.MessageID != "" {
		addReaction(galactus, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID, emoji)
	}
}

func (dgs *GameState) RemoveAllReactions(galactus *galactus_client.GalactusClient) {
	if dgs.GameStateMsg.MessageID != "" {
		removeAllReactions(galactus, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
	}
}

func (dgs *GameState) AddAllReactions(galactus *galactus_client.GalactusClient, emojis []Emoji) {
	for _, e := range emojis {
		dgs.AddReaction(galactus, e.FormatForReaction())
	}
	dgs.AddReaction(galactus, "❌")
}

func (dgs *GameState) DeleteGameStateMsg(client *galactus_client.GalactusClient) {
	if dgs.GameStateMsg.MessageID != "" {
		client.DeleteChannelMessage(dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
		dgs.GameStateMsg.MessageID = ""
	}
}

var DeferredEdits = make(map[string]*discordgo.MessageEmbed)
var DeferredEditsLock = sync.Mutex{}

// Note this is not a pointer; we never expect the underlying DGS to change on an edit
func (dgs GameState) Edit(galactus *galactus_client.GalactusClient, me *discordgo.MessageEmbed) bool {
	newEdit := false

	if !ValidFields(me) {
		return false
	}

	DeferredEditsLock.Lock()

	// if it isn't found, then start the worker to wait to start it (this is a UNIQUE edit)
	if _, ok := DeferredEdits[dgs.GameStateMsg.MessageID]; !ok {
		go deferredEditWorker(galactus, dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
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

func deferredEditWorker(galactus *galactus_client.GalactusClient, channelID, messageID string) {
	time.Sleep(time.Second * time.Duration(DeferredEditSeconds))

	DeferredEditsLock.Lock()
	me := DeferredEdits[messageID]
	delete(DeferredEdits, messageID)
	DeferredEditsLock.Unlock()

	if me != nil {
		galactus.EditChannelMessageEmbed(channelID, messageID, *me)
	}
}

func (dgs *GameState) CreateMessage(galactus *galactus_client.GalactusClient, me *discordgo.MessageEmbed, channelID string, authorID string) {
	dgs.GameStateMsg.LeaderID = authorID
	msg, err := galactus.SendChannelMessageEmbed(channelID, me)
	if err != nil {
		log.Println(err)
	}
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
