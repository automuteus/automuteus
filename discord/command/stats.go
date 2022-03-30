package command

import (
	"github.com/automuteus/automuteus/discord/setting"
	"github.com/bwmarrin/discordgo"
)

const (
	Match = "match"
	Guild = "guild"
)

var Stats = discordgo.ApplicationCommand{
	Name:        "stats",
	Description: "View or clear stats from games played with AutoMuteUs",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:        setting.View,
			Description: "View stats",
			Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        User,
					Description: "User stats",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        User,
							Description: "User whose stats you want to view",
							Type:        discordgo.ApplicationCommandOptionUser,
							Required:    true,
						},
					},
				},
				{
					Name:        Match,
					Description: "Match stats",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        Match,
							Description: "Match ID whose stats you want to view",
							Type:        discordgo.ApplicationCommandOptionString,
							Required:    true,
						},
					},
				},
				{
					Name:        Guild,
					Description: "View this guild's stats",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
			},
		},
		{
			Name:        setting.Clear,
			Description: "Clear stats",
			Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        User,
					Description: "User stats",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        User,
							Description: "User whose stats you want to clear",
							Type:        discordgo.ApplicationCommandOptionUser,
							Required:    true,
						},
					},
				},
				{
					Name:        Guild,
					Description: "Reset this guild's stats",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
			},
		},
	},
}

func GetStatsParams(s *discordgo.Session, guildID string, options []*discordgo.ApplicationCommandInteractionDataOption) (action string, opType string, id string) {
	action = options[0].Name
	opType = options[0].Options[0].Name
	switch opType {
	case User:
		id = options[0].Options[0].Options[0].UserValue(s).ID
	case Guild:
		id = guildID
	case Match:
		id = options[0].Options[0].Options[0].StringValue()
	}
	return action, opType, id
}
