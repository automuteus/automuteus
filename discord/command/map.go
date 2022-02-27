package command

import (
	"fmt"
	"github.com/automuteus/utils/pkg/game"
	"github.com/bwmarrin/discordgo"
	"os"
)

const (
	DefaultBaseUrl = "https://github.com/automuteus/automuteus/blob/master/assets/maps/"
)

var Map = discordgo.ApplicationCommand{
	Name:        "map",
	Description: "View Among Us maps. `skeld`, `mira`, `polus`, or `airship`",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "map_name",
			Description: "Map to display",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{
					Name:  game.MapNames[game.SKELD],
					Value: game.SKELD,
				},
				{
					Name:  game.MapNames[game.MIRA],
					Value: game.MIRA,
				},
				{
					Name:  game.MapNames[game.POLUS],
					Value: game.POLUS,
				},
				{
					Name:  game.MapNames[game.AIRSHIP],
					Value: game.AIRSHIP,
				},
			},
			Required: true,
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
			Content: FormMapUrl(os.Getenv("BASE_MAP_URL"), mapType, detailed),
		},
	}
}

func FormMapUrl(baseUrl string, mapType game.PlayMap, detailed bool) string {
	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}
	// TODO move to utils
	mapString := ""
	for i, v := range game.NameToPlayMap {
		if v == int32(mapType) {
			mapString = i
		}
	}
	if detailed {
		return fmt.Sprintf("%s%s_detailed.png?raw=true", baseUrl, mapString)
	}
	return fmt.Sprintf("%s%s.png?raw=true", baseUrl, mapString)
}
