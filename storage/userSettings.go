package storage

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
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
		URL:         "",
		Type:        "",
		Title:       "Your Settings",
		Description: "Here's all the settings I have for you",
		Timestamp:   "",
		Color:       3066993, //GREEN
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{
				Name:   "Settings",
				Value:  fmt.Sprintf("```JSON\n%s\n```", jBytes),
				Inline: true,
			},
		},
	}
}
