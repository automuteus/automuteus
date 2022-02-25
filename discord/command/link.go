package command

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
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
		// TODO use discordgo.ApplicationCommandOptionChoice instead of arbitrary string args
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "color",
			Description: "In-game color",
			Required:    true,
		},
	},
}

func GetLinkParams(s *discordgo.Session, options []*discordgo.ApplicationCommandInteractionDataOption) (string, string) {
	return options[0].UserValue(s).ID, strings.ReplaceAll(strings.ToLower(options[1].StringValue()), " ", "")
}

func LinkResponse(status LinkStatus, userID, colorOrName string, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var content string
	switch status {
	case LinkSuccess:
		content = sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.link.success",
			Other: "Successfully linked {{.UserID} to an in-game player matching {{.ColorOrName}}",
		}, map[string]interface{}{
			"UserID":      userID,
			"ColorOrName": colorOrName,
		})
	case LinkNoPlayer:
		content = sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.link.noplayer",
			Other: "No player in the current game was detected for {{.UserID}}",
		}, map[string]interface{}{
			"UserID": userID,
		})
	case LinkNoGameData:
		content = sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.link.nogamedata",
			Other: "No game data found for color/name `{{.ColorOrName}}",
		}, map[string]interface{}{
			"ColorOrName": colorOrName,
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
