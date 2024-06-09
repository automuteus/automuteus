package command

import (
	"github.com/j0nas500/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"time"
)

const (
	Category   = "category"
	Users      = "users"
	UsersGames = "users_games"
	Games      = "games"
	GameEvents = "game_events"
)

var Download = discordgo.ApplicationCommand{
	Name:        "download",
	Description: "Download AutoMuteUs data",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:        Category,
			Description: "Data to download",
			Type:        discordgo.ApplicationCommandOptionString,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{
					Name:  Guild,
					Value: Guild,
				},
				{
					Name:  Users,
					Value: Users,
				},
				{
					Name:  UsersGames,
					Value: UsersGames,
				},
				{
					Name:  Games,
					Value: Games,
				},
				{
					Name:  GameEvents,
					Value: GameEvents,
				},
			},
			Required: true,
		},
	},
}

func GetDownloadParams(options []*discordgo.ApplicationCommandInteractionDataOption) string {
	return options[0].StringValue()
}

func DownloadCooldownResponse(sett *settings.GuildSettings, category string, duration time.Duration) *discordgo.InteractionResponse {
	// report with minute-level precision
	durationStr := duration.Truncate(time.Minute).String()
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: 1 << 6,
			Content: sett.LocalizeMessage(&i18n.Message{
				ID: "commands.download.cooldown",
				Other: "Sorry, `{{.Category}}` data can only downloaded once every 24 hours!\n\n" +
					"Please wait {{.Duration}} and then try again",
			}, map[string]interface{}{
				"Category": category,
				// strip the "0s" off the end
				"Duration": durationStr[:len(durationStr)-2],
			}),
		},
	}
}

func DownloadNotGoldResponse(sett *settings.GuildSettings) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: 1 << 6,
			Content: sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.download.nogold",
				Other: "Downloading AutoMuteUs data is reserved for Gold subscribers only!",
			}),
		},
	}
}
