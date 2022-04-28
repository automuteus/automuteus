package command

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type NewStatus int

const (
	NewSuccess NewStatus = iota
	NewNoVoiceChannel
	NewLockout
)

type NewInfo struct {
	Hyperlink   string
	MinimalURL  string
	ConnectCode string
	ActiveGames int64
}

var New = discordgo.ApplicationCommand{
	Name:        "new",
	Description: "Start a new game",
}

func NewResponse(status NewStatus, info NewInfo, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var content string
	var embeds []*discordgo.MessageEmbed
	var flags uint64 = 1 << 6 // private message by default

	switch status {
	case NewSuccess:
		content = sett.LocalizeMessage(&i18n.Message{
			ID: "commands.new.success",
			Other: "Click the following link to link your capture: \n <{{.hyperlink}}>\n\n" +
				"Don't have the capture installed? Latest version [here]({{.downloadURL}})\n\nTo link your capture manually:",
		},
			map[string]interface{}{
				"hyperlink":   info.Hyperlink,
				"downloadURL": CaptureDownloadURL,
			})
		embeds = []*discordgo.MessageEmbed{
			{
				Fields: []*discordgo.MessageEmbedField{
					{
						Name: sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.new.success.url",
							Other: "URL",
						}),
						Value:  info.MinimalURL,
						Inline: true,
					},
					{
						Name: sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.new.success.code",
							Other: "Code",
						}),
						Value:  info.ConnectCode,
						Inline: true,
					},
				},
			},
		}
	case NewNoVoiceChannel:
		content = sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.new.nochannel",
			Other: "Please join a voice channel before starting a match!",
		})
	case NewLockout:
		content = sett.LocalizeMessage(&i18n.Message{
			ID: "commands.new.lockout",
			Other: "If I start any more games, Discord will lock me out, or throttle the games I'm running! 😦\n" +
				"Please try again in a few minutes, or consider AutoMuteUs Premium (`/premium info`)\n" +
				"Current Games: {{.Games}}",
		}, map[string]interface{}{
			"Games": fmt.Sprintf("%d/%d", info.ActiveGames, DefaultMaxActiveGames),
		})
		flags = 0 // public message

	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   flags,
			Content: content,
			Embeds:  embeds,
		},
	}
}
