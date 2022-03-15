package command

import (
	"fmt"
	"github.com/automuteus/automuteus/discord/setting"
	"github.com/bwmarrin/discordgo"
	"log"
)

var Settings = discordgo.ApplicationCommand{
	Name:        "settings",
	Description: "View or change AutoMuteUs settings",
	Options:     settingsToCommandOptions(),
}

func GetSettingsParams(s *discordgo.Session, guildID string, options []*discordgo.ApplicationCommandInteractionDataOption) (setting.Name, string) {
	sett := setting.GetSettingByName(options[0].Name)
	switch sett.ArgumentType {
	case discordgo.ApplicationCommandOptionString:
		if len(options[0].Options) > 0 {
			return sett.Name, options[0].Options[0].StringValue()
		} else {
			return sett.Name, View
		}
	case discordgo.ApplicationCommandOptionBoolean:
		if len(options[0].Options) > 0 {
			return sett.Name, fmt.Sprintf("%t", options[0].Options[0].BoolValue())
		} else {
			return sett.Name, View
		}
	case discordgo.ApplicationCommandOptionUser:
		if len(options[0].Options) > 0 {
			return sett.Name, options[0].Options[0].UserValue(s).Mention()
		} else {
			return sett.Name, View
		}

	case discordgo.ApplicationCommandOptionRole:
		if len(options[0].Options) > 0 {
			return sett.Name, options[0].Options[0].RoleValue(s, guildID).Mention()
		} else {
			return sett.Name, View
		}
	case 0:
		return sett.Name, ""
	}
	return "", ""
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
		if sett.ArgumentType != 0 {
			options = []*discordgo.ApplicationCommandOption{
				{
					Name:        sett.ArgumentName,
					Description: "Argument for setting",
					Type:        sett.ArgumentType,
					Required:    false,
				},
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
