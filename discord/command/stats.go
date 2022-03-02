package command

import (
	"github.com/bwmarrin/discordgo"
)

const (
	UserStats  = "user"
	GuildStats = "guild"
	MatchStats = "match"
)

var Stats = discordgo.ApplicationCommand{
	Name:        "stats",
	Description: "View or clear stats from games played with AutoMuteUs",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:        UserStats,
			Description: "User stats",
			Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        View,
					Description: "View User Stats",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "user",
							Description: "User whose stats you want to view",
							Type:        discordgo.ApplicationCommandOptionUser,
							Required:    true,
						},
					},
				},
				{
					Name:        Clear,
					Description: "Clear User Stats",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "user",
							Description: "User whose stats should be cleared",
							Type:        discordgo.ApplicationCommandOptionUser,
							Required:    true,
						},
					},
				},
			},
		},
		{
			Name:        GuildStats,
			Description: "Guild stats",
			Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        View,
					Description: "View Current Guild Stats",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
				{
					Name:        Clear,
					Description: "Clear Current Guild Stats",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
			},
		},
		{
			Name:        MatchStats,
			Description: "Match stats",
			Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        View,
					Description: "View Match Stats",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "code",
							Description: "Match Code you wish to view stats for",
							Type:        discordgo.ApplicationCommandOptionString,
							Required:    true,
						},
					},
				},
			},
		},
	},
}

// TODO index checking, this would be a dumb way to cause a crash (how reliable is the Discord API conventions...)
func GetStatsParams(s *discordgo.Session, guildID string, options []*discordgo.ApplicationCommandInteractionDataOption) (string, string, string) {
	switch options[0].Name {
	case UserStats:
		return options[0].Options[0].Name, UserStats, options[0].Options[0].Options[0].UserValue(s).ID
	case GuildStats:
		return options[0].Options[0].Name, GuildStats, guildID
	case MatchStats:
		return options[0].Options[0].Name, MatchStats, options[0].Options[0].Options[0].StringValue()
	}
	return "", "", ""
}
