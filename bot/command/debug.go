package command

import (
	"bytes"
	"fmt"
	"github.com/j0nas500/automuteus-tor/v8/bot/setting"
	"github.com/j0nas500/automuteus/v8/pkg/discord"
	"github.com/j0nas500/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	User      = "user"
	GameState = "game-state"
	UnmuteAll = "unmute-all"
	Unmute    = "unmute"
)

var Debug = discordgo.ApplicationCommand{
	Name:        "debug",
	Description: "View and clear debug information for AutoMuteUs",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:        setting.View,
			Description: "View debug info",
			Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        User,
					Description: "User Cache",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        User,
							Description: "User whose cache you want to view",
							Type:        discordgo.ApplicationCommandOptionUser,
							Required:    true,
						},
					},
				},
				{
					Name:        GameState,
					Description: "Game State",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
			},
		},
		{
			Name:        setting.Clear,
			Description: "Clear debug info",
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        User,
					Description: "User whose cache should be cleared",
					Type:        discordgo.ApplicationCommandOptionUser,
					Required:    true,
				},
			},
		},
		{
			Name:        UnmuteAll,
			Description: "Unmute all players",
			Type:        discordgo.ApplicationCommandOptionSubCommand,
		},
		{
			Name:        Unmute,
			Description: "Unmute myself, or a specific user",
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        User,
					Description: "User who should be unmuted/undeafened",
					Type:        discordgo.ApplicationCommandOptionUser,
					Required:    false,
				},
			},
		},
	},
}

func GetDebugParams(s *discordgo.Session, userID string, options []*discordgo.ApplicationCommandInteractionDataOption) (action string, opType string, _ string) {
	action = options[0].Name
	if len(options[0].Options) > 0 {
		opType = options[0].Options[0].Name
	}
	switch action {
	case setting.View:
		if len(options[0].Options[0].Options) > 0 {
			userID = options[0].Options[0].Options[0].UserValue(s).ID
		}
	case setting.Clear:
		fallthrough
	case Unmute:
		if len(options[0].Options) > 0 {
			userID = options[0].Options[0].UserValue(s).ID
		}
	}
	return action, opType, userID
}

func DebugResponse(operationType string, cached map[string]interface{}, stateBytes []byte, id string, err error, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var content string
	var files []*discordgo.File
	switch operationType {
	case setting.View:
		if err != nil {
			content = sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.debug.view.error",
				Other: "Encountered an error trying to view debug information: {{.Error}}",
			}, map[string]interface{}{
				"Error": err.Error(),
			})
		} else {
			if cached != nil {
				if len(cached) == 0 {
					content = sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.debug.view.user.empty",
						Other: "I don't have any saved usernames for {{.User}}",
					}, map[string]interface{}{
						"User": discord.MentionByUserID(id),
					})
				} else {
					str := ""
					for i := range cached {
						str += i + "\n"
					}
					content = sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.debug.view.user.success",
						Other: "I have the following cached usernames for {{.User}}:\n```\n{{.Cached}}\n```",
					}, map[string]interface{}{
						"User":   discord.MentionByUserID(id),
						"Cached": str,
					})
				}
			} else if stateBytes != nil {
				// if the contents are too long, when including the ```JSON``` formatting characters
				if len(stateBytes) > 1988 {
					files = []*discordgo.File{
						{
							Name:        "game-state.json",
							ContentType: "application/json",
							Reader:      bytes.NewReader(stateBytes),
						},
					}
					content = sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.download.file.success",
						Other: "Here's that file for you!",
					})
				} else {
					content = fmt.Sprintf("```JSON\n%s\n```", stateBytes)
				}
			}
		}

	case setting.Clear:
		if err != nil {
			content = sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.debug.clear.error",
				Other: "Encountered an error trying to clear debug information: {{.Error}}",
			}, map[string]interface{}{
				"Error": err.Error(),
			})
		} else {
			content = sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.debug.clear.user.success",
				Other: "Successfully cleared cached usernames for {{.User}}",
			}, map[string]interface{}{
				"User": discord.MentionByUserID(id),
			})
		}
	case Unmute:
		if err != nil {
			content = sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.debug.unmute.error",
				Other: "A game is active in this channel, so only admins can unmute. Please try in another voice channel",
			}, map[string]interface{}{
				"Error": err.Error(),
			})
		}
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   1 << 6,
			Content: content,
			Files:   files,
		},
	}
}
