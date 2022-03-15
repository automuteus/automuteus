package command

import (
	"github.com/bwmarrin/discordgo"
)

var Pause = discordgo.ApplicationCommand{
	Name:        "pause",
	Description: "Pause the current game",
}
