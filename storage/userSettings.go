package storage

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/locale"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type UserSettings struct {
	CacheNames bool `json:"cacheNames"`
}

func MakeUserSettings() *UserSettings {
	return &UserSettings{
		CacheNames: true,
	}
}

func (userSettings *UserSettings) ToEmbed() *discordgo.MessageEmbed {
	jBytes, err := json.MarshalIndent(userSettings, "", "  ")
	if err != nil {
		log.Println(err)
	}

	return &discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: locale.LocalizeMessage(&i18n.Message{
			ID:    "userSettings.ToEmbed.Title",
			Other: "Your Settings",
		}),
		Description: locale.LocalizeMessage(&i18n.Message{
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
				Name: locale.LocalizeMessage(&i18n.Message{
					ID:    "userSettings.ToEmbed.FielnName",
					Other: "Settings",
				}),
				Value:  fmt.Sprintf("```JSON\n%s\n```", jBytes),
				Inline: true,
			},
		},
	}
}
