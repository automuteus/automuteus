package bot

import (
	"fmt"
	"github.com/j0nas500/automuteus-tor/v8/pkg/amongus"
	"github.com/j0nas500/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
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

func (dgs *GameState) checkCacheAndAddUser(g *discordgo.Guild, s *discordgo.Session, userID string) (UserData, bool) {
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
	unsorted := make([]*discordgo.MessageEmbedField, 35)
	num := 0

	for _, player := range dgs.GameData.PlayerData {
		if player.Color < 0 || player.Color > 34 {
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
	for i := 0; i < 35; i++ {
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
