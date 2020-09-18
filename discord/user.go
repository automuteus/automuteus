package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
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
	user       User
	voiceState discordgo.VoiceState
	tracking   bool
	auData     *AmongUserData //we want to point to player data that isn't necessarily correlated with a player yet...
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
