package discord

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/denverquane/amongusdiscord/game"
)

func helpResponse(CommandPrefix string) string {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("Among Us Bot command reference:\n")
	buf.WriteString("Having issues or have suggestions? Join the discord at <https://discord.gg/ZkqZSWF>!\n")
	buf.WriteString(fmt.Sprintf("`%s help` or `%s h`: Print help info and command usage.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s new` or `%s n`: Start the game in this text channel. Accepts room code and region as arguments. Ex: `.au new CODE eu`. Also works for restarting.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s end` or `%s e`: End the game entirely, and stop tracking players. Unmutes all and resets state.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s track` or `%s t`: Instruct bot to only use the provided voice channel for automute. Ex: `%s t <vc_name>`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s link` or `%s l`: Manually link a player to their in-game name or color. Ex: `%s l @player cyan` or `%s l @player bob`\n", CommandPrefix, CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s unlink` or `%s u`: Manually unlink a player. Ex: `%s u @player`\n", CommandPrefix, CommandPrefix, CommandPrefix))
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
		if player.IsLinked() {
			emoji := guild.StatusEmojis[player.IsAlive()][player.GetColor()]
			unsorted[player.GetColor()] = &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("%s", player.GetPlayerName()),
				Value:  fmt.Sprintf("%s <@!%s>", emoji.FormatForInline(), player.GetID()),
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

			guild.Tracking.AddTrackedChannel(c.ID, c.Name, forGhosts)

			log.Println(fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute (for ghosts? %v)!", c.Name, forGhosts))
			return fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute (for ghosts? %v)!", c.Name, forGhosts)
		}
	}
	return fmt.Sprintf("No channel found by the name %s!\n", channelName)
}

func (guild *GuildState) linkPlayerResponse(args []string) {

	userID, err := extractUserIDFromMention(args[0])
	if err != nil {
		log.Printf("Invalid mention format for \"%s\"", args[0])
	}

	combinedArgs := strings.ToLower(strings.Join(args[1:], ""))

	if game.IsColorString(combinedArgs) {
		playerData := guild.AmongUsData.GetByColor(combinedArgs)
		if playerData != nil {
			if v, ok := guild.UserData[userID]; ok {
				v.SetPlayerData(playerData)
				guild.UserData[userID] = v
				log.Println("Correlated and linked user by color")
			}
			log.Printf("No player was found with id %s\n", userID)
		}
		return
	} else {
		playerData := guild.AmongUsData.GetByName(combinedArgs)
		if playerData != nil {
			if v, ok := guild.UserData[userID]; ok {
				v.SetPlayerData(playerData)
				guild.UserData[userID] = v
				log.Println("Correlated and linked user by name")
			}
			log.Printf("No player was found with id %s\n", userID)
		}
	}
}

// TODO:
func gameStateResponse(guild *GuildState) *discordgo.MessageEmbed {
	// we need to generate the messages based on the state of the game
	messages := map[game.Phase]func(guild *GuildState) *discordgo.MessageEmbed{
		game.LOBBY:   lobbyMessage,
		game.TASKS:   gamePlayMessage,
		game.DISCUSS: gamePlayMessage,
	}
	return messages[guild.AmongUsData.GetPhase()](guild)
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

func lobbyMetaEmbedFields(tracking Tracking, room, region string) []*discordgo.MessageEmbedField {
	str := tracking.ToStatusString()
	gameInfoFields := make([]*discordgo.MessageEmbedField, 3)
	gameInfoFields[0] = &discordgo.MessageEmbedField{
		Name:   "room Code",
		Value:  fmt.Sprintf("%s", room),
		Inline: true,
	}
	gameInfoFields[1] = &discordgo.MessageEmbedField{
		Name:   "region",
		Value:  fmt.Sprintf("%s", region),
		Inline: true,
	}
	gameInfoFields[2] = &discordgo.MessageEmbedField{
		Name:   "Tracking",
		Value:  str,
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
	//buf.WriteString(fmt.Sprintf("\n%s %s\n", padToLength("room Code", PaddedLen), padToLength("region", PaddedLen))) // maybe this is a toggle?
	//uf.WriteString(fmt.Sprintf("**%s** **%s**\n", padToLength(g.room, PaddedLen), padToLength(g.region, PaddedLen)))

	//gameInfoFields[2] = &discordgo.MessageEmbedField{
	//	Name:   "\u200B",
	//	Value:  "\u200B",
	//	Inline: false,
	//}
	room, region := g.AmongUsData.GetRoomRegion()
	gameInfoFields := lobbyMetaEmbedFields(g.Tracking, room, region)

	listResp := g.playerListResponse()
	listResp = append(gameInfoFields, listResp...)
	//if len(listResp) > 0 {
	//	buf.WriteString(fmt.Sprintf("\nTracked Player List:\n"))
	//	buf.WriteString(listResp)
	//}

	alarmFormatted := ":x:"
	if v, ok := g.SpecialEmojis["alarm"]; ok {
		alarmFormatted = v.FormatForInline()
	}
	desc := ""
	if g.LinkCode == "" {
		desc = "Successfully linked to capture!"
	} else {
		desc = fmt.Sprintf("%s**No capture linked! Enter the code `%s` in your capture to connect!**%s", alarmFormatted, g.LinkCode, alarmFormatted)
	}

	msg := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       "Lobby is Open!",
		Description: desc,
		Timestamp:   "",
		Footer: &discordgo.MessageEmbedFooter{
			Text:         "React to this message with your in-game color! (or ❌ to leave)",
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
	room, region := guild.AmongUsData.GetRoomRegion()
	gameInfoFields := lobbyMetaEmbedFields(guild.Tracking, room, region)
	listResp := guild.playerListResponse()
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
		Title:       "Game is Running",
		Description: fmt.Sprintf("Current Phase: %s", phase.ToString()),
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
