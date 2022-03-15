package command

import (
	"github.com/bwmarrin/discordgo"
)

var Refresh = discordgo.ApplicationCommand{
	Name:        "refresh",
	Description: "Refresh the game message",
}
