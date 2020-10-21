package discord

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/denverquane/amongusdiscord/storage"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/denverquane/amongusdiscord/game"
)

func helpResponse(version, CommandPrefix string, commands []Command) discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       fmt.Sprintf("AutoMuteUs Bot Commands (v%s):\n", version),
		Description: "Having issues or have suggestions? Join our discord at <https://discord.gg/ZkqZSWF>!",
		Timestamp:   "",
		Color:       15844367, //GOLD
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
	}

	fields := make([]*discordgo.MessageEmbedField, len(commands)-2)
	for i, v := range commands {
		if v.cmdType != Help && v.cmdType != Null {
			fields[i-1] = &discordgo.MessageEmbedField{
				Name:   v.command + " `" + CommandPrefix + " help " + v.command + "`",
				Value:  v.shortDesc,
				Inline: true,
			}
		}
	}

	embed.Fields = fields
	return embed
}

func settingResponse(settings []Setting) discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       "Settings",
		Description: "Available Settings",
		Timestamp:   "",
		Color:       15844367, //GOLD
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
	}

	fields := make([]*discordgo.MessageEmbedField, len(settings))
	for i, v := range settings {
		fields[i] = &discordgo.MessageEmbedField{
			Name:   v.name,
			Value:  v.shortDesc,
			Inline: true,
		}
	}

	embed.Fields = fields
	return embed
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

	g, err := s.State.Guild(GuildID)
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
				guild.userSettingsUpdateChannel <- storage.UserSettingsUpdate{
					UserID: userID,
					Type:   storage.GAME_NAME,
					Value:  playerData.Name,
				}
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
				guild.userSettingsUpdateChannel <- storage.UserSettingsUpdate{
					UserID: userID,
					Type:   storage.GAME_NAME,
					Value:  playerData.Name,
				}
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
		Name:   "Room Code",
		Value:  fmt.Sprintf("%s", room),
		Inline: true,
	}
	gameInfoFields[1] = &discordgo.MessageEmbedField{
		Name:   "Region",
		Value:  fmt.Sprintf("%s", region),
		Inline: true,
	}
	gameInfoFields[2] = &discordgo.MessageEmbedField{
		Name:   "Tracking",
		Value:  str,
		Inline: true,
	}
	gameInfoFields[3] = &discordgo.MessageEmbedField{
		Name:   "Players Linked",
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
		desc = fmt.Sprintf("%s**No capture linked! Click the link in your DMs to connect!**%s", alarmFormatted, alarmFormatted)
	}

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       "Main Menu",
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
		desc = fmt.Sprintf("%s**No capture linked! Click the link in your DMs to connect!**%s", alarmFormatted, alarmFormatted)
	}

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       "Lobby",
		Description: desc,
		Timestamp:   "",
		Footer: &discordgo.MessageEmbedFooter{
			Text:         "React to this message with your in-game color! (or ❌ to leave)",
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
		Title:       string(phase.ToString()),
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
		buf.WriteString("\n**Bot is Paused! Unpause with `" + guild.CommandPrefix() + " p`!**\n\n")
	}

	author := guild.GameStateMsg.leaderID
	if author != "" {
		buf.WriteString("<@" + author + "> is running an Among Us game!\nThe game is happening in ")
	}

	if len(guild.Tracking.tracking) == 0 {
		buf.WriteString("any voice channel!")
	} else {
		t, err := guild.Tracking.FindAnyTrackedChannel(false)
		if err != nil {
			buf.WriteString("an invalid voice channel!")
		} else {
			buf.WriteString("the **" + t.channelName + "** voice channel!")
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
