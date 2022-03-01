package command

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	ISO8601               = "2006-01-02T15:04:05-0700"
	BasePremiumURL        = "https://automute.us/premium?guild="
	CaptureDownloadURL    = "https://capture.automute.us"
	DefaultMaxActiveGames = 150
)

// All is all slash commands for the bot, ordered to match the README
var All = []*discordgo.ApplicationCommand{
	&Help,
	&New,
	&Refresh,
	&Pause,
	&End,
	&Link,
	&Unlink,
	&Privacy,
	&Info,
	&Map,
	&Stats,
	&Premium,
}

func getCommand(cmd string) *discordgo.ApplicationCommand {
	for _, v := range All {
		if v.Name == cmd {
			return v
		}
	}
	return nil
}

func localizeCommandDescription(cmd *discordgo.ApplicationCommand, sett *settings.GuildSettings) string {
	return sett.LocalizeMessage(&i18n.Message{
		ID:    fmt.Sprintf("commands.%s.description", cmd.Name),
		Other: cmd.Description,
	})
}

// TODO supplement these embed with more detail than just the command description
func constructEmbedForCommand(
	cmd *discordgo.ApplicationCommand,
	sett *settings.GuildSettings,
) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       fmt.Sprintf("`/%s`", cmd.Name),
		Description: localizeCommandDescription(cmd, sett),
		Timestamp:   "",
		Color:       15844367, // GOLD
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      nil,
	}
}
