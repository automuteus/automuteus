package discord

import (
	"github.com/bwmarrin/discordgo"
	"time"
)

type GameStateMessage struct {
	MessageID        string `json:"messageID"`
	MessageChannelID string `json:"messageChannelID"`
	LeaderID         string `json:"leaderID"`
	CreationTimeUnix int64  `json:"creationTimeUnix"`
}

func MakeGameStateMessage() GameStateMessage {
	return GameStateMessage{
		MessageID:        "",
		MessageChannelID: "",
		LeaderID:         "",
		CreationTimeUnix: 0,
	}
}

// bumped for public rollout. Don't need to update the status message more than once every 2 secs prob
const DeferredEditSeconds = 2
const ColorSelectID = "select-color"

func (gsm *GameStateMessage) Exists() bool {
	return gsm.MessageID != "" && gsm.MessageChannelID != ""
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

func RemovePendingDGSEdit(messageID string) {
	DeferredEditsLock.Lock()
	delete(DeferredEdits, messageID)
	DeferredEditsLock.Unlock()
}
