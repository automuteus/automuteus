package command

import "github.com/bwmarrin/discordgo"

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
			Name:        "color",
			Description: "In-game color",
			Required:    true,
		},
	},
}
