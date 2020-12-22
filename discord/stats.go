package discord

import (
	"bytes"
	"context"
	"fmt"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/premium"
	"github.com/automuteus/utils/pkg/rediskey"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
	"strconv"
	"strings"
)

const UserLeaderboardCount = 3

const LeaderboardSize = 3

const MinumumGameCountThreshold = 2

func (bot *Bot) UserStatsEmbed(userID, guildID string, sett *storage.GuildSettings, prem premium.Tier) *discordgo.MessageEmbed {
	gamesPlayed := bot.PostgresInterface.NumGamesPlayedByUserOnServer(userID, guildID)
	wins := bot.PostgresInterface.NumWinsOnServer(userID, guildID)

	avatarURL := ""
	mem, err := bot.PrimarySession.GuildMember(guildID, userID)
	if err != nil {
		log.Println(err)
	} else if mem.User != nil {
		avatarURL = mem.User.AvatarURL("")
	}

	fields := make([]*discordgo.MessageEmbedField, 3)
	fields[0] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.GamesPlayed",
			Other: "Games Played",
		}),
		Value:  fmt.Sprintf("%d", gamesPlayed),
		Inline: true,
	}
	fields[1] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.TotalWins",
			Other: "Total Wins",
		}),
		Value:  fmt.Sprintf("%d", wins),
		Inline: true,
	}
	winrate := 0.0
	if gamesPlayed > 0 {
		winrate = 100.0 * (float64(wins) / float64(gamesPlayed))
	}
	fields[2] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.Winrate",
			Other: "Winrate",
		}),
		Value:  fmt.Sprintf("%d/%d | %.0f%%", wins, gamesPlayed, winrate),
		Inline: true,
	}

	extraDesc := sett.LocalizeMessage(&i18n.Message{
		ID:    "responses.userStatsEmbed.NoPremium",
		Other: "Detailed stats are only available for AutoMuteUs Premium users; type `{{.CommandPrefix}} premium` to learn more",
	}, map[string]interface{}{
		"CommandPrefix": sett.CommandPrefix,
	})

	if prem != premium.FreeTier {
		extraDesc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.userStatsEmbed.Premium",
			Other: "Showing additional Premium Stats!\n(Note: stats are still in **BETA**, and will be likely be inaccurate while we work to improve them).",
		})
		colorRankings := bot.PostgresInterface.ColorRankingForPlayerOnServer(userID, guildID)
		if len(colorRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i := 0; i < len(colorRankings) && i < UserLeaderboardCount; i++ {
				elem := colorRankings[i]
				emoji := bot.StatusEmojis[true][elem.Mode]
				buf.WriteString(fmt.Sprintf("%s | %.0f%%", emoji.FormatForInline(), 100.0*float64(elem.Count)/float64(gamesPlayed)))
				if i < len(colorRankings)-1 && i < UserLeaderboardCount-1 {
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
		nameRankings := bot.PostgresInterface.NamesRankingForPlayerOnServer(userID, guildID)
		if len(nameRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i := 0; i < len(nameRankings) && i < UserLeaderboardCount; i++ {
				elem := nameRankings[i]
				buf.WriteString(fmt.Sprintf("%s | %.0f%%", elem.Mode, 100.0*float64(elem.Count)/float64(gamesPlayed)))
				if i < len(nameRankings)-1 && i < UserLeaderboardCount-1 {
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

		guildsPlayedIn := bot.PostgresInterface.NumGuildsPlayedInByUser(userID)
		if guildsPlayedIn > 0 {
			val := sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.userStatsEmbed.ServersPlayedInValue",
				Other: "{{.Servers}} Servers",
			}, map[string]interface{}{
				"Servers": guildsPlayedIn,
			})
			if guildsPlayedIn == 1 {
				val = sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.ServerPlayedInValue",
					Other: "{{.Server}} Server",
				}, map[string]interface{}{
					"Server": guildsPlayedIn,
				})
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.ServersPlayedIn",
					Other: "Played In",
				}),
				Value:  val,
				Inline: true,
			})
		} else {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "\u200b",
				Value:  "\u200b",
				Inline: true,
			})
		}

		totalCrewmateGames := bot.PostgresInterface.NumGamesAsRoleOnServer(userID, guildID, int16(game.CrewmateRole))
		if totalCrewmateGames > 0 {
			crewmateWins := bot.PostgresInterface.NumWinsAsRoleOnServer(userID, guildID, int16(game.CrewmateRole))
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.CrewmateWins",
					Other: "Crewmate Wins",
				}),
				Value:  fmt.Sprintf("%d/%d Games | %.0f%%", crewmateWins, totalCrewmateGames, 100.0*float64(crewmateWins)/float64(totalCrewmateGames)),
				Inline: true,
			})
		} else {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "\u200b",
				Value:  "\u200b",
				Inline: true,
			})
		}
		totalImposterGames := bot.PostgresInterface.NumGamesAsRoleOnServer(userID, guildID, int16(game.ImposterRole))
		if totalImposterGames > 0 {
			imposterWins := bot.PostgresInterface.NumWinsAsRoleOnServer(userID, guildID, int16(game.ImposterRole))
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.ImposterWins",
					Other: "Imposter Wins",
				}),
				Value:  fmt.Sprintf("%d/%d Games | %.0f%%", imposterWins, totalImposterGames, 100.0*float64(imposterWins)/float64(totalImposterGames)),
				Inline: true,
			})
		} else {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "\u200b",
				Value:  "\u200b",
				Inline: true,
			})
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "\u200b",
			Value:  "\u200b",
			Inline: true,
		})

		playerRankings := bot.PostgresInterface.OtherPlayersRankingForPlayerOnServer(userID, guildID)
		if len(playerRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i, v := range playerRankings {
				if i < LeaderboardSize {
					buf.WriteString(fmt.Sprintf("%d Games | %.0f%% | %s\n", v.Count, v.Percent,
						bot.MentionWithCacheData(strconv.FormatUint(v.UserID, 10), guildID, sett)))
				} else {
					break
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.MostPlayedWith",
					Other: "Most Played With",
				}),
				Value:  buf.String(),
				Inline: false,
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
		Color:     3066993, // GREEN
		Image:     nil,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL:      avatarURL,
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

func (bot *Bot) CheckOrFetchCachedUserData(userID, guildID string) (string, string, string) {
	info := rediskey.GetCachedUserInfo(context.Background(), bot.RedisInterface.client, userID, guildID)
	if info == "" {
		mem, err := bot.PrimarySession.GuildMember(guildID, userID)
		if err != nil {
			log.Println(err)
			return "", "", ""
		}
		if mem.User != nil {
			err := rediskey.SetCachedUserInfo(context.Background(), bot.RedisInterface.client, userID, guildID,
				fmt.Sprintf("%s:%s:%s", mem.User.Username, mem.Nick, mem.User.Discriminator))
			if err != nil {
				log.Println(err)
			}
			return mem.User.Username, mem.Nick, mem.User.Discriminator
		}
		return "", mem.Nick, ""
	}
	split := strings.Split(info, ":")
	if len(split) < 3 {
		return "", "", ""
	}
	return split[0], split[1], split[2]
}

// TODO add setting for caching/uncaching userdata
// TODO re-enable this feature after adding that setting
func (bot *Bot) MentionWithCacheData(userID, guildID string, sett *storage.GuildSettings) string {
	//userName, nickname, _ := bot.CheckOrFetchCachedUserData(userID, guildID)
	//if nickname != "" {
	//	return nickname
	//} else if userName != "" {
	//	return userName
	//}

	return "<@" + userID + ">"
}

func (bot *Bot) GuildStatsEmbed(guildID string, sett *storage.GuildSettings, prem premium.Tier) *discordgo.MessageEmbed {
	gname := ""
	avatarURL := ""
	g, err := bot.PrimarySession.Guild(guildID)

	if err != nil {
		log.Println(err)
		gname = guildID
	} else {
		gname = g.Name
		avatarURL = g.IconURL()
	}

	gamesPlayed := bot.PostgresInterface.NumGamesPlayedOnGuild(guildID)

	fields := make([]*discordgo.MessageEmbedField, 1)
	fields[0] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.guildStatsEmbed.GamesPlayed",
			Other: "Games Played",
		}),
		Value:  fmt.Sprintf("%d", gamesPlayed),
		Inline: false,
	}

	extraDesc := sett.LocalizeMessage(&i18n.Message{
		ID:    "responses.guildStatsEmbed.NoPremium",
		Other: "Detailed stats are only available for AutoMuteUs Premium users; type `{{.CommandPrefix}} premium` to learn more",
	}, map[string]interface{}{
		"CommandPrefix": sett.CommandPrefix,
	})

	if prem != premium.FreeTier {
		extraDesc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.guildStatsEmbed.Premium",
			Other: "Showing additional Premium Stats!\n(Note: stats are still in **BETA**, and will be likely be inaccurate while we work to improve them).",
		})
		gid, err := strconv.ParseUint(guildID, 10, 64)
		if err == nil {
			totalGameRankings := bot.PostgresInterface.TotalGamesRankingForServer(gid)

			buf := bytes.NewBuffer([]byte{})
			for i := 0; i < len(totalGameRankings) && i < LeaderboardSize; i++ {
				elem := totalGameRankings[i]
				buf.WriteString(fmt.Sprintf("%d | %s", elem.Count,
					bot.MentionWithCacheData(strconv.FormatUint(elem.Mode, 10), guildID, sett)))
				if i < len(totalGameRankings)-1 && i < LeaderboardSize-1 {
					buf.WriteByte('\n')
				}
			}
			if len(totalGameRankings) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.guildStatsEmbed.MostGames",
						Other: "Most Games",
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}

			overallGameRankings := bot.PostgresInterface.TotalWinRankingForServer(gid)
			buf = bytes.NewBuffer([]byte{})
			count := 0
			for i := 0; i < len(overallGameRankings) && count < LeaderboardSize; i++ {
				elem := overallGameRankings[i]
				if elem.Count > MinumumGameCountThreshold {
					buf.WriteString(fmt.Sprintf("%.0f%% | %s", elem.WinRate,
						bot.MentionWithCacheData(strconv.FormatUint(elem.UserID, 10), guildID, sett)))
					if i < len(overallGameRankings)-1 && count < LeaderboardSize-1 {
						buf.WriteByte('\n')
					}
					count++
				}
			}
			if len(overallGameRankings) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.guildStatsEmbed.TotalWinrate",
						Other: "Total Winrate (3+ Games)",
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "\u200b",
				Value:  "\u200b",
				Inline: true,
			})

			crewmateGameRankings := bot.PostgresInterface.TotalWinRankingForServerByRole(gid, 0)
			buf = bytes.NewBuffer([]byte{})
			count = 0
			for i := 0; i < len(crewmateGameRankings) && count < LeaderboardSize; i++ {
				elem := crewmateGameRankings[i]
				if elem.Count > MinumumGameCountThreshold {
					buf.WriteString(fmt.Sprintf("%.0f%% | %s", elem.WinRate,
						bot.MentionWithCacheData(strconv.FormatUint(elem.UserID, 10), guildID, sett)))
					if i < len(crewmateGameRankings)-1 && count < LeaderboardSize-1 {
						buf.WriteByte('\n')
					}
					count++
				}
			}
			if len(crewmateGameRankings) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.guildStatsEmbed.CrewmateWins",
						Other: "Crewmate Winrate (3+ Games)",
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}

			imposterGameRankings := bot.PostgresInterface.TotalWinRankingForServerByRole(gid, 1)
			buf = bytes.NewBuffer([]byte{})
			count = 0
			for i := 0; i < len(imposterGameRankings) && count < LeaderboardSize; i++ {
				elem := imposterGameRankings[i]
				if elem.Count > MinumumGameCountThreshold {
					buf.WriteString(fmt.Sprintf("%.0f%% | %s", elem.WinRate,
						bot.MentionWithCacheData(strconv.FormatUint(elem.UserID, 10), guildID, sett)))
					if i < len(imposterGameRankings)-1 && count < LeaderboardSize-1 {
						buf.WriteByte('\n')
					}
					count++
				}
			}
			if len(imposterGameRankings) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.guildStatsEmbed.ImposterWins",
						Other: "Imposter Winrate (3+ Games)",
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}
		}
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
		Color:     3066993, // GREEN
		Image:     nil,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL:      avatarURL,
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

func (bot *Bot) GameStatsEmbed(matchID, connectCode string, sett *storage.GuildSettings, prem premium.Tier) *discordgo.MessageEmbed {
	gameData, err := bot.PostgresInterface.GetGame(connectCode, matchID)
	if err != nil {
		log.Fatal(err)
	}

	events, err := bot.PostgresInterface.GetGameEvents(matchID)
	if err != nil {
		log.Fatal(err)
	}

	stats := storage.StatsFromGameAndEvents(gameData, events)
	return stats.ToDiscordEmbed(connectCode+":"+matchID, sett)
}
