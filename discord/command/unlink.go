package command

import "github.com/bwmarrin/discordgo"

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
