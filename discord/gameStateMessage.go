package discord

import (
	"github.com/automuteus/automuteus/metrics"
	"github.com/automuteus/utils/pkg/settings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// bumped for public rollout. Don't need to update the status message more than once every 2 secs prob
const DeferredEditSeconds = 2
const colorSelectID = "select-color"

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

func (gsm *GameStateMessage) Exists() bool {
	return gsm.MessageID != "" && gsm.MessageChannelID != ""
}

func (dgs *GameState) DeleteGameStateMsg(s *discordgo.Session, reset bool) bool {
	if dgs.GameStateMsg.Exists() {
		err := s.ChannelMessageDelete(dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
		if err != nil {
			return false
		}
		if reset {
			dgs.GameStateMsg = MakeGameStateMessage()
		}
		return true
	}
	return false
}

var DeferredEdits = make(map[string]*discordgo.MessageEmbed)
var DeferredEditsLock = sync.Mutex{}

// Note this is not a pointer; we never expect the underlying DGS to change on an edit
func (dgs GameState) dispatchEdit(s *discordgo.Session, me *discordgo.MessageEmbed) (newEdit bool) {
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

func (dgs GameState) shouldRefresh() bool {
	// discord dictates that we can't edit messages that are older than 1 hour
	return (time.Now().Sub(time.Unix(dgs.GameStateMsg.CreationTimeUnix, 0))) > time.Hour
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

func (dgs *GameState) CreateMessage(s *discordgo.Session, me *discordgo.MessageEmbed, channelID string, authorID string) bool {
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    colorSelectID,
					Placeholder: "Select your in-game color",
					Options:     EmojisToSelectMenuOptions(GlobalAlivenessEmojis[true], X),
				},
			},
		},
	}
	msg := sendEmbedWithComponents(s, channelID, me, components)
	if msg != nil {
		dgs.GameStateMsg.LeaderID = authorID
		dgs.GameStateMsg.MessageChannelID = msg.ChannelID
		dgs.GameStateMsg.MessageID = msg.ID
		dgs.GameStateMsg.CreationTimeUnix = time.Now().Unix()
		return true
	}
	return false
}

func (bot *Bot) DispatchRefreshOrEdit(readOnlyDgs *GameState, dgsRequest GameStateRequest, sett *settings.GuildSettings) {
	if readOnlyDgs.shouldRefresh() {
		bot.RefreshGameStateMessage(dgsRequest, sett)
	} else {
		edited := readOnlyDgs.dispatchEdit(bot.PrimarySession, bot.gameStateResponse(readOnlyDgs, sett))
		if edited {
			metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
		}
	}
}
