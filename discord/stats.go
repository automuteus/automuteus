package discord

import (
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
)

func (bot *Bot) UserStatsEmbed(userID, guildID string, sett *storage.GuildSettings, premium string) *discordgo.MessageEmbed {
	gamesPlayed := bot.PostgresInterface.NumGamesPlayedByUser(userID)
	gamesPlayedServer := bot.PostgresInterface.NumGamesPlayedByUserOnServer(userID, guildID)

	avatarUrl := ""
	mem, err := bot.PrimarySession.GuildMember(guildID, userID)
	if err != nil {
		log.Println(err)
	} else if mem.User != nil {
		avatarUrl = mem.User.AvatarURL("")
	}

	fields := make([]*discordgo.MessageEmbedField, 2)
	fields[0] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.GamesPlayed",
			Other: "Total Games Played",
		}),
		Value:  fmt.Sprintf("%d", gamesPlayed),
		Inline: true,
	}
	fields[1] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.GamesPlayedOnServer",
			Other: "On This Server",
		}),
		Value:  fmt.Sprintf("%d", gamesPlayedServer),
		Inline: false,
	}

	extraDesc := sett.LocalizeMessage(&i18n.Message{
		ID:    "responses.userStatsEmbed.NoPremium",
		Other: "Detailed stats are only available for AutoMuteUs Premium users; type `{{.CommandPrefix}} premium` to learn more",
	}, map[string]interface{}{
		"CommandPrefix": sett.CommandPrefix,
	})

	if premium != "Free" {
		extraDesc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.Premium",
			Other: "Showing additional Premium Stats!",
		})
		colorRankings := bot.PostgresInterface.ColorRankingForPlayer(userID)
		if len(colorRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i := 0; i < len(colorRankings); i++ {
				elem := colorRankings[i]
				emoji := bot.StatusEmojis[true][elem.Mode]
				buf.WriteString(fmt.Sprintf("%s | %.0f%%", emoji.FormatForInline(), 100.0*float64(elem.Count)/float64(gamesPlayed)))
				if i < len(colorRankings)-1 {
					buf.WriteByte('\n')
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.FavoriteColors",
					Other: "Favorite Colors",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}
		nameRankings := bot.PostgresInterface.NamesRanking(userID)
		if len(nameRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i := 0; i < len(nameRankings); i++ {
				elem := nameRankings[i]
				buf.WriteString(fmt.Sprintf("%s | %.0f%%", elem.Mode, 100.0*float64(elem.Count)/float64(gamesPlayed)))
				if i < len(nameRankings)-1 {
					buf.WriteByte('\n')
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.FavoriteNames",
					Other: "Favorite Names",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}
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
		}) + "\n\n" + extraDesc,
		Timestamp: "",
		Color:     3066993, //GREEN
		Image:     nil,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL:      avatarUrl,
			ProxyURL: "",
			Width:    0,
			Height:   0,
		},
		Video:    nil,
		Provider: nil,
		Author:   nil,
		Fields:   fields,
	}
	return &embed
}

func (bot *Bot) GuildStatsEmbed(guildID string, sett *storage.GuildSettings, premium string) *discordgo.MessageEmbed {
	gname := ""
	avatarUrl := ""
	g, err := bot.PrimarySession.Guild(guildID)

	if err != nil {
		log.Println(err)
		gname = guildID
	} else {
		gname = g.Name
		avatarUrl = g.IconURL()
	}

	gamesPlayed := bot.PostgresInterface.NumGamesPlayedOnGuild(guildID)

	fields := make([]*discordgo.MessageEmbedField, 1)
	fields[0] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.guildStatsEmbed.GamesPlayed",
			Other: "Total Games Played",
		}),
		Value:  fmt.Sprintf("%d", gamesPlayed),
		Inline: true,
	}

	extraDesc := sett.LocalizeMessage(&i18n.Message{
		ID:    "responses.guildStatsEmbed.NoPremium",
		Other: "Detailed stats are only available for AutoMuteUs Premium users; type `{{.CommandPrefix}} premium` to learn more",
	}, map[string]interface{}{
		"CommandPrefix": sett.CommandPrefix,
	})

	if premium != "Free" {
		extraDesc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.guildStatsEmbed.Premium",
			Other: "Showing additional Premium Stats!",
		})
	}

	var embed = discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.guildStatsEmbed.Title",
			Other: "Guild Stats",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.guildStatsEmbed.Desc",
			Other: "Guild stats for {{.GuildName}}",
		}, map[string]interface{}{
			"GuildName": gname,
		}) + "\n\n" + extraDesc,
		Timestamp: "",
		Color:     3066993, //GREEN
		Image:     nil,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL:      avatarUrl,
			ProxyURL: "",
			Width:    0,
			Height:   0,
		},
		Video:    nil,
		Provider: nil,
		Author:   nil,
		Fields:   fields,
	}
	return &embed
}
