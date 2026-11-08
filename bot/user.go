package bot

import (
	"github.com/automuteus/automuteus/v8/pkg/amongus"
	"github.com/bwmarrin/discordgo"
)

// User struct
type User struct {
	Nick     string `json:"Nick"`
	UserID   string `json:"UserID"`
	UserName string `json:"UserName"`
}

// UserData struct
type UserData struct {
	User         User   `json:"User"`
	ShouldBeMute bool   `json:"ShouldBeMute"`
	ShouldBeDeaf bool   `json:"ShouldBeDeaf"`
	InGameName   string `json:"PlayerName"`
	// FriendCode is the Friend Code of the linked player, when the Capture reports one. It identifies them even
	// after a name change, or after leaving and rejoining.
	FriendCode string `json:"FriendCode,omitempty"`
}

func MakeUserDataFromDiscordUser(dUser *discordgo.User, nick string) UserData {
	return UserData{
		User: User{
			Nick:     nick,
			UserID:   dUser.ID,
			UserName: dUser.Username,
		},
		ShouldBeDeaf: false,
		ShouldBeMute: false,
		InGameName:   amongus.UnlinkedPlayerName,
	}
}

func (user *UserData) GetNickName() string {
	return user.User.Nick
}

func (user *UserData) SetShouldBeMuteDeaf(mute, deaf bool) {
	user.ShouldBeMute = mute
	user.ShouldBeDeaf = deaf
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

func (user *UserData) Link(player amongus.PlayerData) {
	user.InGameName = player.Name
	user.FriendCode = player.FriendCode
}

func (user *UserData) Unlink() {
	user.InGameName = amongus.UnlinkedPlayerName
	user.FriendCode = ""
}

// IsLinkedTo reports whether the user is linked to the in-game player. When both sides have a Friend Code it alone
// decides, so a different account playing under the same name is not mistaken for them.
func (user *UserData) IsLinkedTo(player amongus.PlayerData) bool {
	if user.InGameName == amongus.UnlinkedPlayerName {
		return false
	}
	if user.FriendCode != "" && player.FriendCode != "" {
		return user.FriendCode == player.FriendCode
	}
	return user.InGameName == player.Name
}
