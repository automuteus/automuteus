package discord

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/automuteus/galactus/broker"
	"strings"
	"time"

	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"

	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const ISO8601 = "2006-01-02T15:04:05-0700"

func helpResponse(isAdmin, isPermissioned bool, CommandPrefix string, commands []Command, sett *storage.GuildSettings) discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.Title",
			Other: "AutoMuteUs Bot Commands:\n",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.SubTitle",
			Other: "[View the Github Project](https://github.com/denverquane/automuteus) or [Join our Discord](https://discord.gg/ZkqZSWF)\n\nType `{{.CommandPrefix}} help <command>` to see more details on a command!",
		},
			map[string]interface{}{
				"CommandPrefix": CommandPrefix,
			}),
		Timestamp: "",
		Color:     15844367, //GOLD
		Image:     nil,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL:      "https://github.com/denverquane/automuteus/blob/master/assets/BotProfilePicture.png?raw=true",
			ProxyURL: "",
			Width:    0,
			Height:   0,
		},
		Video:    nil,
		Provider: nil,
		Author:   nil,
		Footer:   nil,
	}

	fields := make([]*discordgo.MessageEmbedField, 0)
	for _, v := range commands {
		if !v.secret && v.cmdType != Help && v.cmdType != Null {
			if (!v.adminSetting || isAdmin) && (!v.permissionSetting || isPermissioned) {
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   v.emoji + " " + v.command,
					Value:  sett.LocalizeMessage(v.shortDesc),
					Inline: true,
				})
			}
		}
	}

	embed.Fields = fields
	return embed
}

func settingResponse(CommandPrefix string, settings []Setting, sett *storage.GuildSettings) *discordgo.MessageEmbed {
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
				"CommandPrefix": CommandPrefix,
			}),
		Timestamp: "",
		Color:     15844367, //GOLD
		Image:     nil,
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
	}

	fields := make([]*discordgo.MessageEmbedField, len(settings))
	for i, v := range settings {
		fields[i] = &discordgo.MessageEmbedField{
			Name:   v.name,
			Value:  sett.LocalizeMessage(v.shortDesc),
			Inline: true,
		}
	}

	embed.Fields = fields
	return &embed
}

func (bot *Bot) statsResponse(sett *storage.GuildSettings) *discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Title",
			Other: "Bot Stats",
		}),
		Description: "",
		Timestamp:   time.Now().Format(ISO8601),
		Color:       2067276, //DARK GREEN
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
	}

	version, commit := broker.GetVersionAndCommit(bot.RedisInterface.client)
	totalGuilds := broker.GetGuildCounter(bot.RedisInterface.client, version)
	totalGames := broker.GetActiveGames(bot.RedisInterface.client)

	fields := make([]*discordgo.MessageEmbedField, 4)
	fields[0] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Guilds",
			Other: "Total Guilds",
		}),
		Value:  fmt.Sprintf("%d", totalGuilds),
		Inline: true,
	}
	fields[1] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Games",
			Other: "Active Games",
		}),
		Value:  fmt.Sprintf("%d", totalGames),
		Inline: true,
	}
	fields[2] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Version",
			Other: "Version",
		}),
		Value:  version,
		Inline: true,
	}
	fields[3] = &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.statsResponse.Commit",
			Other: "Commit",
		}),
		Value:  commit,
		Inline: true,
	}

	embed.Fields = fields
	return &embed
}

// TODO:
func (bot *Bot) gameStateResponse(dgs *DiscordGameState, sett *storage.GuildSettings) *discordgo.MessageEmbed {
	// we need to generate the messages based on the state of the game
	messages := map[game.Phase]func(dgs *DiscordGameState, emojis AlivenessEmojis, sett *storage.GuildSettings) *discordgo.MessageEmbed{
		game.MENU:    menuMessage,
		game.LOBBY:   lobbyMessage,
		game.TASKS:   gamePlayMessage,
		game.DISCUSS: gamePlayMessage,
	}
	return messages[dgs.AmongUsData.Phase](dgs, bot.StatusEmojis, sett)
}

func lobbyMetaEmbedFields(room, region string, playerCount int, linkedPlayers int, sett *storage.GuildSettings) []*discordgo.MessageEmbedField {
	gameInfoFields := make([]*discordgo.MessageEmbedField, 0)
	if room != "" {
		gameInfoFields = append(gameInfoFields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.RoomCode",
				Other: "🔒 ROOM CODE",
			}),
			Value:  fmt.Sprintf("%s", room),
			Inline: false,
		})
	}
	if region != "" {
		gameInfoFields = append(gameInfoFields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.Region",
				Other: "🌎 REGION",
			}),
			Value:  fmt.Sprintf("%s", region),
			Inline: false,
		})
	}

	//necessary with the latest checks for linked players
	//probably still broken, though -_-
	if linkedPlayers > playerCount {
		linkedPlayers = playerCount
	}
	gameInfoFields = append(gameInfoFields, &discordgo.MessageEmbedField{
		Name: sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMetaEmbedFields.PlayersLinked",
			Other: "Players Linked",
		}),
		Value:  fmt.Sprintf("%v/%v", linkedPlayers, playerCount),
		Inline: false,
	})

	return gameInfoFields
}

func menuMessage(dgs *DiscordGameState, emojis AlivenessEmojis, sett *storage.GuildSettings) *discordgo.MessageEmbed {

	color := 15158332 //red
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
			Other: "❌**No capture linked! Click the link in your DMs to connect!**❌",
		})
		footer = nil
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
		Fields:      nil,
	}
	return &msg
}

func lobbyMessage(dgs *DiscordGameState, emojis AlivenessEmojis, sett *storage.GuildSettings) *discordgo.MessageEmbed {
	//gameInfoFields[2] = &discordgo.MessageEmbedField{
	//	Name:   "\u200B",
	//	Value:  "\u200B",
	//	Inline: false,
	//}
	room, region := dgs.AmongUsData.GetRoomRegion()
	gameInfoFields := lobbyMetaEmbedFields(room, region, dgs.AmongUsData.GetNumDetectedPlayers(), dgs.GetCountLinked(), sett)

	listResp := dgs.ToEmojiEmbedFields(emojis, sett)
	listResp = append(gameInfoFields, listResp...)

	color := 15158332 //red
	desc := ""
	if dgs.Linked {
		desc = dgs.makeDescription(sett)
		color = 3066993
	} else {
		desc = sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.lobbyMessage.notLinked.Description",
			Other: "❌**No capture linked! Click the link in your DMs to connect!**❌",
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
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
		Fields:    listResp,
	}
	return &msg
}

func gamePlayMessage(dgs *DiscordGameState, emojis AlivenessEmojis, sett *storage.GuildSettings) *discordgo.MessageEmbed {
	//send empty fields because we don't need to display those fields during the game...
	gameInfoFields := lobbyMetaEmbedFields("", "", dgs.AmongUsData.GetNumDetectedPlayers(), dgs.GetCountLinked(), sett)
	listResp := dgs.ToEmojiEmbedFields(emojis, sett)
	listResp = append(gameInfoFields, listResp...)
	var color int

	phase := dgs.AmongUsData.GetPhase()

	switch phase {
	case game.TASKS:
		color = 3447003 //BLUE
	case game.DISCUSS:
		color = 10181046 //PURPLE
	default:
		color = 15158332 //RED
	}

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       sett.LocalizeMessage(phase.ToLocale()),
		Description: dgs.makeDescription(sett),
		Timestamp:   time.Now().Format(ISO8601),
		Color:       color,
		Footer:      nil,
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
		Fields:      listResp,
	}

	return &msg
}

func (dgs *DiscordGameState) makeDescription(sett *storage.GuildSettings) string {
	buf := bytes.NewBuffer([]byte{})
	if !dgs.Running {
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.makeDescription.GameNotRunning",
			Other: "\n⚠**Bot is Paused!**⚠\n\n",
		}))
	}

	author := dgs.GameStateMsg.LeaderID
	if author != "" {
		buf.WriteString(sett.LocalizeMessage(&i18n.Message{
			ID:    "responses.makeDescription.author",
			Other: "<@{{.author}}> is running an Among Us game!\nThe game is happening in ",
		},
			map[string]interface{}{
				"author": author,
			}))
	}

	buf.WriteString(dgs.Tracking.ToDescString(sett))

	return buf.String()
}

func extractUserIDFromMention(mention string) (string, error) {
	//nickname format
	if strings.HasPrefix(mention, "<@!") && strings.HasSuffix(mention, ">") {
		return mention[3 : len(mention)-1], nil
		//non-nickname format
	} else if strings.HasPrefix(mention, "<@") && strings.HasSuffix(mention, ">") {
		return mention[2 : len(mention)-1], nil
	} else {
		return "", errors.New("mention does not conform to the correct format")
	}
}

func extractRoleIDFromMention(mention string) (string, error) {
	//role is formatted <&123456>
	if strings.HasPrefix(mention, "<@&") && strings.HasSuffix(mention, ">") {
		return mention[3 : len(mention)-1], nil
	} else {
		return "", errors.New("mention does not conform to the correct format")
	}
}
