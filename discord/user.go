package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
)

// User struct
type User struct {
	Nick          string `json:"Nick"`
	UserID        string `json:"UserID"`
	UserName      string `json:"UserName"`
	Discriminator string `json:"Discriminator"`
	OriginalNick  string `json:"OriginalNick"`
}

// UserData struct
type UserData struct {
	User               User   `json:"User"`
	PendingVoiceUpdate bool   `json:"PendingVoiceUpdate"`
	InGameName         string `json:"PlayerName"`
}

func MakeUserDataFromDiscordUser(dUser *discordgo.User, nick string) UserData {
	return UserData{
		User: User{
			Nick:          nick,
			UserID:        dUser.ID,
			UserName:      dUser.Username,
			Discriminator: dUser.Discriminator,
			OriginalNick:  nick,
		},
		PendingVoiceUpdate: false,
		InGameName:         game.UnlinkedPlayerName,
	}
}

func (user *UserData) IsPendingVoiceUpdate() bool {
	return user.PendingVoiceUpdate
}

func (user *UserData) SetPendingVoiceUpdate(is bool) {
	user.PendingVoiceUpdate = is
}

func (user *UserData) GetNickName() string {
	return user.User.Nick
}

func (user *UserData) GetOriginalNickName() string {
	return user.User.OriginalNick
}

func (user *UserData) NicknamesMatch() bool {
	return user.User.Nick == user.User.OriginalNick
}

func (user *UserData) GetUserName() string {
	return user.User.UserName
}

func (user *UserData) GetID() string {
	return user.User.UserID
}

func (user *UserData) GetPlayerName() string {
	return user.InGameName
}

func (user *UserData) Link(player game.PlayerData) {
	user.InGameName = player.Name
}

//
//// AmongUsPlayerMatch determines if a player is in the game
//func (user *UserData) AmongUsPlayerMatch(player game.Player) bool {
//	return user.AuData.Color == player.Color && user.AuData.Name == player.Name
//}
