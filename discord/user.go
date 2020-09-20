package discord

import (
	"fmt"

	"github.com/denverquane/amongusdiscord/game"
)

// User struct
type User struct {
	nick          string
	userID        string
	userName      string
	discriminator string
}

// UserData struct
type UserData struct {
	user               User
	pendingVoiceUpdate bool
	auData             *AmongUserData //we want to point to player data that isn't necessarily correlated with a player yet...
}

// IsAlive for a user
func (user *UserData) IsAlive() bool {
	if user.auData != nil {
		return user.auData.IsAlive
	}
	return true //Assume that users we can't correlate to among us game data are always alive (safer policy)
}

// AmongUsPlayerMatch determines if a player is in the game
func (user *UserData) AmongUsPlayerMatch(player game.Player) bool {
	return user.auData != nil && user.auData.Color == player.Color && user.auData.Name == player.Name
}

// AmongUserData struct
type AmongUserData struct {
	Color   int
	Name    string
	IsAlive bool
}

// ToString a user
func (auData *AmongUserData) ToString() string {

	return fmt.Sprintf("{ Name: %s, Color: %s, Alive: %v }\n", auData.Name, GetColorStringForInt(auData.Color), auData.IsAlive)
}

func (auData *AmongUserData) isDifferent(player game.Player) bool {
	return auData.IsAlive != !player.IsDead || auData.Color != player.Color || auData.Name != player.Name
}

func shouldBeMuted(phase game.Phase, user UserData, channelID string, trackedChannels map[string]Tracking) bool {
	//don't mute users who aren't linked to in-game data
	if user.auData == nil {
		return false
	}

	if len(trackedChannels) == 0 || channelID == "" {
		//do nothing; we are tracked/ we count
	} else {
		tracked := false
		for _, v := range trackedChannels {
			if v.channelID == channelID {
				tracked = true
				break
			}
		}
		if !tracked {
			return false //if not tracking the channel this person is in, default to unmuted
		}
	}

	switch phase {
	case game.LOBBY:
		return false //unmute all players in lobby
	case game.DISCUSS:
		return !user.IsAlive() //if we're in discussion, then mute the player if they're dead
	case game.TASKS:
		return true //mute all players in tasks
	default:
		return false
	}
}
