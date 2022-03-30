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
