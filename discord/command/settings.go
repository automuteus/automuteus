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

func GetSettingsParams(s *discordgo.Session, options []*discordgo.ApplicationCommandInteractionDataOption) (setting.Name, []string, error) {
	sett := setting.GetSettingByName(options[0].Name)
	args := make([]string, len(options[0].Options))
	for i, v := range options[0].Options {
		arg := sett.Arguments[i]
		// TODO this should be more flexible, not just string arguments. But requires all the tests to change, etc
		args[i] = arg.AsString(v, s)

		// ensure that the option fulfills the constraints specified
		err := arg.Validate(v)
		if err != nil {
			return sett.Name, args, err
		}
	}

	return sett.Name, args, nil
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
		var options []*discordgo.ApplicationCommandOption
		for _, opt := range sett.Arguments {
			if opt.OptionType != 0 {
				options = append(options, &discordgo.ApplicationCommandOption{
					Name:        opt.Name,
					Description: opt.Name,
					Type:        opt.OptionType,
					Required:    opt.Required,
					Choices:     opt.Choices(),
				},
				)
			}
		}
		choices = append(choices, &discordgo.ApplicationCommandOption{
			Name:        string(sett.Name),
			Description: sett.ShortDesc,
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Options:     options,
		})
	}
	return choices
}
