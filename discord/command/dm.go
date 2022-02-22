package command

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func DmResponse(sett *settings.GuildSettings) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: sett.LocalizeMessage(&i18n.Message{
				ID: "commands.dm",
				Other: "Sorry, I cant't respond to DM. " +
					"Please execute the command in text channel instead.",
			}),
		},
	}
}
