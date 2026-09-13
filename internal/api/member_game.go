package api

import (
	"encoding/json"
	"errors"

	"github.com/automuteus/automuteus/v8/pkg/game"
)

// MemberGameState explicitly lists public fields. Do not embed bot or capture
// structs: their future fields must not silently become part of the user API.
type MemberGameState struct {
	GuildID      string `json:"guildID"`
	Running      bool   `json:"running"`
	Linked       bool   `json:"linked"`
	VoiceChannel string `json:"voiceChannel"`
	AmongUsData  struct {
		Phase      game.Phase   `json:"phase"`
		Region     string       `json:"region"`
		Map        game.PlayMap `json:"map"`
		PlayerData map[string]struct {
			Name    string `json:"name"`
			Color   int    `json:"color"`
			IsAlive bool   `json:"isAlive"`
		} `json:"playerData"`
	} `json:"amongUsData"`
}

func memberGameState(raw json.RawMessage, guildID, connectCode string) (MemberGameState, error) {
	var identity struct {
		GuildID     string `json:"guildID"`
		ConnectCode string `json:"connectCode"`
	}
	var view MemberGameState
	if json.Unmarshal(raw, &identity) != nil || identity.GuildID != guildID || identity.ConnectCode != connectCode {
		return view, errors.New("game does not match requested guild and code")
	}
	err := json.Unmarshal(raw, &view)
	return view, err
}

func memberRoomCode(raw json.RawMessage) (string, error) {
	var state struct {
		AmongUsData struct {
			Room string `json:"room"`
		} `json:"amongUsData"`
	}
	err := json.Unmarshal(raw, &state)
	return state.AmongUsData.Room, err
}
