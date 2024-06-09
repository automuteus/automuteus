package command

import (
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"strings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type LinkStatus int

const (
	LinkSuccess LinkStatus = iota
	LinkNoPlayer
	LinkNoGameData
)

var Link = discordgo.ApplicationCommand{
	Name:        "link",
	Description: "Link a Discord User to their in-game color",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionUser,
			Name:        "user",
			Description: "User to link",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "vanillacolor",
			Description: "Vanilla In-game color",
			Required:    false,
			Choices:     colorsVanillaToCommandChoices(),
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "torcolor",
			Description: "Tor In-game color",
			Required:    false,
			Choices:     colorsTorToCommandChoices(),
		},
	},
}

func GetLinkParams(s *discordgo.Session, options []*discordgo.ApplicationCommandInteractionDataOption) (string, string) {
	if len(options) < 2 {
		return options[0].UserValue(s).ID, ""
	}
	return options[0].UserValue(s).ID, strings.ReplaceAll(strings.ToLower(options[1].StringValue()), " ", "")
}

func LinkResponse(status LinkStatus, userID, color string, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var content string
	switch status {
	case LinkSuccess:
		content = sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.link.success",
			Other: "Successfully linked {{.UserMention}} to an in-game player with the color: `{{.Color}}`",
		}, map[string]interface{}{
			"UserMention": discord.MentionByUserID(userID),
			"Color":       color,
		})
	case LinkNoPlayer:
		content = sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.link.noplayer",
			Other: "No player in the current game was detected for {{.UserMention}}",
		}, map[string]interface{}{
			"UserMention": discord.MentionByUserID(userID),
		})
	case LinkNoGameData:
		content = sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.link.nogamedata",
			Other: "No game data found for the color `{{.Color}}`",
		}, map[string]interface{}{
			"Color": color,
		})
	}

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   1 << 6,
			Content: content,
		},
	}
}
