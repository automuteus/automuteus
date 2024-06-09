package command

import (
	"github.com/j0nas500/automuteus/v8/pkg/game"
	"github.com/bwmarrin/discordgo"
	"os"
)

var Map = discordgo.ApplicationCommand{
	Name:        "map",
	Description: "View Among Us game maps",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "map_name",
			Description: "Map to display",
			Choices:     mapsToCommandChoices(),
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "detailed",
			Description: "View detailed map",
			Required:    false,
		},
	},
}

func GetMapParams(options []*discordgo.ApplicationCommandInteractionDataOption) (_ game.PlayMap, detailed bool) {
	if len(options) > 1 {
		detailed = options[1].BoolValue()
	}
	return game.PlayMap(options[0].IntValue()), detailed
}

func MapResponse(mapType game.PlayMap, detailed bool) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: game.FormMapUrl(os.Getenv("BASE_MAP_URL"), mapType, detailed),
		},
	}
}
