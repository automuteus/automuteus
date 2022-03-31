package command

import (
	"fmt"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	ISO8601               = "2006-01-02T15:04:05-0700"
	BasePremiumURL        = "https://automute.us/premium?guild="
	CaptureDownloadURL    = "https://capture.automute.us"
	DefaultMaxActiveGames = 150
	UnmuteAll             = "unmute-all"
)

// All is all slash commands for the bot, ordered to match the README
var All = []*discordgo.ApplicationCommand{
	&Help,
	&New,
	&Refresh,
	&Pause,
	&End,
	&Link,
	&Unlink,
	&Settings,
	&Privacy,
	&Info,
	&Map,
	&Stats,
	&Premium,
	&Debug,
}

func DeadlockGameStateResponse(command string, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: 1 << 6,
			Content: sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.deadlock",
				Other: "I wasn't able to obtain the game state for your {{.Command}} command. Please try again.",
			}, map[string]interface{}{
				"Command": command,
			}),
		},
	}
}

func InsufficientPermissionsResponse(sett *settings.GuildSettings) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: 1 << 6,
			Content: sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.no_permissions",
				Other: "Sorry, you don't have the required permissions to issue that command.",
			}),
		},
	}
}

func getCommand(cmd string) *discordgo.ApplicationCommand {
	for _, v := range All {
		if v.Name == cmd {
			return v
		}
	}
	return nil
}

func localizeCommandDescription(cmd *discordgo.ApplicationCommand, sett *settings.GuildSettings) string {
	return sett.LocalizeMessage(&i18n.Message{
		ID:    fmt.Sprintf("commands.%s.description", cmd.Name),
		Other: cmd.Description,
	})
}

// TODO supplement these embed with more detail than just the command description
func constructEmbedForCommand(
	cmd *discordgo.ApplicationCommand,
	sett *settings.GuildSettings,
) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       fmt.Sprintf("`/%s`", cmd.Name),
		Description: localizeCommandDescription(cmd, sett),
		Timestamp:   "",
		Color:       15844367, // GOLD
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      nil,
	}
}

func colorsToCommandChoices() []*discordgo.ApplicationCommandOptionChoice {
	var choices []*discordgo.ApplicationCommandOptionChoice
	for color := range game.ColorStrings {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  color,
			Value: color,
		})
	}
	return choices
}

func mapsToCommandChoices() []*discordgo.ApplicationCommandOptionChoice {
	var choices []*discordgo.ApplicationCommandOptionChoice
	for mapValue, mapName := range game.MapNames {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  mapName,
			Value: mapValue,
		})
	}
	return choices
}

func NoGameResponse(sett *settings.GuildSettings) *discordgo.InteractionResponse {
	return PrivateResponse(
		sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.error.nogame",
			Other: "No game is currently running.",
		}))
}

func PrivateResponse(content string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   1 << 6,
			Content: content,
		},
	}
}

func PrivateErrorResponse(cmd string, err error, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: 1 << 6,
			Content: sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.error",
				Other: "Error executing `{{.Command}}`: `{{.Error}}`",
			}, map[string]interface{}{
				"Command": cmd,
				"Error":   err.Error(),
			}),
		},
	}
}

func ReinviteMeResponse(sett *settings.GuildSettings) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.error.reinvite",
				Other: "I'm missing the required permissions to function properly in this server or channel!\n\nYou may need to reinvite me [here](https://add.automute.us)",
			}),
		},
	}
}
