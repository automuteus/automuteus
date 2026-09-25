package command

import (
	"strings"

	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// Settings no longer edits anything itself: guild settings are managed on the web dashboard, and this command
// hands out the link to this server's page there.
var Settings = discordgo.ApplicationCommand{
	Name:        "settings",
	Description: "Get a link to the web dashboard, where AutoMuteUs settings for this server are managed",
}

// SettingsURL is the dashboard page for one guild's settings. webURL is the bot's WEB_URL, with or without a
// trailing slash; the page itself still requires the visitor to own or manage the guild.
func SettingsURL(webURL, guildID string) string {
	return strings.TrimRight(webURL, "/") + "/settings?guild=" + guildID
}

// SettingsResponse is the private reply to /settings: a short explanation plus a link button to the dashboard.
func SettingsResponse(webURL, guildID string, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	url := SettingsURL(webURL, guildID)
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
			Content: sett.LocalizeMessage(&i18n.Message{
				ID: "commands.settings.dashboard",
				Other: "Settings for this server are managed on the web dashboard. Sign in with Discord; the server owner " +
					"and members with Administrator or Manage Server can change them.\n<{{.URL}}>",
			}, map[string]interface{}{
				"URL": url,
			}),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Style: discordgo.LinkButton,
							URL:   url,
							Label: sett.LocalizeMessage(&i18n.Message{
								ID:    "commands.settings.open",
								Other: "Open settings",
							}),
							// Every button needs an emoji: discordgo 0.27 serializes a missing one as an empty
							// name, which Discord rejects with COMPONENT_INVALID_EMOJI.
							Emoji: discordgo.ComponentEmoji{Name: "⚙️"},
						},
					},
				},
			},
		},
	}
}
