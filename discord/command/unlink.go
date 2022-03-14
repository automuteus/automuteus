package command

import (
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type UnlinkStatus int

const (
	UnlinkSuccess UnlinkStatus = iota
	UnlinkNoPlayer
)

var Unlink = discordgo.ApplicationCommand{
	Name:        "unlink",
	Description: "Unlink a Discord User from their in-game color",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionUser,
			Name:        "user",
			Description: "User to unlink",
			Required:    true,
		},
	},
}

func GetUnlinkParams(s *discordgo.Session, options []*discordgo.ApplicationCommandInteractionDataOption) string {
	return options[0].UserValue(s).ID
}

func UnlinkResponse(status UnlinkStatus, userID string, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var content string
	switch status {
	case UnlinkSuccess:
		content = sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.unlink.success",
			Other: "Successfully unlinked {{.UserMention}}",
		}, map[string]interface{}{
			"UserMention": discord.MentionByUserID(userID),
		})
	case UnlinkNoPlayer:
		content = sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.unlink.noplayer",
			Other: "No player in the current game was detected for {{.UserMention}}",
		}, map[string]interface{}{
			"UserMention": discord.MentionByUserID(userID),
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
