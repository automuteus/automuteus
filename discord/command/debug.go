package command

import (
	"fmt"
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	UserCache = "user-cache"
	User      = "user"
	GameState = "game-state"
)

var Debug = discordgo.ApplicationCommand{
	Name:        "debug",
	Description: "View and clear debug information for AutoMuteUs",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Name:        UserCache,
			Description: "User cache",
			Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        View,
					Description: "View User Cache",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        User,
							Description: "User whose cache you want to view",
							Type:        discordgo.ApplicationCommandOptionUser,
							Required:    false,
						},
					},
				},
				{
					Name:        Clear,
					Description: "Clear User Cache",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        User,
							Description: "User whose cache should be cleared. Defaults to self.",
							Type:        discordgo.ApplicationCommandOptionUser,
							Required:    false,
						},
					},
				},
			},
		},
		{
			Name:        GameState,
			Description: "Game state",
			Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        View,
					Description: "View Game State",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
				},
			},
		},
		{
			Name:        UnmuteAll,
			Description: "Unmute all players",
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			// TODO sub-arguments to unmute specific players?
		},
	},
}

func GetDebugParams(s *discordgo.Session, userID string, options []*discordgo.ApplicationCommandInteractionDataOption) (string, string, string) {
	switch options[0].Name {
	case UserCache:
		if len(options[0].Options[0].Options) > 0 {
			userID = options[0].Options[0].Options[0].UserValue(s).ID
		}
		return options[0].Options[0].Name, UserCache, userID
	case GameState:
		return options[0].Options[0].Name, GameState, ""
	case UnmuteAll:
		return UnmuteAll, UnmuteAll, ""
	}
	return "", "", ""
}

func DebugResponse(operationType string, cached map[string]interface{}, stateBytes []byte, id string, err error, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var content string
	switch operationType {
	case View:
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

	case Clear:
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
