package command

import (
	"github.com/bwmarrin/discordgo"
)

var Help = discordgo.ApplicationCommand{
	Name:        "help",
	Description: "AutoMuteUs help",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "command",
			Description: "Command to view details for",
			Required:    false,
		},
	},
}
