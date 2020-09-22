package discord

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"strings"

	"github.com/denverquane/amongusdiscord/game"
)

func helpResponse(CommandPrefix string) string {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("Among Us Bot command reference:\n")
	buf.WriteString("Having issues or have suggestions? Join the discord at https://discord.gg/ZkqZSWF !\n")
	buf.WriteString(fmt.Sprintf("`%s help` or `%s h`: Print help info and command usage.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s new` or `%s n`: Start the game in this text channel. Accepts Room code and Region as arguments. Ex: `.au new CODE eu`. Also works for restarting.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s end` or `%s e`: End the game entirely, and stop tracking players. Unmutes all and resets state.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s track` or `%s t`: Instruct bot to only use the provided voice channel for automute. Ex: `%s t <vc_name>`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s link` or `%s l`: Manually link a player to their in-game name or color. Ex: `%s l @player cyan` or `%s l @player bob`\n", CommandPrefix, CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s force` or `%s f`: Force a transition to a stage if you encounter a problem in the state. Ex: `%s f task` or `%s f d`(discuss)\n", CommandPrefix, CommandPrefix, CommandPrefix, CommandPrefix))

	return buf.String()
}

//TODO Kaividian mentioned this format might be weird? How to properly @mention a player? <!@ vs <@ for ex...
func (guild *GuildState) playerListResponse() []*discordgo.MessageEmbedField {
	unsorted := make([]*discordgo.MessageEmbedField, 12)

	num := 0
	//buf.WriteString("Player List:\n")
	guild.UserDataLock.RLock()
	for _, player := range guild.UserData {
		if player.auData != nil {
			emoji := guild.StatusEmojis[player.auData.IsAlive][player.auData.Color]
			unsorted[player.auData.Color] = &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("%s", player.auData.Name),
				Value:  fmt.Sprintf("%s <@!%s>", emoji.FormatForInline(), player.user.userID),
				Inline: true,
			}
			num++
		}
	}
	guild.UserDataLock.RUnlock()
	sorted := make([]*discordgo.MessageEmbedField, num)
	num = 0
	for i := 0; i < 12; i++ {
		if unsorted[i] != nil {
			sorted[num] = unsorted[i]
			num++
		}
	}
	return sorted
}

func (guild *GuildState) trackChannelResponse(channelName string, allChannels []*discordgo.Channel, forGhosts bool) string {
	for _, c := range allChannels {
		if (strings.ToLower(c.Name) == strings.ToLower(channelName) || c.ID == channelName) && c.Type == 2 {

			//TODO check duplicates (for logging)
			guild.Tracking[c.ID] = Tracking{
				channelID:   c.ID,
				channelName: c.Name,
				forGhosts:   forGhosts,
			}
			log.Println(fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute (for ghosts? %v)!", c.Name, forGhosts))
			return fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute (for ghosts? %v)!", c.Name, forGhosts)
		}
	}
	return fmt.Sprintf("No channel found by the name %s!\n", channelName)
}

func (guild *GuildState) linkPlayerResponse(args []string, allAuData map[string]*AmongUserData) string {

	userID, err := extractUserIDFromMention(args[0])
	if err != nil {
		return fmt.Sprintf("Invalid mention format for \"%s\"", args[0])
	}

	combinedArgs := strings.ToLower(strings.Join(args[1:], ""))

	if IsColorString(combinedArgs) {
		str, _ := guild.matchByColor(userID, combinedArgs, allAuData)
		log.Println(str)
		return str
	}

	guild.UserDataLock.Lock()
	defer guild.UserDataLock.Unlock()

	inGameName := combinedArgs
	for name, auData := range allAuData {
		name = strings.ToLower(strings.ReplaceAll(name, " ", ""))
		log.Println(name)
		if name == inGameName {
			if user, ok := guild.UserData[userID]; ok {
				user.auData = auData //point to the single copy in memory
				guild.UserData[userID] = user
				log.Printf("Linked %s to %s", args[0], user.auData.ToString())
				return fmt.Sprintf("Successfully linked player via Name!")
			}
			return fmt.Sprintf("No user found with userID %s", userID)
		}
	}
	return fmt.Sprintf(":x: No in-game name was found matching %s!\n", inGameName)
}

func (guild *GuildState) matchByColor(userID, text string, allAuData map[string]*AmongUserData) (string, bool) {
	guild.UserDataLock.Lock()
	defer guild.UserDataLock.Unlock()

	for _, auData := range allAuData {
		if GetColorStringForInt(auData.Color) == strings.ToLower(text) {
			if user, ok := guild.UserData[userID]; ok {
				user.auData = auData //point to the single copy in memory
				//user.visualTrack = true
				guild.UserData[userID] = user
				log.Printf("Linked %s to %s", userID, user.auData.ToString())
				return fmt.Sprintf("Successfully linked player via Color!"), true
			}
			return fmt.Sprintf("No user found with userID %s", userID), false
		}
	}
	return fmt.Sprintf(":x: No in-game player data was found matching that color!\n"), false
}

// TODO:
func gameStateResponse(guild *GuildState) *discordgo.MessageEmbed {
	// we need to generate the messages based on the state of the game
	messages := map[game.Phase]func(guild *GuildState) *discordgo.MessageEmbed{
		game.LOBBY:   lobbyMessage,
		game.TASKS:   gamePlayMessage,
		game.DISCUSS: gamePlayMessage,
	}
	return messages[guild.GamePhase](guild)
}

//func padToLength(input string, length int) string {
//	diff := length - len(input)
//	if diff > 0 {
//		return input + strings.Repeat("  ", diff)
//	}
//	return input
//}
//
//const PaddedLen = 20

func lobbyMetaEmbedFields(tracking map[string]Tracking, room, region string) []*discordgo.MessageEmbedField {
	buf := bytes.NewBuffer([]byte(""))
	if len(tracking) == 0 {
		buf.WriteString(fmt.Sprintf("Any Voice Channel"))
	} else {
		i := 0
		for _, v := range tracking {
			buf.WriteString(fmt.Sprintf("%s", v.channelName))
			if v.forGhosts {
				buf.WriteString("(ghosts)")
			}
			if i < len(tracking)-1 {
				buf.WriteString(" or ")
			}
			i++
		}
	}
	gameInfoFields := make([]*discordgo.MessageEmbedField, 3)
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
		Value:  buf.String(),
		Inline: false,
	}
	return gameInfoFields
}

var Thumbnail = discordgo.MessageEmbedThumbnail{
	URL:      "https://github.com/denverquane/amongusdiscord/blob/master/assets/botProfilePicture.jpg?raw=true",
	ProxyURL: "",
	Width:    200,
	Height:   200,
}

func lobbyMessage(g *GuildState) *discordgo.MessageEmbed {
	//buf.WriteString("Lobby is open!\n")
	//if g.LinkCode != "" {
	//	alarmFormatted := ":x:"
	//	if v, ok := g.SpecialEmojis["alarm"]; ok {
	//		alarmFormatted = v.FormatForInline()
	//	}
	//
	//	buf.WriteString(fmt.Sprintf("%s **No capture is linked! Use the guildID %s to connect!** %s\n", alarmFormatted, g.LinkCode, alarmFormatted))
	//}
	//buf.WriteString(fmt.Sprintf("\n%s %s\n", padToLength("Room Code", PaddedLen), padToLength("Region", PaddedLen))) // maybe this is a toggle?
	//uf.WriteString(fmt.Sprintf("**%s** **%s**\n", padToLength(g.Room, PaddedLen), padToLength(g.Region, PaddedLen)))

	//gameInfoFields[2] = &discordgo.MessageEmbedField{
	//	Name:   "\u200B",
	//	Value:  "\u200B",
	//	Inline: false,
	//}
	gameInfoFields := lobbyMetaEmbedFields(g.Tracking, g.Room, g.Region)

	listResp := g.playerListResponse()
	listResp = append(gameInfoFields, listResp...)
	//if len(listResp) > 0 {
	//	buf.WriteString(fmt.Sprintf("\nTracked Player List:\n"))
	//	buf.WriteString(listResp)
	//}

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       "Lobby is Open!",
		Description: "",
		Timestamp:   "",
		Footer: &discordgo.MessageEmbedFooter{
			Text:         "React to this message with your in-game color once you join the game!",
			IconURL:      "",
			ProxyIconURL: "",
		},
		Color:     3066993, //GREEN
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
	gameInfoFields := lobbyMetaEmbedFields(guild.Tracking, guild.Room, guild.Region)
	listResp := guild.playerListResponse()
	listResp = append(gameInfoFields, listResp...)
	//guild.UserDataLock.Unlock()
	var color int

	switch guild.GamePhase {
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
		Title:       "Game is Running",
		Description: fmt.Sprintf("Current Phase: %s", guild.GamePhase.ToString()),
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
