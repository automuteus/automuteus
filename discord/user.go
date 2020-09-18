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

func (user *UserData) IsAlive() bool {
	if user.auData != nil {
		return user.auData.IsAlive
	}
	return true //Assume that users we can't correlate to among us game data are always alive (safer policy)
}

func (user *UserData) AmongUsPlayerMatch(player game.Player) bool {
	return user.auData != nil && user.auData.Color == player.Color && user.auData.Name == player.Name
}

type AmongUserData struct {
	Color   int
	Name    string
	IsAlive bool
}

func (auData *AmongUserData) ToString() string {

	return fmt.Sprintf("{ Name: %s, Color: %s, Alive: %v }\n", auData.Name, GetColorStringForInt(auData.Color), auData.IsAlive)
}

func (auData *AmongUserData) isDifferent(player game.Player) bool {
	return auData.IsAlive != !player.IsDead || auData.Color != player.Color || auData.Name != player.Name
}

func MakeAllEmptyAmongUsData() []AmongUserData {
	allData := make([]AmongUserData, 12)
	for i := 0; i < 12; i++ {
		allData[i] = AmongUserData{
			Color:   i,
			Name:    "",
			IsAlive: true,
		}
	}
	return allData
}
