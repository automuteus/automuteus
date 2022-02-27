package command

import (
	"github.com/bwmarrin/discordgo"
)

const (
	UserStats  = "user"
	GuildStats = "guild"
	MatchStats = "match"
)

// TODO stats commands to be supported:
// user view (optional userid param; defaults to current user)
// user reset (requires admin, should confirm)
// guild view
// guild reset (requires admin, should confirm)
// match view (requires match ID code)

var Stats = discordgo.ApplicationCommand{
	Name:        "stats",
	Description: "View stats from games played with AutoMuteUs",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "stats_type",
			Description: "Stats type to display",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{
					Name:  UserStats,
					Value: UserStats,
				},
				{
					Name:  GuildStats,
					Value: GuildStats,
				},
				{
					Name:  MatchStats,
					Value: MatchStats,
				},
			},
			Required: true,
		},
	},
}
