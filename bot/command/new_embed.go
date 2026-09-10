package command

import (
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// newSuccessEmbed tells the user how to connect their capture to the game that was just created. The happy path is a
// single click: the link opens the capture via its aucapture:// protocol handler and connects it. Everything else
// (installing the capture, pasting the link, entering the URL and code by hand) is presented as a fallback.
func newSuccessEmbed(info NewInfo, sett *settings.GuildSettings) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.new.success.title",
			Other: "🎮 Game created!",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID: "commands.new.success.steps",
			Other: "## [Click here to connect your capture]({{.apiHyperlink}})\n" +
				"AutoMuteUs Capture will open on its own and connect to this game.\n\n" +
				"**Nothing happens?**\n" +
				"• Don't have the capture yet? [Download it here]({{.downloadURL}}) and run it once. Then click the link above again.\n" +
				"• Or, try pasting this link into your web browser's address bar: `{{.hyperlink}}`\n" +
				"• Or, type the URL and code below into the capture yourself and press Connect.\n" +
				"**Didn't work or it errors?**\n" +
				"The capture also needs the [.NET 5 Desktop Runtime]({{.runtimeURL}}) or it won't open.",
		}, map[string]interface{}{
			"apiHyperlink": info.ApiHyperlink,
			"downloadURL":  CaptureDownloadURL,
			"runtimeURL":   DotNetRuntimeDownloadURL,
			"hyperlink":    info.Hyperlink,
		}),
		Color: 15844367, // GOLD
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "https://raw.githubusercontent.com/automuteus/automuteus/refs/heads/master/assets/BotProfilePicture.png",
		},
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
		Footer: &discordgo.MessageEmbedFooter{
			Text: sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.new.success.footer",
				Other: "Once the capture connects, the game message in this channel will update.",
			}),
		},
	}
}
