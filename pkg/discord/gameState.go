package discord

import (
	"fmt"
	"github.com/automuteus/automuteus/v8/pkg/amongus"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
	"sync"
	"time"
)

// GameState represents a full record of the entire current game's state. It is intended to be fully JSON-serializable,
// so that any shard/worker can pick up the game state and operate upon it (using locks as necessary)
type GameState struct {
	GuildID string `json:"guildID"`

	ConnectCode string `json:"connectCode"`

	Linked     bool `json:"linked"`
	Running    bool `json:"running"`
	Subscribed bool `json:"subscribed"`

	MatchID        int64 `json:"matchID"`
	MatchStartUnix int64 `json:"matchStartUnix"`

	UserData     UserDataSet `json:"userData"`
	VoiceChannel string      `json:"voiceChannel"`

	GameStateMsg GameStateMessage `json:"gameStateMessage"`

	GameData amongus.GameData `json:"amongUsData"`
}

func NewDiscordGameState(guildID string) *GameState {
	dgs := GameState{GuildID: guildID}
	dgs.Reset()
	return &dgs
}

func (dgs *GameState) DeleteGameStateMsg(s *discordgo.Session, reset bool) bool {
	retValue := false
	if dgs.GameStateMsg.Exists() {
		err := s.ChannelMessageDelete(dgs.GameStateMsg.MessageChannelID, dgs.GameStateMsg.MessageID)
		if err != nil {
			retValue = false
		} else {
			retValue = true
		}
	}
	// whether or not we were successful in deleting the message, reset the state
	if reset {
		dgs.GameStateMsg = MakeGameStateMessage()
	}
	return retValue
}

func (dgs *GameState) Reset() {
	// Explicitly does not reset the GuildID!
	dgs.ConnectCode = ""
	dgs.Linked = false
	dgs.Running = false
	dgs.Subscribed = false
	dgs.MatchID = -1
	dgs.MatchStartUnix = -1
	dgs.UserData = map[string]UserData{}
	dgs.VoiceChannel = ""
	dgs.GameStateMsg = MakeGameStateMessage()
	dgs.GameData = amongus.NewGameData()
}

func (dgs *GameState) CreateMessage(s *discordgo.Session, me *discordgo.MessageEmbed, channelID string, authorID string) bool {
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    ColorSelectID,
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

// Note this is not a pointer; we never expect the underlying DGS to change on an edit
func (dgs GameState) DispatchEdit(s *discordgo.Session, me *discordgo.MessageEmbed) (newEdit bool) {
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

// returns the description and color to use, based on the gamestate
// usage dictates DEFAULT should be overwritten by other state subsequently,
// whereas RED and DARK_ORANGE are error/flag values that should be passed on
func (dgs *GameState) DescriptionAndColor(sett *settings.GuildSettings) (string, int) {
	if !dgs.Linked {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.notLinked.Description",
			Other: "❌**No capture linked! Click the link above to connect!**❌",
		}), RED // red
	} else if !dgs.Running {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.makeDescription.GameNotRunning",
			Other: "\n⚠ **Bot is Paused!** ⚠\n\n",
		}), DARK_ORANGE
	}
	return "\n", DEFAULT

}

func (dgs *GameState) CheckCacheAndAddUser(g *discordgo.Guild, s *discordgo.Session, userID string) (UserData, bool) {
	if g == nil {
		return UserData{}, false
	}
	// check and see if they're cached first
	for _, v := range g.Members {
		if v.User != nil && v.User.ID == userID {
			user := MakeUserDataFromDiscordUser(v.User, v.Nick)
			dgs.UserData[v.User.ID] = user
			return user, true
		}
	}
	mem, err := s.GuildMember(g.ID, userID)
	if err != nil {
		log.Println(err)
		return UserData{}, false
	}
	user := MakeUserDataFromDiscordUser(mem.User, mem.Nick)
	dgs.UserData[mem.User.ID] = user
	return user, true
}

func (dgs *GameState) ToEmojiEmbedFields(emojis AlivenessEmojis, sett *settings.GuildSettings) []*discordgo.MessageEmbedField {
	unsorted := make([]*discordgo.MessageEmbedField, 18)
	num := 0

	for _, player := range dgs.GameData.PlayerData {
		if player.Color < 0 || player.Color > 17 {
			break
		}
		for _, userData := range dgs.UserData {
			if userData.InGameName == player.Name {
				emoji := emojis[player.IsAlive][player.Color]
				unsorted[player.Color] = &discordgo.MessageEmbedField{
					Name:   player.Name,
					Value:  fmt.Sprintf("%s <@!%s>", emoji.FormatForInline(), userData.GetID()),
					Inline: true,
				}
				break
			}
		}
		// no player matched; unlinked player
		if unsorted[player.Color] == nil {
			emoji := emojis[player.IsAlive][player.Color]
			unsorted[player.Color] = &discordgo.MessageEmbedField{
				Name: player.Name,
				Value: fmt.Sprintf("%s **%s**", emoji.FormatForInline(), sett.LocalizeMessage(&i18n.Message{
					ID:    "discordGameState.ToEmojiEmbedFields.Unlinked",
					Other: "Unlinked",
				})),
				Inline: true,
			}
		}
		num++
	}

	sorted := make([]*discordgo.MessageEmbedField, num)
	num = 0
	for i := 0; i < 18; i++ {
		if unsorted[i] != nil {
			sorted[num] = unsorted[i]
			num++
		}
	}
	// balance out the last row of embeds with an extra inline field
	if num%3 == 2 {
		sorted = append(sorted, &discordgo.MessageEmbedField{
			Name:   "\u200b",
			Value:  "\u200b",
			Inline: true,
		})
	}
	return sorted
}

var DeferredEdits = make(map[string]*discordgo.MessageEmbed)
var DeferredEditsLock = sync.Mutex{}

func (dgs GameState) ShouldRefresh() bool {
	// discord dictates that we can't edit messages that are older than 1 hour, so they should be refreshed
	return (time.Now().Sub(time.Unix(dgs.GameStateMsg.CreationTimeUnix, 0))) > time.Hour
}
