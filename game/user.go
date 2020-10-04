package game

import (
	"github.com/bwmarrin/discordgo"
)

// User struct
type User struct {
	nick          string
	userID        string
	userName      string
	discriminator string
	originalNick  string
}

// UserData struct
type UserData struct {
	user               User
	pendingVoiceUpdate bool
	cachedPlayerName   string
	auData             *PlayerData //we want to point to player data that isn't necessarily correlated with a player yet...
}

func MakeUserDataFromDiscordUser(dUser *discordgo.User, nick string) UserData {
	return UserData{
		user: User{
			nick:          nick,
			userID:        dUser.ID,
			userName:      dUser.Username,
			discriminator: dUser.Discriminator,
			originalNick:  nick,
		},
		cachedPlayerName:   "",
		pendingVoiceUpdate: false,
		auData:             nil,
	}
}

// IsAlive for a user
func (user *UserData) IsAlive() bool {
	if user.auData != nil {
		return user.auData.IsAlive
	}
	return true //Assume that users we can't correlate to among us game data are always alive (safer policy)
}

func (user *UserData) IsLinked() bool {
	return user.auData != nil
}

func (user *UserData) IsPendingVoiceUpdate() bool {
	return user.pendingVoiceUpdate
}

func (user *UserData) SetPendingVoiceUpdate(is bool) {
	user.pendingVoiceUpdate = is
}

func (user *UserData) GetNickName() string {
	return user.user.nick
}

func (user *UserData) GetOriginalNickName() string {
	return user.user.originalNick
}

func (user *UserData) NicknamesMatch() bool {
	return user.user.nick == user.user.originalNick
}

func (user *UserData) GetUserName() string {
	return user.user.userName
}

func (user *UserData) GetID() string {
	return user.user.userID
}

func (user *UserData) GetPlayerName() string {
	return user.cachedPlayerName
}

func (user *UserData) SetPlayerData(player *PlayerData) {
	if player != nil {
		user.cachedPlayerName = player.Name
	}

	user.auData = player
}

func (user *UserData) GetColor() int {
	if user.auData != nil {
		return user.auData.Color
	} else {
		return 0
	}
}

// AmongUsPlayerMatch determines if a player is in the game
func (user *UserData) AmongUsPlayerMatch(player Player) bool {
	return user.auData != nil && user.auData.Color == player.Color && user.auData.Name == player.Name
}
