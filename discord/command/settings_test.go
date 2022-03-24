package command

import (
	"github.com/bwmarrin/discordgo"
	"testing"
)

func TestGetSettingsParams(t *testing.T) {
	options := []*discordgo.ApplicationCommandInteractionDataOption{
		&discordgo.ApplicationCommandInteractionDataOption{
			Name: "admin-user-ids",
			Type: discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				&discordgo.ApplicationCommandInteractionDataOption{
					Name: "user",
					Type: discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						&discordgo.ApplicationCommandInteractionDataOption{
							Name:  "user",
							Type:  discordgo.ApplicationCommandOptionUser,
							Value: "1234",
						},
					},
				},
			},
		},
	}
	settingName, args := GetSettingsParams(nil, options)
	if settingName != "admin-user-ids" {
		t.Fail()
	}
	if args[0] != "<@1234>" {
		t.Fail()
	}
}
