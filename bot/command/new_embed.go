package command

import (
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// newSuccessEmbed tells the user how to connect their capture to the game that was just created. The happy path is a
// single click: the link opens the capture via its aucapture:// protocol handler and connects it. Everything else
// (installing the capture, pasting the link, entering the server URL and code by hand) is presented as a fallback.
// The aucapture:// link itself is sent as the message content, not in the embed (see NewResponse).
func newSuccessEmbed(info NewInfo, sett *settings.GuildSettings) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.new.success.title",
			Other: "🎮 Game created!",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID: "commands.new.success.connectSteps",
			Other: "## [Click here to connect your capture]({{.apiHyperlink}})\n" +
				"AutoMuteUs Capture will open on its own and connect to this game.\n\n" +
				"**Nothing happens?**\n" +
				"• Don't have the capture yet? [Download it here]({{.downloadURL}}) and run it once. Then click the link above again.\n" +
				"• Or, paste the `aucapture://` link at the top of this message into your web browser's address bar.\n" +
				"• Or, type the server URL and code below into the capture yourself and press Connect.",
		}, map[string]interface{}{
			"apiHyperlink": info.ApiHyperlink,
			"downloadURL":  CaptureDownloadURL,
		}) + "\n\n" + sett.LocalizeMessage(&i18n.Message{
			// The click-here link opens the Capture App; the Capture Mod is handed the aucapture:// link instead.
			ID: "commands.new.success.captureModCopy",
			Other: "**Using the Capture Mod?** Copy the `aucapture://` link at the top of this message, then press **Paste capture link** in Among Us. " +
				"On mobile, long-press the message and choose **Copy Text**.",
		}),
		Color: 15844367, // GOLD
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "https://raw.githubusercontent.com/automuteus/automuteus/refs/heads/master/assets/BotProfilePicture.png",
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				// Only the capture uses this URL; opening it in a browser does nothing useful.
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.new.success.serverURL",
					Other: "Server URL (manual connect only)",
				}),
				// Inline code keeps Discord from turning it into a clickable link.
				Value:  "`" + info.MinimalURL + "`",
				Inline: true,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.new.success.code",
					Other: "Code",
				}),
				Value:  "`" + info.ConnectCode + "`",
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
