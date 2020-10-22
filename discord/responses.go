package discord

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/locale"
	
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func helpResponse(version, CommandPrefix string) string {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.Title",
			Other: "Among Us Bot Commands (v{{.version}}):\n",
		},
		map[string]interface{}{
			"version": version,
		}))
	buf.WriteString(locale.LocalizeSimpleMessage(&i18n.Message{
			ID:    "responses.helpResponse.SubTitle",
			Other: "Having issues or have suggestions? Join the discord at <https://discord.gg/ZkqZSWF>!\n",
		}))
	buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.about.help",
			Other: "`{{.CommandPrefix}} help` or `{{.CommandPrefix}} h`: Print help info and command usage.\n",
		},
		map[string]interface{}{
			"CommandPrefix": CommandPrefix,
		}))
	buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.about.new",
			Other: "`{{.CommandPrefix}} new` or `{{.CommandPrefix}} n`: Start the game in this text channel. Accepts room code and region as arguments. Ex: `{{.CommandPrefix}} new CODE eu`. Also works for restarting.\n",
		},
		map[string]interface{}{
			"CommandPrefix": CommandPrefix,
		}))
	buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.about.refresh",
			Other: "`{{.CommandPrefix}} refresh` or `{{.CommandPrefix}} r`: Remake the bot's status message entirely, in case it ends up too far up in the chat.\n",
		},
		map[string]interface{}{
			"CommandPrefix": CommandPrefix,
		}))
	buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.about.end",
			Other: "`{{.CommandPrefix}} end` or `{{.CommandPrefix}} e`: End the game entirely, and stop tracking players. Unmutes all and resets state.\n",
		},
		map[string]interface{}{
			"CommandPrefix": CommandPrefix,
		}))
	buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.about.track",
			Other: "`{{.CommandPrefix}} track` or `{{.CommandPrefix}} t`: Instruct bot to only use the provided voice channel for automute. Ex: {{.CommandPrefix}}s t <vc_name>`\n",
		},
		map[string]interface{}{
			"CommandPrefix": CommandPrefix,
		}))
	buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.about.link",
			Other: "`{{.CommandPrefix}} link` or `{{.CommandPrefix}} l`: Manually link a player to their in-game name or color. Ex: `{{.CommandPrefix}} l @player cyan` or `{{.CommandPrefix}} l @player bob`\n",
		},
		map[string]interface{}{
			"CommandPrefix": CommandPrefix,
		}))
	buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.about.unlink",
			Other: "`{{.CommandPrefix}} unlink` or `{{.CommandPrefix}} u`: Manually unlink a player. Ex: {{.CommandPrefix}}s u @player`\n",
		},
		map[string]interface{}{
			"CommandPrefix": CommandPrefix,
		}))
	buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.about.settings",
			Other: "`{{.CommandPrefix}} settings` or `{{.CommandPrefix}} s`: View and change settings for the bot, such as the command prefix or mute behavior\n",
		},
		map[string]interface{}{
			"CommandPrefix": CommandPrefix,
		}))
	buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.about.force",
			Other: "`{{.CommandPrefix}} force` or `{{.CommandPrefix}} f`: Force a transition to a stage if you encounter a problem in the state. Ex: `{{.CommandPrefix}} f task` or `{{.CommandPrefix}} f d`(discuss)\n",
		},
		map[string]interface{}{
			"CommandPrefix": CommandPrefix,
		}))
	buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.helpResponse.about.pause",
			Other: "`{{.CommandPrefix}} pause` or `{{.CommandPrefix}} p`: Pause the bot, and don't let it automute anyone until unpaused. **will not un-mute muted players, be careful!**\n",
		},
		map[string]interface{}{
			"CommandPrefix": CommandPrefix,
		}))

	return buf.String()
}

func (guild *GuildState) trackChannelResponse(channelName string, allChannels []*discordgo.Channel, forGhosts bool) string {
	for _, c := range allChannels {
		if (strings.ToLower(c.Name) == strings.ToLower(channelName) || c.ID == channelName) && c.Type == 2 {

			guild.Tracking.AddTrackedChannel(c.ID, c.Name, forGhosts)

			log.Println(fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute (for ghosts? %v)!", c.Name, forGhosts))
			return fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute (for ghosts? %v)!", c.Name, forGhosts)
		}
	}
	return fmt.Sprintf("No channel found by the name %s!\n", channelName)
}

func (guild *GuildState) linkPlayerResponse(s *discordgo.Session, GuildID string, args []string) {

	g, err := s.State.Guild(guild.PersistentGuildData.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	userID := getMemberFromString(s, GuildID, args[0])
	if userID == "" {
		log.Printf("Sorry, I don't know who `%s` is. You can pass in ID, username, username#XXXX, nickname or @mention", args[0])
	}

	_, added := guild.checkCacheAndAddUser(g, s, userID)
	if !added {
		log.Println("No users found in Discord for userID " + userID)
	}

	combinedArgs := strings.ToLower(strings.Join(args[1:], ""))

	if game.IsColorString(combinedArgs) {
		playerData := guild.AmongUsData.GetByColor(combinedArgs)
		if playerData != nil {
			found := guild.UserData.UpdatePlayerData(userID, playerData)
			if found {
				log.Printf("Successfully linked %s to a color\n", userID)
			} else {
				log.Printf("No player was found with id %s\n", userID)
			}
		}
		return
	} else {
		playerData := guild.AmongUsData.GetByName(combinedArgs)
		if playerData != nil {
			found := guild.UserData.UpdatePlayerData(userID, playerData)
			if found {
				log.Printf("Successfully linked %s by name\n", userID)
			} else {
				log.Printf("No player was found with id %s\n", userID)
			}
		}
	}
}

// TODO:
func gameStateResponse(guild *GuildState) *discordgo.MessageEmbed {
	// we need to generate the messages based on the state of the game
	messages := map[game.Phase]func(guild *GuildState) *discordgo.MessageEmbed{
		game.MENU:    menuMessage,
		game.LOBBY:   lobbyMessage,
		game.TASKS:   gamePlayMessage,
		game.DISCUSS: gamePlayMessage,
	}
	return messages[guild.AmongUsData.GetPhase()](guild)
}

func lobbyMetaEmbedFields(tracking *Tracking, room, region string, playerCount int, linkedPlayers int) []*discordgo.MessageEmbedField {
	str := tracking.ToStatusString()
	gameInfoFields := make([]*discordgo.MessageEmbedField, 4)
	gameInfoFields[0] = &discordgo.MessageEmbedField{
		Name:   locale.LocalizeSimpleMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.RoomCode",
				Other: "Room Code",
			}),
		Value:  fmt.Sprintf("%s", room),
		Inline: true,
	}
	gameInfoFields[1] = &discordgo.MessageEmbedField{
		Name:   locale.LocalizeSimpleMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.Region",
				Other: "Region",
			}),
		Value:  fmt.Sprintf("%s", region),
		Inline: true,
	}
	gameInfoFields[2] = &discordgo.MessageEmbedField{
		Name:   locale.LocalizeSimpleMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.Tracking",
				Other: "Tracking",
			}),
		Value:  str,
		Inline: true,
	}
	gameInfoFields[3] = &discordgo.MessageEmbedField{
		Name:   locale.LocalizeSimpleMessage(&i18n.Message{
				ID:    "responses.lobbyMetaEmbedFields.PlayersLinked",
				Other: "Players Linked",
			}),
		Value:  fmt.Sprintf("%v/%v", linkedPlayers, playerCount),
		Inline: false,
	}

	return gameInfoFields
}

// Thumbnail for the bot
var Thumbnail = discordgo.MessageEmbedThumbnail{
	URL:      "https://github.com/denverquane/amongusdiscord/blob/master/assets/botProfilePicture.jpg?raw=true",
	ProxyURL: "",
	Width:    200,
	Height:   200,
}

func menuMessage(g *GuildState) *discordgo.MessageEmbed {
	alarmFormatted := ":x:"
	if v, ok := g.SpecialEmojis["alarm"]; ok {
		alarmFormatted = v.FormatForInline()
	}
	color := 15158332 //red
	desc := ""
	if g.Linked {
		desc = g.makeDescription()
		color = 3066993
	} else {
		desc = locale.LocalizeMessage(&i18n.Message{
				ID:    "responses.menuMessage.notLinked.Description",
				Other: "{{.alarmFormattedStart}}**No capture linked! Click the link in your DMs to connect!**{{.alarmFormattedEnd}}",
			},
			map[string]interface{}{
				"alarmFormattedStart": alarmFormatted,
				"alarmFormattedEnd": alarmFormatted,
			})
	}

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       locale.LocalizeSimpleMessage(&i18n.Message{
				ID:    "responses.menuMessage.Title",
				Other: "Main Menu",
			}),
		Description: desc,
		Timestamp:   "",
		Footer:      nil,
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

func lobbyMessage(g *GuildState) *discordgo.MessageEmbed {
	//gameInfoFields[2] = &discordgo.MessageEmbedField{
	//	Name:   "\u200B",
	//	Value:  "\u200B",
	//	Inline: false,
	//}
	room, region := g.AmongUsData.GetRoomRegion()
	gameInfoFields := lobbyMetaEmbedFields(&g.Tracking, room, region, g.AmongUsData.NumDetectedPlayers(), g.UserData.GetCountLinked())

	listResp := g.UserData.ToEmojiEmbedFields(g.AmongUsData.NameColorMappings(), g.AmongUsData.NameAliveMappings(), g.StatusEmojis)
	listResp = append(gameInfoFields, listResp...)

	alarmFormatted := ":x:"
	if v, ok := g.SpecialEmojis["alarm"]; ok {
		alarmFormatted = v.FormatForInline()
	}
	color := 15158332 //red
	desc := ""
	if g.Linked {
		desc = g.makeDescription()
		color = 3066993
	} else {
		desc = locale.LocalizeMessage(&i18n.Message{
				ID:    "responses.lobbyMessage.notLinked.Description",
				Other: "{{.alarmFormattedStart}}**No capture linked! Click the link in your DMs to connect!**{{.alarmFormattedEnd}}",
			},
			map[string]interface{}{
				"alarmFormattedStart": alarmFormatted,
				"alarmFormattedEnd": alarmFormatted,
			})
	}

	emojiLeave := "❌"
	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       locale.LocalizeSimpleMessage(&i18n.Message{
				ID:    "responses.lobbyMessage.Title",
				Other: "Lobby",
			}),
		Description: desc,
		Timestamp:   "",
		Footer: &discordgo.MessageEmbedFooter{
			Text:  locale.LocalizeMessage(&i18n.Message{
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

func gamePlayMessage(guild *GuildState) *discordgo.MessageEmbed {
	// add the player list
	//guild.UserDataLock.Lock()
	room, region := guild.AmongUsData.GetRoomRegion()
	gameInfoFields := lobbyMetaEmbedFields(&guild.Tracking, room, region, guild.AmongUsData.NumDetectedPlayers(), guild.UserData.GetCountLinked())
	listResp := guild.UserData.ToEmojiEmbedFields(guild.AmongUsData.NameColorMappings(), guild.AmongUsData.NameAliveMappings(), guild.StatusEmojis)
	listResp = append(gameInfoFields, listResp...)
	//guild.UserDataLock.Unlock()
	var color int

	phase := guild.AmongUsData.GetPhase()

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
		Title:       locale.LocalizeSimpleMessage(phase.ToLocale()),
		Description: guild.makeDescription(),
		Timestamp:   "",
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

func (guild *GuildState) makeDescription() string {
	buf := bytes.NewBuffer([]byte{})
	if !guild.GameRunning {
		buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.makeDescription.GameNotRunning",
			Other: "\n**Bot is Paused! Unpause with `{{.CommandPrefix}} p`!**\n\n",
		},
		map[string]interface{}{
			"CommandPrefix": guild.PersistentGuildData.CommandPrefix,
		}))
	}

	author := guild.GameStateMsg.leaderID
	if author != "" {
		buf.WriteString(locale.LocalizeMessage(&i18n.Message{
			ID:    "responses.makeDescription.author",
			Other: "<@{{.author}}> is running an Among Us game!\nThe game is happening in ",
		},
		map[string]interface{}{
			"author": author,
		}))
	}

	if len(guild.Tracking.tracking) == 0 {
		buf.WriteString(locale.LocalizeSimpleMessage(&i18n.Message{
			ID:    "responses.makeDescription.anyVoiceChannel",
			Other: "any voice channel!",
		}))
	} else {
		t, err := guild.Tracking.FindAnyTrackedChannel(false)
		if err != nil {
			buf.WriteString(locale.LocalizeSimpleMessage(&i18n.Message{
				ID:    "responses.makeDescription.invalidVoiceChannel",
				Other: "an invalid voice channel!",
			}))
		} else {
			buf.WriteString(locale.LocalizeMessage(&i18n.Message{
				ID:    "responses.makeDescription.voiceChannelName",
				Other: "the **{{.channelName}}** voice channel!",
			},
			map[string]interface{}{
				"channelName": t.channelName,
			}))
		}
	}

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
