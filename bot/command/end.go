package command

import (
	"github.com/bwmarrin/discordgo"
)

var End = discordgo.ApplicationCommand{
	Name:        "end",
	Description: "End a game",
}
