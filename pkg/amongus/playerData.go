package amongus

import (
	"github.com/automuteus/automuteus/v8/pkg/game"
)

type PlayerData struct {
	Color   int    `json:"color"`
	Name    string `json:"name"`
	IsAlive bool   `json:"isAlive"`
	// FriendCode is empty when the Capture doesn't report it; see game.Player.
	FriendCode string `json:"friendCode,omitempty"`
}

const UnlinkedPlayerName = "UnlinkedPlayer"

var UnlinkedPlayer = PlayerData{
	Color:   -1,
	Name:    UnlinkedPlayerName,
	IsAlive: true,
}

// FriendCodeCacheKeyPrefix marks a Friend Code among the in-game names in a guild's cache of who has played as whom.
const FriendCodeCacheKeyPrefix = "fc:"

// FriendCodeCacheKey is the key a Friend Code is remembered under in a guild's cache.
func FriendCodeCacheKey(friendCode string) string {
	return FriendCodeCacheKeyPrefix + friendCode
}

func (auData *PlayerData) isDifferent(player game.Player) bool {
	return auData.IsAlive != !player.IsDead || auData.Color != player.Color || auData.Name != player.Name ||
		auData.FriendCode != player.FriendCode
}
