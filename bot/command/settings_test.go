package command

import (
	"github.com/j0nas500/automuteus-tor/v8/bot/setting"
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
	settingName, args := GetSettingsParams(options)
	if settingName != "admin-user-ids" {
		t.Fail()
	}
	if args[0] != "<@1234>" {
		t.Fail()
	}

	options = []*discordgo.ApplicationCommandInteractionDataOption{
		&discordgo.ApplicationCommandInteractionDataOption{
			Name: "admin-user-ids",
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
	if settingName != "admin-user-ids" {
		t.Fail()
	}
	if args[0] != setting.Clear {
		t.Fail()
	}
}

// TODO construct a test to validate complex settings behavior, like voice rules or delays
