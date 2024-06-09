package bot

import (
	"github.com/j0nas500/automuteus-tor/v8/pkg/amongus"
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
}
