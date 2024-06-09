package command

import (
	"fmt"
	"github.com/j0nas500/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"time"
)

var Info = discordgo.ApplicationCommand{
	Name:        "info",
	Description: "AutoMuteUs info",
}

type BotInfo struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	ShardID     int    `json:"shardID"`
	ShardCount  int    `json:"shardCount"`
	TotalGuilds int64  `json:"totalGuilds"`
	ActiveGames int64  `json:"activeGames"`
	TotalUsers  int64  `json:"totalUsers"`
	TotalGames  int64  `json:"totalGames"`
}

func InfoResponse(info BotInfo, guildID string, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	embed := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.info.title",
			Other: "Bot Info",
		}),
		Description: "",
		Timestamp:   time.Now().Format(ISO8601),
		Color:       2067276, // DARK GREEN
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Footer: &discordgo.MessageEmbedFooter{
			Text: sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.info.footer",
				Other: "{{.Version}}-{{.Commit}} | Shard {{.ID}}/{{.Num}}",
			},
				map[string]interface{}{
					"Version": info.Version,
					"Commit":  info.Commit,
					"ID":      fmt.Sprintf("%d", info.ShardID),
					"Num":     fmt.Sprintf("%d", info.ShardCount),
				}),
			IconURL:      "",
			ProxyIconURL: "",
		},
	}

	fields := make([]*discordgo.MessageEmbedField, 12)
	var version = info.Version
	if version == "" {
		version = "Unknown"
	}
	fields[0] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.info.version",
			Other: "Version",
		}),
		Value:  version,
		Inline: true,
	}
	fields[1] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.info.library",
			Other: "Library",
		}),
		Value:  "discordgo",
		Inline: true,
	}
	fields[2] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.info.creator",
			Other: "Creator",
		}),
		Value:  "Soup#4222",
		Inline: true,
	}
	fields[3] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.info.guilds",
			Other: "Guilds",
		}),
		Value:  fmt.Sprintf("%d", info.TotalGuilds),
		Inline: true,
	}
	fields[4] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.info.activegames",
			Other: "Active Games",
		}),
		Value:  fmt.Sprintf("%d", info.ActiveGames),
		Inline: true,
	}
	fields[5] = &discordgo.MessageEmbedField{
		Name:   "\u200B",
		Value:  "\u200B",
		Inline: true,
	}
	fields[6] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.info.totalgames",
			Other: "Total Games",
		}),
		Value:  fmt.Sprintf("%d", info.TotalGames),
		Inline: true,
	}
	fields[7] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.info.totalusers",
			Other: "Total Users",
		}),
		Value:  fmt.Sprintf("%d", info.TotalUsers),
		Inline: true,
	}
	fields[8] = &discordgo.MessageEmbedField{
		Name:   "\u200B",
		Value:  "\u200B",
		Inline: true,
	}
	fields[9] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.info.website",
			Other: "Website",
		}),
		Value:  "[automute.us](https://automute.us)",
		Inline: true,
	}
	fields[10] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.info.invite",
			Other: "Invite",
		}),
		Value:  "[add.automute.us](https://add.automute.us)",
		Inline: true,
	}
	fields[11] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.info.premium",
			Other: "Premium",
		}),
		Value:  "[PayPal](" + BasePremiumURL + guildID + ")",
		Inline: true,
	}

	embed.Fields = fields
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{&embed},
		},
	}
}
