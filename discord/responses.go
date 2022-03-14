package discord

import (
	"bytes"
	"fmt"
	"github.com/automuteus/automuteus/discord/command"
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/settings"
	"os"
	"strings"
	"time"

	"github.com/automuteus/automuteus/amongus"
	"github.com/automuteus/automuteus/discord/setting"
	"github.com/automuteus/utils/pkg/game"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const ISO8601 = "2006-01-02T15:04:05-0700"

func settingResponse(commandPrefix string, settings []setting.Setting, sett *settings.GuildSettings, prem bool) *discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.settingResponse.Title",
			Other: "Settings",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.settingResponse.Description",
			Other: "Type `{{.CommandPrefix}} settings <setting>` to change a setting from those listed below",
		},
			map[string]interface{}{
				"CommandPrefix": commandPrefix,
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
				Value:  sett.LocalizeMessage(v.ShortDesc),
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
			Other: "The following settings are only for AutoMuteUs premium users.\nType `{{.CommandPrefix}} premium` to learn more!",
		},
			map[string]interface{}{
				"CommandPrefix": commandPrefix,
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
				Value:  sett.LocalizeMessage(v.ShortDesc),
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
	color := 15158332 // red
	desc := ""
	var footer *discordgo.MessageEmbedFooter
	if dgs.Linked {
		desc = dgs.makeDescription(sett)
		color = 3066993
		footer = &discordgo.MessageEmbedFooter{
			Text: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.menuMessage.Linked.FooterText",
				Other: "(Enter a game lobby in Among Us to start the match)",
			}),
			IconURL:      "",
			ProxyIconURL: "",
		}
	} else {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.menuMessage.notLinked.Description",
			Other: "❌**No capture linked! Click the link above to connect!**❌",
		})
		footer = nil
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

	color := 15158332 // red
	desc := ""
	if dgs.Linked {
		desc = dgs.makeDescription(sett)
		color = 3066993
	} else {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMessage.notLinked.Description",
			Other: "❌**No capture linked! Click the link above to connect!**❌",
		})
	}

	emojiLeave := "❌"
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
				Other: "React to this message with your in-game color! (or {{.emojiLeave}} to leave)",
			},
				map[string]interface{}{
					"emojiLeave": emojiLeave,
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
		Color:       12745742, // DARK GOLD
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
	var thumbNail *discordgo.MessageEmbedThumbnail = nil
	if playMap != game.EMPTYMAP && playMap != game.DLEKS {
		thumbNail = &discordgo.MessageEmbedThumbnail{
			URL: command.FormMapUrl(os.Getenv("BASE_MAP_URL"), playMap, sett.MapVersion == "detailed"),
		}
	}
	return thumbNail
}

func gamePlayMessage(dgs *GameState, emojis AlivenessEmojis, sett *settings.GuildSettings) *discordgo.MessageEmbed {
	phase := dgs.GameData.GetPhase()
	playMap := dgs.GameData.GetPlayMap()
	// send empty fields because we don't need to display those fields during the game...
	listResp := dgs.ToEmojiEmbedFields(emojis, sett)
	desc := ""

	desc = dgs.makeDescription(sett)
	gameInfoFields := lobbyMetaEmbedFields("", "", dgs.GameStateMsg.LeaderID, dgs.VoiceChannel, dgs.GameData.GetNumDetectedPlayers(), dgs.GetCountLinked(), sett)
	listResp = append(gameInfoFields, listResp...)

	var color int
	switch phase {
	case game.TASKS:
		color = 3447003 // BLUE
	case game.DISCUSS:
		color = 10181046 // PURPLE
	default:
		color = 15158332 // RED
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

func (dgs *GameState) makeDescription(sett *settings.GuildSettings) string {
	buf := bytes.NewBuffer([]byte{})
	if !dgs.Running {
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.makeDescription.GameNotRunning",
			Other: "\n⚠ **Bot is Paused!** ⚠\n\n",
		}))
	} else {
		buf.WriteRune('\n')
	}

	return buf.String()
}

func nonPremiumSettingResponse(sett *settings.GuildSettings) string {
	return sett.LocalizeMessage(&i18n.Message{
		ID:    "responses.nonPremiumSetting.Desc",
		Other: "Sorry, but that setting is reserved for AutoMuteUs Premium users! See `{{.CommandPrefix}} premium` for details",
	}, map[string]interface{}{
		"CommandPrefix": sett.GetCommandPrefix(),
	})
}
