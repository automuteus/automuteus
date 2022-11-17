package command

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"time"
)

const (
	Category = "category"
)

var Download = discordgo.ApplicationCommand{
	Name:        "download",
	Description: "Download AutoMuteUs data",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:        Category,
			Description: "Data to download",
			Type:        discordgo.ApplicationCommandOptionString,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				&discordgo.ApplicationCommandOptionChoice{
					Name:  Guild,
					Value: Guild,
				},
				// TODO add option to download individual user data?
			},
			Required: true,
		},
	},
}

func GetDownloadParams(options []*discordgo.ApplicationCommandInteractionDataOption) string {
	return options[0].StringValue()
}

func DownloadGuildOnCooldownResponse(sett *settings.GuildSettings, duration time.Duration) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: 1 << 6,
			Content: sett.LocalizeMessage(&i18n.Message{
				ID: "commands.download.guild.cooldown",
				Other: "Sorry, guild stats can only downloaded once every 24 hours!\n\n" +
					"Please wait {{.Duration}} and then try again",
			}, map[string]interface{}{
				"Duration": duration.Truncate(time.Hour).String(),
			}),
		},
	}
}

func DownloadNotGoldResponse(sett *settings.GuildSettings) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: 1 << 6,
			Content: sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.download.nogold",
				Other: "Downloading AutoMuteUs data is reserved for Gold subscribers only!",
			}),
		},
	}
}
