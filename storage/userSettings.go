package storage

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type UserSettings struct {
}

func MakeUserSettings() *UserSettings {
	return &UserSettings{}
}

func (userSettings *UserSettings) ToEmbed(sett *GuildSettings) *discordgo.MessageEmbed {
	jBytes, err := json.MarshalIndent(userSettings, "", "  ")
	if err != nil {
		log.Println(err)
	}

	return &discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "userSettings.ToEmbed.Title",
			Other: "Your Settings",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID:    "userSettings.ToEmbed.Description",
			Other: "Here's all the settings I have for you",
		}),
		Timestamp: "",
		Color:     3066993, //GREEN
		Image:     nil,
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "userSettings.ToEmbed.FielnName",
					Other: "Settings",
				}),
				Value:  fmt.Sprintf("```JSON\n%s\n```", jBytes),
				Inline: true,
			},
		},
	}
}
