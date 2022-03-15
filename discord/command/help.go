package command

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var Help = discordgo.ApplicationCommand{
	Name:        "help",
	Description: "AutoMuteUs help",

	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "command",
			Description: "Command to view details for",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{
					Name:  New.Name,
					Value: New.Name,
				},
				{
					Name:  Refresh.Name,
					Value: Refresh.Name,
				},
				{
					Name:  Pause.Name,
					Value: Pause.Name,
				},
				{
					Name:  End.Name,
					Value: End.Name,
				},
				{
					Name:  Link.Name,
					Value: Link.Name,
				},
				{
					Name:  Unlink.Name,
					Value: Unlink.Name,
				},
				{
					Name:  Settings.Name,
					Value: Settings.Name,
				},
				{
					Name:  Privacy.Name,
					Value: Privacy.Name,
				},
				{
					Name:  Info.Name,
					Value: Info.Name,
				},
				{
					Name:  Map.Name,
					Value: Map.Name,
				},
				{
					Name:  Stats.Name,
					Value: Stats.Name,
				},
				{
					Name:  Premium.Name,
					Value: Premium.Name,
				},
			},
			Required: false,
		},
	},
}

func HelpResponse(sett *settings.GuildSettings, options []*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionResponse {
	if len(options) > 0 {
		cmd := getCommand(options[0].StringValue())
		embed := constructEmbedForCommand(cmd, sett)
		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		}
	} else {
		m := HelpEmbedResponse(All, sett)
		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{&m},
			},
		}
	}
}

func HelpEmbedResponse(commands []*discordgo.ApplicationCommand, sett *settings.GuildSettings) discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.help.title",
			Other: "AutoMuteUs Bot Commands:\n",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.help.subtitle",
			Other: "[View the Github Project](https://github.com/automuteus/automuteus) or [Join our Discord](https://discord.gg/ZkqZSWF)\n\nType `/help <command>` to see more details on a command!",
		}),
		Timestamp: "",
		Color:     15844367, // GOLD
		Image:     nil,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL:      "https://github.com/automuteus/automuteus/blob/master/assets/BotProfilePicture.png?raw=true",
			ProxyURL: "",
			Width:    0,
			Height:   0,
		},
		Video:    nil,
		Provider: nil,
		Author:   nil,
		Footer:   nil,
	}

	fields := make([]*discordgo.MessageEmbedField, 0)
	for _, v := range commands {
		if v.Name != "help" {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("`/%s`", v.Name),
				Value:  localizeCommandDescription(v, sett),
				Inline: true,
			})
		}
	}
	if len(fields)%3 == 2 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "\u200B",
			Value:  "\u200B",
			Inline: true,
		})
	}

	embed.Fields = fields
	return embed
}
