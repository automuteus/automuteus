package discord

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func (bot *Bot) UserStatsEmbed(userID string, sett *storage.GuildSettings) *discordgo.MessageEmbed {
	gamesPlayed := bot.PostgresInterface.NumGamesPlayedByUser(userID)
	colorRankings := bot.PostgresInterface.ColorRankingForPlayer(userID)

	fields := []*discordgo.MessageEmbedField{}
	fields = append(fields, &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.GamesPlayed",
			Other: "Games Played",
		}),
		Value:  fmt.Sprintf("%d", gamesPlayed),
		Inline: false,
	})
	if len(colorRankings) > 0 {
		elem := colorRankings[0]
		emoji := bot.StatusEmojis[true][elem.Mode]
		fields = append(fields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.userStatsEmbed.FavoriteColor",
				Other: "Favorite Color",
			}),
			Value:  fmt.Sprintf("%s (%d games)", emoji.FormatForInline(), elem.Count),
			Inline: false,
		})
	}

	var embed = discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.Title",
			Other: "User Stats",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.Desc",
			Other: "User stats for {{.User}}",
		}, map[string]interface{}{
			"User": "<@!" + userID + ">",
		}),
		Timestamp: "",
		Color:     3066993, //GREEN
		Image:     nil,
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
		Fields:    fields,
	}
	return &embed
}
