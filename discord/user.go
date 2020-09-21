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

// return value is mute, deaf
func getVoiceStateChanges(guild *GuildState, user UserData, voiceChannelID string) (bool, bool) {
	if user.auData == nil || len(guild.Tracking) == 0 || voiceChannelID == "" {
		return false, false
	}

	if isVoiceChannelTracked(voiceChannelID, guild.Tracking) {
		playerMuteStates := map[game.Phase]bool{
			game.LOBBY:   false,
			game.TASKS:   true,
			game.DISCUSS: !user.IsAlive(),
		}
		if guild.MoveDeadPlayers {
			// isAlive -> gamePhase => mute
			playerMuteStates = map[game.Phase]bool{
				game.LOBBY:   false,
				game.TASKS:   user.IsAlive(),
				game.DISCUSS: !user.IsAlive(),
			}
			playerDeafStates := map[game.Phase]bool{
				game.LOBBY:   false,
				game.TASKS:   user.IsAlive(),
				game.DISCUSS: false,
			}
			return playerMuteStates[guild.GamePhase], playerDeafStates[guild.GamePhase]
		}

		return playerMuteStates[guild.GamePhase], false
	}

	return false, false
}
