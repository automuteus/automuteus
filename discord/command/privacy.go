package command

import (
	"bytes"
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/automuteus/utils/pkg/storage"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type PrivacyStatus int

const (
	PrivacyInfo PrivacyStatus = iota
	PrivacyShowMe
	PrivacyOptIn
	PrivacyOptOut
	PrivacyUnknown
	PrivacyCacheClear
)

var Privacy = discordgo.ApplicationCommand{
	Name:        "privacy",
	Description: "View AMU privacy info",

	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "command",
			Description: "Privacy command",
			Required:    false,
		},
		// TODO use subcommands here? like /privacy <cache> clear/show?
	},
}

func GetPrivacyParam(options []*discordgo.ApplicationCommandInteractionDataOption) PrivacyStatus {
	if len(options) == 0 {
		return PrivacyInfo
	}
	opt := options[0].StringValue()

	switch opt {
	case "info":
		fallthrough
	case "help":
		return PrivacyInfo

	case "show":
		fallthrough
	case "me":
		fallthrough
	case "cache":
		fallthrough
	case "showme":
		return PrivacyShowMe

	case "optin":
		return PrivacyOptIn

	case "optout":
		return PrivacyOptOut

	case "clear":
		fallthrough
	case "clearcache":
		fallthrough
	case "cacheclear":
		return PrivacyCacheClear

	default:
		return PrivacyUnknown
	}
}

func PrivacyResponse(status PrivacyStatus, cached map[string]interface{}, user *storage.PostgresUser, err error, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var content string
	switch status {
	case PrivacyUnknown:
		content = sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.privacy.unknownarg",
			Other: "❌ Sorry, I didn't recognize that argument",
		})
	case PrivacyInfo:
		content = sett.LocalizeMessage(&i18n.Message{
			ID: "commands.privacy.info",
			Other: "AutoMuteUs privacy and data collection details.\n" +
				"More details [here](https://github.com/automuteus/automuteus/blob/master/PRIVACY.md)\n" +
				"(I accept `showme`/`cache`,`optin`,`optout`, and `clearcache` as arguments)",
		})
	case PrivacyShowMe:
		if cached == nil || len(cached) == 0 {
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
	case PrivacyCacheClear:
		if err == nil {
			content = sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.privacy.cacheclear.success",
				Other: "✅ I successfully cleared your player name cache",
			})
		} else {
			content = sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.privacy.cacheclear.error",
				Other: "❌ I encountered an error clearing your player name cache:\n`{{.Error}}`",
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
