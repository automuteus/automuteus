package command

import (
	"fmt"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"os"
	"strings"
)

const (
	DefaultBaseUrl = "https://github.com/automuteus/automuteus/blob/master/assets/maps/"
)

type MapType string

// Note: these are the exact names of the png files in the Github repository. No Dleks as of now
const (
	Skeld   MapType = "the_skeld"
	Mira            = "mira_hq"
	Polus           = "polus"
	Airship         = "airship"
	NilMap
)

// PlayMapToMapType exists as a function mapping between in-game maps, and the ones we have asset files for on Github
func PlayMapToMapType(mapType game.PlayMap) MapType {
	switch mapType {
	case game.SKELD:
		return Skeld
	case game.MIRA:
		return Mira
	case game.POLUS:
		return Polus
	case game.AIRSHIP:
		return Airship
	}
	return NilMap
}

var Map = discordgo.ApplicationCommand{
	Name:        "map",
	Description: "View Among Us maps. `skeld`, `mira`, `polus`, or `airship`",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "map_name",
			Description: "Map to display",
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

func GetMapParams(options []*discordgo.ApplicationCommandInteractionDataOption) (_ MapType, detailed bool) {
	if len(options) > 1 {
		detailed = options[1].BoolValue()
	}
	switch strings.ToLower(options[0].StringValue()) {
	case "the skeld":
		fallthrough
	case "the_skeld":
		fallthrough
	case "skeld":
		return Skeld, detailed

	case "mira hq":
		fallthrough
	case "mira_hq":
		fallthrough
	case "mirahq":
		fallthrough
	case "mira":
		return Mira, detailed

	case "polus":
		return Polus, detailed

	case "ship":
		fallthrough
	case "air":
		fallthrough
	case "airship":
		return Airship, detailed
	default:
		return NilMap, detailed
	}
}

func MapResponse(mapType MapType, detailed bool, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	if mapType == NilMap {
		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: sett.LocalizeMessage(&i18n.Message{
					ID: "commands.map.unknown",
					Other: "Sorry, I don't understand the map name you provided. " +
						"Please provide `skeld`, `mira`, `polus`, or `airship`",
				}),
			},
		}
	}
	// TODO is there a better interactionresponse than one that just puts the URL in the content? Seems to work fine
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: FormMapUrl(os.Getenv("BASE_MAP_URL"), mapType, detailed),
		},
	}
}

func FormMapUrl(baseUrl string, mapType MapType, detailed bool) string {
	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}
	if detailed {
		return fmt.Sprintf("%s%s_detailed.png?raw=true", baseUrl, mapType)
	}
	return fmt.Sprintf("%s%s.png?raw=true", baseUrl, mapType)
}
