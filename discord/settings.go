package discord

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
)

type Setting struct {
	command         string
	description     string
	fullDescription string
	usage           string
}

var AllSettings = []Setting{
	{
		command:         "prefix",
		description:     "Change the command prefix",
		fullDescription: "Change the command prefix used by this bot",
		usage:           "prefix <prefix>",
	},
}

func TopSettingsMessage(prefix string) *discordgo.MessageEmbed {
	var embed = discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       "Settings",
		Description: fmt.Sprintf(""),
		Timestamp:   "",
		Color:       3066993, //GREEN
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      settingsToEmbedFields(prefix, AllSettings),
	}
	return &embed
}

func settingsToEmbedFields(prefix string, settings []Setting) []*discordgo.MessageEmbedField {
	fields := make([]*discordgo.MessageEmbedField, len(settings))
	for i, v := range settings {
		fields[i] = &discordgo.MessageEmbedField{
			Name:   prefix + v.command,
			Value:  v.description,
			Inline: false,
		}
	}
	return fields
}
