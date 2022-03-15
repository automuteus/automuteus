package command

import (
	"bytes"
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/automuteus/utils/pkg/storage"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	PrivacyInfo   = "info"
	PrivacyShowMe = "show-me"
	PrivacyOptIn  = "opt-in"
	PrivacyOptOut = "opt-out"
)

var Privacy = discordgo.ApplicationCommand{
	Name:        "privacy",
	Description: "View AMU privacy info",

	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:        "command",
			Description: "Privacy command",
			Type:        discordgo.ApplicationCommandOptionString,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{
					Name:  PrivacyInfo,
					Value: PrivacyInfo,
				},
				{
					Name:  PrivacyShowMe,
					Value: PrivacyShowMe,
				},
				{
					Name:  PrivacyOptIn,
					Value: PrivacyOptIn,
				},
				{
					Name:  PrivacyOptOut,
					Value: PrivacyOptOut,
				},
			},
			Required: false,
		},
	},
}

func GetPrivacyParam(options []*discordgo.ApplicationCommandInteractionDataOption) string {
	if len(options) == 0 {
		return PrivacyInfo
	}
	return options[0].StringValue()
}

func PrivacyResponse(status string, cached map[string]interface{}, user *storage.PostgresUser, err error, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var content string
	switch status {
	case PrivacyInfo:
		content = sett.LocalizeMessage(&i18n.Message{
			ID: "commands.privacy.info",
			Other: "AutoMuteUs privacy and data collection details.\n" +
				"More details [here](https://github.com/automuteus/automuteus/blob/master/PRIVACY.md)",
		})
	case PrivacyShowMe:
		if len(cached) == 0 {
			content = sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.privacy.showme.nocache",
				Other: "❌ I don't have any cached player names stored for you!",
			})
		} else {
			buf := bytes.NewBuffer([]byte(sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.privacy.showme.cache",
				Other: "❗ Here's your cached in-game names:",
			})))
			buf.WriteString("\n```\n")
			for n := range cached {
				buf.WriteString(fmt.Sprintf("%s\n", n))
			}
			buf.WriteString("```")
			content = buf.String()
		}
		if user != nil && user.UserID != 0 {
			content += "\n"
			if user.Opt {
				content += sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.privacy.showme.optin",
					Other: "❗ You are opted **in** to data collection for game statistics",
				})
			} else {
				content += sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.privacy.showme.optout",
					Other: "❌ You are opted **out** of data collection for game statistics, or you haven't played a game yet",
				})
			}
		}

	case PrivacyOptOut:
		fallthrough
	case PrivacyOptIn:
		if err == nil {
			content = sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.privacy.opt.success",
				Other: "✅ I successfully changed your opt in/out status",
			})
		} else {
			content = sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.privacy.opt.error",
				Other: "❌ I encountered an error changing your opt in/out status:\n`{{.Error}}`",
			}, map[string]interface{}{
				"Error": err.Error(),
			})
		}
	}

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   1 << 6,
			Content: content,
		},
	}
}
