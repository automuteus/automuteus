package command

import (
	"github.com/automuteus/automuteus/v8/bot/setting"
	"github.com/bwmarrin/discordgo"
	"testing"
)

func TestGetSettingsParams(t *testing.T) {
	options := []*discordgo.ApplicationCommandInteractionDataOption{
		&discordgo.ApplicationCommandInteractionDataOption{
			Name: "operator-roles",
			Type: discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				&discordgo.ApplicationCommandInteractionDataOption{
					Name: "role",
					Type: discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						&discordgo.ApplicationCommandInteractionDataOption{
							Name:  "role",
							Type:  discordgo.ApplicationCommandOptionRole,
							Value: "1234",
						},
					},
				},
			},
		},
	}
	settingName, args := GetSettingsParams(options)
	if settingName != "operator-roles" {
		t.Fail()
	}
	if args[0] != "<@&1234>" {
		t.Fail()
	}

	options = []*discordgo.ApplicationCommandInteractionDataOption{
		&discordgo.ApplicationCommandInteractionDataOption{
			Name: "operator-roles",
			Type: discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				&discordgo.ApplicationCommandInteractionDataOption{
					Name: setting.Clear,
					Type: discordgo.ApplicationCommandOptionSubCommand,
				},
			},
		},
	}
	settingName, args = GetSettingsParams(options)
	if settingName != "operator-roles" {
		t.Fail()
	}
	if args[0] != setting.Clear {
		t.Fail()
	}
}

// TODO construct a test to validate complex settings behavior, like voice rules or delays
