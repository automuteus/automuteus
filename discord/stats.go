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
		leaderBoardSize := sett.GetLeaderboardSize()
		extraDesc = ""
		//extraDesc = sett.LocalizeMessage(&i18n.Message{
		//	ID:    "responses.userStatsEmbed.Premium",
		//	Other: "Showing additional Premium Stats!\n(Note: stats are still in **BETA**, and will be likely be inaccurate while we work to improve them).",
		//})
		colorRankings := bot.PostgresInterface.ColorRankingForPlayerOnServer(userID, guildID)
		if len(colorRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i := 0; i < len(colorRankings) && i < leaderBoardSize; i++ {
				elem := colorRankings[i]
				emoji := bot.StatusEmojis[true][elem.Mode]
				buf.WriteString(fmt.Sprintf("%s | %.0f%%", emoji.FormatForInline(), 100.0*float64(elem.Count)/float64(gamesPlayed)))
				if i < len(colorRankings)-1 && i < leaderBoardSize-1 {
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
			for i := 0; i < len(nameRankings) && i < leaderBoardSize; i++ {
				elem := nameRankings[i]
				buf.WriteString(fmt.Sprintf("%s | %.0f%%", elem.Mode, 100.0*float64(elem.Count)/float64(gamesPlayed)))
				if i < len(nameRankings)-1 && i < leaderBoardSize-1 {
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
				Value: fmt.Sprintf("%d/%d %s | %.0f%%", crewmateWins, totalCrewmateGames,
					sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.stats.Games",
						Other: "Games",
					}),
					100.0*float64(crewmateWins)/float64(totalCrewmateGames)),
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
				Value: fmt.Sprintf("%d/%d %s | %.0f%%", imposterWins, totalImposterGames,
					sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.stats.Games",
						Other: "Games",
					}),
					100.0*float64(imposterWins)/float64(totalImposterGames)),
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
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "\u200b",
			Value:  "\u200b",
			Inline: false,
		})

		playerRankings := bot.PostgresInterface.OtherPlayersRankingForPlayerOnServer(userID, guildID)
		if len(playerRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i, v := range playerRankings {
				if i < leaderBoardSize {
					buf.WriteString(fmt.Sprintf("%d %s | %.0f%% | %s\n", v.Count,
						sett.LocalizeMessage(&i18n.Message{
							ID:    "responses.stats.Games",
							Other: "Games",
						}),
						v.Percent,
						bot.MentionWithCacheData(strconv.FormatUint(v.UserID, 10), guildID, sett)))
				} else {
					break
				}
			}
		}

		bestImpostorTeammateRankings := bot.PostgresInterface.BestTeammateByRole(userID, guildID, int16(game.ImposterRole), 2)
		if len(bestImpostorTeammateRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i, v := range bestImpostorTeammateRankings {
				if i < leaderBoardSize {
					buf.WriteString(fmt.Sprintf("%d/%d %s | %.0f%% | %s\n", v.WinCount, v.Count,
						sett.LocalizeMessage(&i18n.Message{
							ID:    "responses.stats.Won",
							Other: "Won",
						}),
						v.WinRate,
						bot.MentionWithCacheData(strconv.FormatUint(v.TeammateID, 10), guildID, sett)))
				} else {
					break
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.BestTeammateImpostor",
					Other: "Best Impostor Played With",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}

		worstImpostorTeammateRankings := bot.PostgresInterface.WorstTeammateByRole(userID, guildID, int16(game.ImposterRole), 2)
		if len(worstImpostorTeammateRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i, v := range worstImpostorTeammateRankings {
				if i < leaderBoardSize {
					buf.WriteString(fmt.Sprintf("%d/%d %s | %.0f%% | %s\n", v.LooseCount, v.Count,
						sett.LocalizeMessage(&i18n.Message{
							ID:    "responses.stats.Lost",
							Other: "Lost",
						}),
						v.LooseRate,
						bot.MentionWithCacheData(strconv.FormatUint(v.TeammateID, 10), guildID, sett)))
				} else {
					break
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.WorstTeammateImpostor",
					Other: "Worst Impostor Played With",
				}),
				Value:  buf.String(),
				Inline: true,
			})
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "\u200b",
				Value:  "\u200b",
				Inline: false,
			})
		}

		bestCrewmateTeammateRankings := bot.PostgresInterface.BestTeammateByRole(userID, guildID, int16(game.CrewmateRole), sett.GetLeaderboardMin())
		if len(bestCrewmateTeammateRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i, v := range bestCrewmateTeammateRankings {
				if i < leaderBoardSize {
					buf.WriteString(fmt.Sprintf("%d/%d %s | %.0f%% | %s\n", v.WinCount, v.Count,
						sett.LocalizeMessage(&i18n.Message{
							ID:    "responses.stats.Won",
							Other: "Won",
						}),
						v.WinRate,
						bot.MentionWithCacheData(strconv.FormatUint(v.TeammateID, 10), guildID, sett)))
				} else {
					break
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.BestTeammateCrewmate",
					Other: "Best Crewmate Played With",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}

		worstCrewmateTeammateRankings := bot.PostgresInterface.WorstTeammateByRole(userID, guildID, int16(game.CrewmateRole), sett.GetLeaderboardMin())
		if len(bestCrewmateTeammateRankings) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i, v := range worstCrewmateTeammateRankings {
				if i < leaderBoardSize {
					buf.WriteString(fmt.Sprintf("%d/%d %s | %.0f%% | %s\n", v.LooseCount, v.Count,
						sett.LocalizeMessage(&i18n.Message{
							ID:    "responses.stats.Lost",
							Other: "Lost",
						}),
						v.LooseRate,
						bot.MentionWithCacheData(strconv.FormatUint(v.TeammateID, 10), guildID, sett)))
				} else {
					break
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.WorstTeammateCrewmate",
					Other: "Worst Crewmate Played With",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}

		userExiledAsImpostor := bot.PostgresInterface.UserWinByActionAndRole(userID, guildID, strconv.Itoa(int(game.EXILED)), int16(game.ImposterRole))
		if len(userExiledAsImpostor) > 0 {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "\u200b",
				Value:  "\u200b",
				Inline: false,
			})
			buf := bytes.NewBuffer([]byte{})
			for i, v := range userExiledAsImpostor {
				if i < leaderBoardSize {
					buf.WriteString(fmt.Sprintf("%d/%d %s | %.0f%% %s\n", v.TotalAction, v.Count,
						sett.LocalizeMessage(&i18n.Message{
							ID:    "responses.stats.Exiled",
							Other: "Exiled",
						}),
						v.WinRate,
						sett.LocalizeMessage(&i18n.Message{
							ID:    "responses.stats.Won",
							Other: "Won",
						})))
				} else {
					break
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.ExiledAsImpostor",
					Other: "Exiled as Impostor",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}

		userExiledAsCrewmate := bot.PostgresInterface.UserWinByActionAndRole(userID, guildID, strconv.Itoa(int(game.EXILED)), int16(game.CrewmateRole))
		if len(userExiledAsImpostor) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i, v := range userExiledAsCrewmate {
				if i < leaderBoardSize {
					buf.WriteString(fmt.Sprintf("%d/%d %s | %.0f%% %s\n", v.TotalAction, v.Count,
						sett.LocalizeMessage(&i18n.Message{
							ID:    "responses.stats.Exiled",
							Other: "Exiled",
						}),
						v.WinRate,
						sett.LocalizeMessage(&i18n.Message{
							ID:    "responses.stats.Won",
							Other: "Won",
						})))
				} else {
					break
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.ExiledAsCrewmate",
					Other: "Exiled as Crewmate",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}

		userKilledAsCrewmate := bot.PostgresInterface.UserWinByActionAndRole(userID, guildID, strconv.Itoa(int(game.DIED)), int16(game.CrewmateRole))
		if len(userKilledAsCrewmate) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i, v := range userKilledAsCrewmate {
				if i < leaderBoardSize {
					buf.WriteString(fmt.Sprintf("%d/%d %s | %.0f%% %s\n", v.TotalAction, v.Count,
						sett.LocalizeMessage(&i18n.Message{
							ID:    "responses.stats.Killed",
							Other: "Killed",
						}),
						v.WinRate,
						sett.LocalizeMessage(&i18n.Message{
							ID:    "responses.stats.Won",
							Other: "Won",
						})))
				} else {
					break
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.KilledAsCrewmate",
					Other: "Killed as Crewmate",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}

		userFirstTimeKilled := bot.PostgresInterface.UserFrequentFirstTarget(userID, guildID, strconv.Itoa(int(game.DIED)), sett.GetLeaderboardSize())
		if len(userFirstTimeKilled) > 0 {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "\u200b",
				Value:  "\u200b",
				Inline: false,
			})
			buf := bytes.NewBuffer([]byte{})
			for i, v := range userFirstTimeKilled {
				if i < leaderBoardSize {
					buf.WriteString(fmt.Sprintf("%d/%d | %.0f%%\n", v.TotalDeath, v.Count, v.DeathRate))
				} else {
					break
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.FrequentFirstTarget",
					Other: "Frequent first target",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}

		userMostFrequentKilledBy := bot.PostgresInterface.UserMostFrequentKilledBy(userID, guildID)
		if len(userMostFrequentKilledBy) > 0 {
			buf := bytes.NewBuffer([]byte{})
			for i, v := range userMostFrequentKilledBy {
				if i < leaderBoardSize {
					buf.WriteString(fmt.Sprintf("%d/%d | %.0f%% | %s\n", v.TotalDeath, v.Encounter, v.DeathRate,
						bot.MentionWithCacheData(strconv.FormatUint(v.TeammateID, 10), guildID, sett)))
				} else {
					break
				}
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "responses.userStatsEmbed.FrequentKilledBy",
					Other: "Frequent Killed By",
				}),
				Value:  buf.String(),
				Inline: true,
			})
		}
	}

	fields = TrimEmbedFields(fields)

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

func (bot *Bot) MentionWithCacheData(userID, guildID string, sett *storage.GuildSettings) string {
	if !sett.LeaderboardMention {
		userName, nickname, _ := bot.CheckOrFetchCachedUserData(userID, guildID)
		if nickname != "" {
			return nickname
		} else if userName != "" {
			return userName
		}
	}

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
		Inline: gamesPlayed > 0, // if 0 games played, this would be on its own line
	}

	if gamesPlayed > 0 {
		crewmateWins := bot.PostgresInterface.NumGamesWonAsRoleOnServer(guildID, game.CrewmateRole)
		imposterWins := bot.PostgresInterface.NumGamesWonAsRoleOnServer(guildID, game.ImposterRole)

		fields = append(fields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.guildStatsEmbed.GamesWonCrewmate",
				Other: "Crewmate Winrate",
			}),
			Value:  fmt.Sprintf("%.0f%%", 100.0*(float64(crewmateWins)/float64(gamesPlayed))),
			Inline: true,
		})
		fields = append(fields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.guildStatsEmbed.GamesWonImposter",
				Other: "Imposter Winrate",
			}),
			Value:  fmt.Sprintf("%.0f%%", 100.0*(float64(imposterWins)/float64(gamesPlayed))),
			Inline: true,
		})
	}

	extraDesc := sett.LocalizeMessage(&i18n.Message{
		ID:    "responses.guildStatsEmbed.NoPremium",
		Other: "Detailed stats are only available for AutoMuteUs Premium users; type `{{.CommandPrefix}} premium` to learn more",
	}, map[string]interface{}{
		"CommandPrefix": sett.CommandPrefix,
	})

	if prem != premium.FreeTier {
		leaderboardSize := sett.GetLeaderboardSize()
		leaderboardMin := sett.GetLeaderboardMin()
		extraDesc = ""
		//extraDesc = sett.LocalizeMessage(&i18n.Message{
		//	ID:    "responses.guildStatsEmbed.Premium",
		//	Other: "Showing additional Premium Stats!\n(Note: stats are still in **BETA**, and will be likely be inaccurate while we work to improve them).",
		//})
		gid, err := strconv.ParseUint(guildID, 10, 64)
		if err == nil {
			totalGameRankings := bot.PostgresInterface.TotalGamesRankingForServer(gid)

			buf := bytes.NewBuffer([]byte{})
			for i := 0; i < len(totalGameRankings) && i < leaderboardSize; i++ {
				elem := totalGameRankings[i]
				buf.WriteString(fmt.Sprintf("%d | %s", elem.Count,
					bot.MentionWithCacheData(strconv.FormatUint(elem.Mode, 10), guildID, sett)))
				if i < len(totalGameRankings)-1 && i < leaderboardSize-1 {
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
			for i := 0; i < len(overallGameRankings) && count < leaderboardSize; i++ {
				elem := overallGameRankings[i]
				if elem.Count > int64(leaderboardMin) {
					buf.WriteString(fmt.Sprintf("%.0f%% | %s", elem.WinRate,
						bot.MentionWithCacheData(strconv.FormatUint(elem.UserID, 10), guildID, sett)))
					if i < len(overallGameRankings)-1 && count < leaderboardSize-1 {
						buf.WriteByte('\n')
					}
					count++
				}
			}
			if len(overallGameRankings) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.guildStatsEmbed.TotalWinrate",
						Other: "Total Winrate ({{.Min}}+ Games)",
					}, map[string]interface{}{
						"Min": leaderboardMin,
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "\u200b",
				Value:  "\u200b",
				Inline: false,
			})

			crewmateGameRankings := bot.PostgresInterface.TotalWinRankingForServerByRole(gid, 0)
			buf = bytes.NewBuffer([]byte{})
			count = 0
			for i := 0; i < len(crewmateGameRankings) && count < leaderboardSize; i++ {
				elem := crewmateGameRankings[i]
				if elem.Count > int64(leaderboardMin) {
					buf.WriteString(fmt.Sprintf("%.0f%% | %s", elem.WinRate,
						bot.MentionWithCacheData(strconv.FormatUint(elem.UserID, 10), guildID, sett)))
					if i < len(crewmateGameRankings)-1 && count < leaderboardSize-1 {
						buf.WriteByte('\n')
					}
					count++
				}
			}
			if len(crewmateGameRankings) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.guildStatsEmbed.CrewmateWins",
						Other: "Crewmate Winrate ({{.Min}}+ Games)",
					}, map[string]interface{}{
						"Min": leaderboardMin,
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}

			imposterGameRankings := bot.PostgresInterface.TotalWinRankingForServerByRole(gid, 1)
			buf = bytes.NewBuffer([]byte{})
			count = 0
			for i := 0; i < len(imposterGameRankings) && count < leaderboardSize; i++ {
				elem := imposterGameRankings[i]
				if elem.Count > int64(leaderboardMin) {
					buf.WriteString(fmt.Sprintf("%.0f%% | %s", elem.WinRate,
						bot.MentionWithCacheData(strconv.FormatUint(elem.UserID, 10), guildID, sett)))
					if i < len(imposterGameRankings)-1 && count < leaderboardSize-1 {
						buf.WriteByte('\n')
					}
					count++
				}
			}
			if len(imposterGameRankings) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.guildStatsEmbed.ImposterWins",
						Other: "Imposter Winrate ({{.Min}}+ Games)",
					}, map[string]interface{}{
						"Min": leaderboardMin,
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}

			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "\u200b",
				Value:  "\u200b",
				Inline: false,
			})

			bestImpostorTeammateForServerRankings := bot.PostgresInterface.BestTeammateForServerByRole(guildID, int16(game.ImposterRole), 2)
			if len(bestImpostorTeammateForServerRankings) > 0 {
				buf := bytes.NewBuffer([]byte{})
				for i, v := range bestImpostorTeammateForServerRankings {
					if i < leaderboardSize {
						buf.WriteString(fmt.Sprintf("%.0f%% | %s | %s\n", v.WinRate,
							bot.MentionWithCacheData(strconv.FormatUint(v.UserID, 10), guildID, sett),
							bot.MentionWithCacheData(strconv.FormatUint(v.TeammateID, 10), guildID, sett)))
					} else {
						break
					}
				}
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.userStatsEmbed.BestTeammateServerImpostor",
						Other: "Best Impostor Team",
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}

			worstImpostorTeammateServerRankings := bot.PostgresInterface.WorstTeammateForServerByRole(guildID, int16(game.ImposterRole), 2)
			if len(worstImpostorTeammateServerRankings) > 0 {
				buf := bytes.NewBuffer([]byte{})
				for i, v := range worstImpostorTeammateServerRankings {
					if i < leaderboardSize {
						buf.WriteString(fmt.Sprintf("%.0f%% | %s | %s\n", v.LooseRate,
							bot.MentionWithCacheData(strconv.FormatUint(v.UserID, 10), guildID, sett),
							bot.MentionWithCacheData(strconv.FormatUint(v.TeammateID, 10), guildID, sett)))
					} else {
						break
					}
				}
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.userStatsEmbed.WorstTeammateServerImpostor",
						Other: "Worst Impostor Team",
					}),
					Value:  buf.String(),
					Inline: true,
				})
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   "\u200b",
					Value:  "\u200b",
					Inline: false,
				})
			}

			bestCrewmateTeammateServerRankings := bot.PostgresInterface.BestTeammateForServerByRole(guildID, int16(game.CrewmateRole), sett.GetLeaderboardMin())
			if len(bestCrewmateTeammateServerRankings) > 0 {
				buf := bytes.NewBuffer([]byte{})
				for i, v := range bestCrewmateTeammateServerRankings {
					if i < leaderboardSize {
						buf.WriteString(fmt.Sprintf("%.0f%% | %s | %s\n", v.WinRate,
							bot.MentionWithCacheData(strconv.FormatUint(v.UserID, 10), guildID, sett),
							bot.MentionWithCacheData(strconv.FormatUint(v.TeammateID, 10), guildID, sett)))
					} else {
						break
					}
				}
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.userStatsEmbed.BestTeammateServerCrewmate",
						Other: "Best Crewmate Team",
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}

			worstCrewmateTeammateRankings := bot.PostgresInterface.WorstTeammateForServerByRole(guildID, int16(game.CrewmateRole), sett.GetLeaderboardMin())
			if len(worstCrewmateTeammateRankings) > 0 {
				buf := bytes.NewBuffer([]byte{})
				for i, v := range worstCrewmateTeammateRankings {
					if i < leaderboardSize {
						buf.WriteString(fmt.Sprintf("%.0f%% | %s | %s\n", v.LooseRate,
							bot.MentionWithCacheData(strconv.FormatUint(v.UserID, 10), guildID, sett),
							bot.MentionWithCacheData(strconv.FormatUint(v.TeammateID, 10), guildID, sett)))
					} else {
						break
					}
				}
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.userStatsEmbed.WorstTeammateServerCrewmate",
						Other: "Worst Crewmate Team",
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}

			userMostFirstTimeKilledForServer := bot.PostgresInterface.UserMostFrequentFirstTargetForServer(guildID, strconv.Itoa(int(game.DIED)), sett.GetLeaderboardSize())
			if len(userMostFirstTimeKilledForServer) > 0 {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   "\u200b",
					Value:  "\u200b",
					Inline: false,
				})
				buf := bytes.NewBuffer([]byte{})
				for i, v := range userMostFirstTimeKilledForServer {
					if i < leaderboardSize {
						buf.WriteString(fmt.Sprintf("%d/%d | %.0f%% | %s\n", v.TotalDeath, v.Count, v.DeathRate,
							bot.MentionWithCacheData(strconv.FormatUint(v.UserID, 10), guildID, sett)))
					} else {
						break
					}
				}
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.userStatsEmbed.MostFrequentFirstTarget",
						Other: "Most Frequent First Target",
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}

			userMostFrequentKilledByServer := bot.PostgresInterface.UserMostFrequentKilledByServer(guildID)
			if len(userMostFrequentKilledByServer) > 0 {
				buf := bytes.NewBuffer([]byte{})
				for i, v := range userMostFrequentKilledByServer {
					if i < leaderboardSize {
						buf.WriteString(fmt.Sprintf("%d/%d | %.0f%% | %s %s %s\n", v.TotalDeath, v.Encounter, v.DeathRate,
							bot.MentionWithCacheData(strconv.FormatUint(v.TeammateID, 10), guildID, sett),
							sett.LocalizeMessage(&i18n.Message{
								ID:    "responses.stats.Killed",
								Other: ":knife:",
							}),
							bot.MentionWithCacheData(strconv.FormatUint(v.UserID, 10), guildID, sett)))
					} else {
						break
					}
				}
				fields = append(fields, &discordgo.MessageEmbedField{
					Name: sett.LocalizeMessage(&i18n.Message{
						ID:    "responses.userStatsEmbed.FrequentKilledBy",
						Other: " Most Frequent Killed By",
					}),
					Value:  buf.String(),
					Inline: true,
				})
			}
		}
	}

	fields = TrimEmbedFields(fields)

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

func TrimEmbedFields(fields []*discordgo.MessageEmbedField) []*discordgo.MessageEmbedField {
	i := 0
	for _, v := range fields {
		if v.Value != "" {
			if v.Value == "69" || strings.Contains(v.Value, " 69") || v.Value == "420" || strings.Contains(v.Value, " 420") {
				v.Value = "😎 " + v.Value + " 😎"
			}
			fields[i] = v
			i++
		}
	}
	// prevent memory leak by erasing truncated values
	for j := i; j < len(fields); j++ {
		fields[j] = nil
	}

	return fields[:i]
}
