package command

import (
	"github.com/automuteus/automuteus/discord/setting"
	"github.com/bwmarrin/discordgo"
	"log"
)

var Settings = discordgo.ApplicationCommand{
	Name:        "settings",
	Description: "View or change AutoMuteUs settings",
	Options:     settingsToCommandOptions(),
}

func GetSettingsParams(s *discordgo.Session, options []*discordgo.ApplicationCommandInteractionDataOption) (string, []string) {
	sett := setting.GetSettingByName(options[0].Name)
	args := make([]string, len(options[0].Options))
	for i, v := range options[0].Options {
		arg := sett.Arguments[i]
		if arg.Type == discordgo.ApplicationCommandOptionSubCommand {
			args[i] = setting.ToString(v.Options[0], s)
		} else {
			// TODO this should be more flexible, not just string arguments. But requires all the tests to change, etc
			args[i] = setting.ToString(v, s)
		}
	}

	return sett.Name, args
}

func SettingsResponse(m interface{}) *discordgo.InteractionResponse {
	content := ""
	var embeds []*discordgo.MessageEmbed
	switch msg := m.(type) {
	case string:
		content = msg
	case discordgo.MessageEmbed:
		embed := msg
		embeds = append(embeds, &embed)
	case *discordgo.MessageEmbed:
		embeds = append(embeds, msg)
	case nil:
		// do nothing
	default:
		log.Printf("Incapable of processing sendMessage of type: %T", msg)
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			//Flags:   0,
			Content: content,
			Embeds:  embeds,
		},
	}
}

func settingsToCommandOptions() []*discordgo.ApplicationCommandOption {
	var choices []*discordgo.ApplicationCommandOption
	for _, sett := range setting.AllSettings {
		optionType := discordgo.ApplicationCommandOptionSubCommand

		// if arguments are subcommands, then make this one a group
		if len(sett.Arguments) > 0 && sett.Arguments[0].Type == discordgo.ApplicationCommandOptionSubCommand {
			optionType = discordgo.ApplicationCommandOptionSubCommandGroup
		}
		choices = append(choices, &discordgo.ApplicationCommandOption{
			Name:        sett.Name,
			Description: sett.ShortDesc,
			Type:        optionType,
			Options:     sett.Arguments,
		})
	}
	return choices
}
