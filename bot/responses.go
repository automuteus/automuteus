package bot

import (
	"fmt"
	"github.com/j0nas500/automuteus-tor/v8/pkg/amongus"
	"github.com/j0nas500/automuteus/v8/pkg/discord"
	"github.com/j0nas500/automuteus/v8/pkg/settings"
	"os"
	"strings"
	"time"

	"github.com/j0nas500/automuteus/v8/bot/setting"
	"github.com/j0nas500/automuteus/v8/pkg/game"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const ISO8601 = "2006-01-02T15:04:05-0700"

func settingResponse(settings []setting.Setting, sett *settings.GuildSettings, prem bool) *discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.settingResponse.Title",
			Other: "Settings",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.settingResponse.Description",
			Other: "Type `/settings <setting>` to change a setting from those listed below",
		}),
		Timestamp: "",
		Color:     15844367, // GOLD
		Image:     nil,
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
	}

	fields := make([]*discordgo.MessageEmbedField, 0)
	for _, v := range settings {
		if !v.Premium {
			name := v.Name
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   name,
				Value:  sett.LocalizeMessage(&i18n.Message{Other: v.ShortDesc}),
				Inline: true,
			})
		}
	}
	var desc string
	if prem {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.settingResponse.PremiumThanks",
			Other: "Thanks for being an AutoMuteUs Premium user!",
		})
	} else {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.settingResponse.PremiumNoThanks",
			Other: "The following settings are only for AutoMuteUs premium users.\nType `/premium` to learn more!",
		})
	}
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "\u200B",
		Value:  "\u200B",
		Inline: false,
	})
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "💎 Premium Settings 💎",
		Value:  desc,
		Inline: false,
	})
	for _, v := range settings {
		if v.Premium {
			name := v.Name
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   name,
				Value:  sett.LocalizeMessage(&i18n.Message{Other: v.ShortDesc}),
				Inline: true,
			})
		}
	}

	embed.Fields = fields
	return &embed
}

func (bot *Bot) gameStateResponse(dgs *GameState, sett *settings.GuildSettings) *discordgo.MessageEmbed {
	// we need to generate the messages based on the state of the game
	messages := map[game.Phase]func(dgs *GameState, emojis AlivenessEmojis, sett *settings.GuildSettings) *discordgo.MessageEmbed{
		game.MENU:     menuMessage,
		game.LOBBY:    lobbyMessage,
		game.TASKS:    gamePlayMessage,
		game.DISCUSS:  gamePlayMessage,
		game.GAMEOVER: gamePlayMessage,
	}
	return messages[dgs.GameData.Phase](dgs, bot.StatusEmojis, sett)
}

func lobbyMetaEmbedFields(room, region string, author, voiceChannelID string, playerCount int, linkedPlayers int, sett *settings.GuildSettings) []*discordgo.MessageEmbedField {
	gameInfoFields := make([]*discordgo.MessageEmbedField, 0)
	if author != "" {
		gameInfoFields = append(gameInfoFields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.Host",
				Other: "Host",
			}),
			Value:  discord.MentionByUserID(author),
			Inline: true,
		})
	}
	if voiceChannelID != "" {
		gameInfoFields = append(gameInfoFields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.VoiceChannel",
				Other: "Voice Channel",
			}),
			Value:  discord.MentionByChannelID(voiceChannelID),
			Inline: true,
		})
	}
	if linkedPlayers > playerCount {
		linkedPlayers = playerCount
	}
	gameInfoFields = append(gameInfoFields, &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMetaEmbedFields.PlayersLinked",
			Other: "Players Linked",
		}),
		Value:  fmt.Sprintf("%v/%v", linkedPlayers, playerCount),
		Inline: true,
	})
	if room != "" {
		switch {
		case sett.DisplayRoomCode == "spoiler":
			room = fmt.Sprintf("||%v||", room)
		case sett.DisplayRoomCode == "never":
			room = strings.Repeat("\\*", len(room))
		}
		gameInfoFields = append(gameInfoFields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.RoomCode",
				Other: "🔒 ROOM CODE",
			}),
			Value:  room,
			Inline: false,
		})
	}
	if region != "" {
		gameInfoFields = append(gameInfoFields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.Region",
				Other: "🌎 REGION",
			}),
			Value:  region,
			Inline: false,
		})
	}

	return gameInfoFields
}

func menuMessage(dgs *GameState, _ AlivenessEmojis, sett *settings.GuildSettings) *discordgo.MessageEmbed {
	var footer *discordgo.MessageEmbedFooter
	desc, color := dgs.descriptionAndColor(sett)
	if color == discord.DEFAULT {
		color = discord.GREEN
		footer = &discordgo.MessageEmbedFooter{
			Text: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.menuMessage.Linked.FooterText",
				Other: "(Enter a game lobby in Among Us to start the match)",
			}),
			IconURL:      "",
			ProxyIconURL: "",
		}
	}

	fields := make([]*discordgo.MessageEmbedField, 0)
	author := dgs.GameStateMsg.LeaderID
	if author != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.Host",
				Other: "Host",
			}),
			Value:  discord.MentionByUserID(author),
			Inline: true,
		})
	}
	if dgs.VoiceChannel != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.VoiceChannel",
				Other: "Voice Channel",
			}),
			Value:  "<#" + dgs.VoiceChannel + ">",
			Inline: true,
		})
	}
	if len(fields) == 2 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "\u200B",
			Value:  "\u200B",
			Inline: true,
		})
	}

	msg := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.menuMessage.Title",
			Other: "Main Menu",
		}),
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
		Footer:      footer,
		Color:       color,
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      fields,
	}
	return &msg
}

func lobbyMessage(dgs *GameState, emojis AlivenessEmojis, sett *settings.GuildSettings) *discordgo.MessageEmbed {
	room, region, playMap := dgs.GameData.GetRoomRegionMap()
	gameInfoFields := lobbyMetaEmbedFields(room, region, dgs.GameStateMsg.LeaderID, dgs.VoiceChannel, dgs.GameData.GetNumDetectedPlayers(), dgs.GetCountLinked(), sett)

	listResp := dgs.ToEmojiEmbedFields(emojis, sett)
	listResp = append(gameInfoFields, listResp...)

	desc, color := dgs.descriptionAndColor(sett)
	if color == discord.DEFAULT {
		color = discord.GREEN
	}

	msg := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMessage.Title",
			Other: "Lobby",
		}),
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
		Footer: &discordgo.MessageEmbedFooter{
			Text: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMessage.Footer.Text",
				Other: "Use the select below with your in-game color! (or {{.X}} to leave)",
			},
				map[string]interface{}{
					"X": X,
				}),
			IconURL:      "",
			ProxyIconURL: "",
		},
		Color:     color,
		Image:     nil,
		Thumbnail: getThumbnailFromMap(playMap, sett),
		Video:     nil,
		Provider:  nil,
		Author:    nil,
		Fields:    listResp,
	}
	return &msg
}

func gameOverMessage(dgs *GameState, emojis AlivenessEmojis, sett *settings.GuildSettings, winners string) *discordgo.MessageEmbed {
	_, _, playMap := dgs.GameData.GetRoomRegionMap()

	listResp := dgs.ToEmojiEmbedFields(emojis, sett)

	desc := sett.LocalizeMessage(&i18n.Message{
		ID:    "eventHandler.gameOver.matchID",
		Other: "Game Over! View the match's stats using Match ID: `{{.MatchID}}`\n{{.Winners}}",
	},
		map[string]interface{}{
			"MatchID": matchIDCode(dgs.ConnectCode, dgs.MatchID),
			"Winners": winners,
		})

	var footer *discordgo.MessageEmbedFooter

	if sett.DeleteGameSummaryMinutes > 0 {
		footer = &discordgo.MessageEmbedFooter{
			Text: sett.LocalizeMessage(&i18n.Message{
				ID:    "eventHandler.gameOver.deleteMessageFooter",
				Other: "Deleting message {{.Mins}} mins from:",
			},
				map[string]interface{}{
					"Mins": sett.DeleteGameSummaryMinutes,
				}),
			IconURL:      "",
			ProxyIconURL: "",
		}
	}

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       sett.LocalizeMessage(amongus.ToLocale(game.GAMEOVER)),
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
		Footer:      footer,
		Color:       discord.DARK_GOLD, // DARK GOLD
		Image:       nil,
		Thumbnail:   getThumbnailFromMap(playMap, sett),
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      listResp,
	}
	return &msg
}

func getThumbnailFromMap(playMap game.PlayMap, sett *settings.GuildSettings) *discordgo.MessageEmbedThumbnail {
	url := game.FormMapUrl(os.Getenv("BASE_MAP_URL"), playMap, sett.MapVersion == "detailed")
	if url != "" {
		return &discordgo.MessageEmbedThumbnail{
			URL: url,
		}
	}
	return nil
}

func gamePlayMessage(dgs *GameState, emojis AlivenessEmojis, sett *settings.GuildSettings) *discordgo.MessageEmbed {
	phase := dgs.GameData.GetPhase()
	playMap := dgs.GameData.GetPlayMap()
	// send empty fields because we don't need to display those fields during the game...
	listResp := dgs.ToEmojiEmbedFields(emojis, sett)

	gameInfoFields := lobbyMetaEmbedFields("", "", dgs.GameStateMsg.LeaderID, dgs.VoiceChannel, dgs.GameData.GetNumDetectedPlayers(), dgs.GetCountLinked(), sett)
	listResp = append(gameInfoFields, listResp...)
	desc, color := dgs.descriptionAndColor(sett)
	if color == discord.DEFAULT {
		switch phase {
		case game.TASKS:
			color = discord.BLUE
		case game.DISCUSS:
			color = discord.PURPLE
		}
	}
	title := sett.LocalizeMessage(amongus.ToLocale(phase))

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       title,
		Description: desc,
		Timestamp:   time.Now().Format(ISO8601),
		Color:       color,
		Footer:      nil,
		Image:       nil,
		Thumbnail:   getThumbnailFromMap(playMap, sett),
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      listResp,
	}

	return &msg
}

// returns the description and color to use, based on the gamestate
// usage dictates DEFAULT should be overwritten by other state subsequently,
// whereas RED and DARK_ORANGE are error/flag values that should be passed on
func (dgs *GameState) descriptionAndColor(sett *settings.GuildSettings) (string, int) {
	if !dgs.Linked {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.notLinked.Description",
			Other: "❌**No capture linked! Click the link above to connect!**❌",
		}), discord.RED // red
	} else if !dgs.Running {
		return sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.makeDescription.GameNotRunning",
			Other: "\n⚠ **Bot is Paused!** ⚠\n\n",
		}), discord.DARK_ORANGE
	}
	return "\n", discord.DEFAULT

}

func nonPremiumSettingResponse(sett *settings.GuildSettings) string {
	return sett.LocalizeMessage(&i18n.Message{
		ID:    "responses.nonPremiumSetting.Desc",
		Other: "Sorry, but that setting is reserved for AutoMuteUs Premium users! See `/premium` for details",
	})
}
