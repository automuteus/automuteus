package command

import (
	"fmt"
	"github.com/automuteus/automuteus/discord/setting"
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	User      = "user"
	GameState = "game-state"
)

var Debug = discordgo.ApplicationCommand{
	Name:        "debug",
	Description: "View and clear debug information for AutoMuteUs",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:              setting.View,
			NameLocalizations: map[discordgo.Locale]string{
				// TODO
				// discordgo.Japanese: "",
			},
			Description:              "View debug info",
			DescriptionLocalizations: map[discordgo.Locale]string{
				// TODO
				// discordgo.Japanese: "",
			},
			Type: discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:              User,
					NameLocalizations: map[discordgo.Locale]string{
						// TODO
						// discordgo.Japanese: "",
					},
					Description:              "User Cache",
					DescriptionLocalizations: map[discordgo.Locale]string{
						// TODO
						// discordgo.Japanese: "",
					},
					Type: discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:              User,
							NameLocalizations: map[discordgo.Locale]string{
								// TODO
								// discordgo.Japanese: "",
							},
							Description:              "User whose cache you want to view",
							DescriptionLocalizations: map[discordgo.Locale]string{
								// TODO
								// discordgo.Japanese: "",
							},
							Type:     discordgo.ApplicationCommandOptionUser,
							Required: true,
						},
					},
				},
				{
					Name:              GameState,
					NameLocalizations: map[discordgo.Locale]string{
						// TODO
						// discordgo.Japanese: "",
					},
					Description:              "Game State",
					DescriptionLocalizations: map[discordgo.Locale]string{
						// TODO
						// discordgo.Japanese: "",
					},
					Type: discordgo.ApplicationCommandOptionSubCommand,
				},
			},
		},
		{
			Name:              setting.Clear,
			NameLocalizations: map[discordgo.Locale]string{
				// TODO
				// discordgo.Japanese: "",
			},
			Description:              "Clear debug info",
			DescriptionLocalizations: map[discordgo.Locale]string{
				// TODO
				// discordgo.Japanese: "",
			},
			Type: discordgo.ApplicationCommandOptionSubCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:              User,
					NameLocalizations: map[discordgo.Locale]string{
						// TODO
						// discordgo.Japanese: "",
					},
					Description:              "User whose cache should be cleared",
					DescriptionLocalizations: map[discordgo.Locale]string{
						// TODO
						// discordgo.Japanese: "",
					},
					Type:     discordgo.ApplicationCommandOptionUser,
					Required: true,
				},
			},
		},
		{
			Name:              UnmuteAll,
			NameLocalizations: map[discordgo.Locale]string{
				// TODO
				// discordgo.Japanese: "",
			},
			Description:              "Unmute all players",
			DescriptionLocalizations: map[discordgo.Locale]string{
				// TODO
				// discordgo.Japanese: "",
			},
			Type: discordgo.ApplicationCommandOptionSubCommand,
			// TODO sub-arguments to unmute specific players?
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
		if len(options[0].Options[0].Options) > 0 {
			userID = options[0].Options[0].Options[0].UserValue(s).ID
		}
	}
	return action, opType, userID
}

func DebugResponse(operationType string, cached map[string]interface{}, stateBytes []byte, id string, err error, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var content string
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
				// TODO needs to be multiple messages
				content = fmt.Sprintf("```JSON\n%s\n```", stateBytes)
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
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   1 << 6,
			Content: content,
		},
	}
}
