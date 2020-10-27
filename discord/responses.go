package discord

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/denverquane/amongusdiscord/game"
)

func helpResponse(version, CommandPrefix string, commands []Command) discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       fmt.Sprintf("AutoMuteUs Bot Commands (v%s):\n", version),
		Description: fmt.Sprintf("Having issues or have suggestions? Join our discord at <https://discord.gg/ZkqZSWF>!\nType `%s help <command>` to see more details on a command!", CommandPrefix),
		Timestamp:   "",
		Color:       15844367, //GOLD
		Image:       nil,
		Thumbnail:   nil,
		Video:       nil,
		Provider:    nil,
		Author:      nil,
	}

	fields := make([]*discordgo.MessageEmbedField, 0)
	for _, v := range commands {
		if !v.secret && v.cmdType != Help && v.cmdType != Null {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   v.command,
				Value:  v.shortDesc,
				Inline: true,
			})
		}
	}

	embed.Fields = fields
	return embed
}

func settingResponse(cp string, settings []Setting) discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		URL:         "",
		Type:        "",
		Title:       "Settings",
		Description: "Type `" + cp + " settings <setting>` to change a setting from those listed below",
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

// TODO:
func (bot *Bot) gameStateResponse(dgs *DiscordGameState) *discordgo.MessageEmbed {
	// we need to generate the messages based on the state of the game
	messages := map[game.Phase]func(dgs *DiscordGameState, emojis AlivenessEmojis) *discordgo.MessageEmbed{
		game.MENU:    menuMessage,
		game.LOBBY:   lobbyMessage,
		game.TASKS:   gamePlayMessage,
		game.DISCUSS: gamePlayMessage,
	}
	return messages[dgs.AmongUsData.Phase](dgs, bot.StatusEmojis)
}

func lobbyMetaEmbedFields(tracking TrackingChannel, room, region string, playerCount int, linkedPlayers int) []*discordgo.MessageEmbedField {
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
		Value:  tracking.ToStatusString(),
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

func menuMessage(dgs *DiscordGameState, emojis AlivenessEmojis) *discordgo.MessageEmbed {

	color := 15158332 //red
	desc := ""
	if dgs.Linked {
		desc = dgs.makeDescription()
		color = 3066993
	} else {
		desc = "❌**No capture linked! Click the link in your DMs to connect!**❌"
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

func lobbyMessage(dgs *DiscordGameState, emojis AlivenessEmojis) *discordgo.MessageEmbed {
	//gameInfoFields[2] = &discordgo.MessageEmbedField{
	//	Name:   "\u200B",
	//	Value:  "\u200B",
	//	Inline: false,
	//}
	room, region := dgs.GetRoomRegion()
	gameInfoFields := lobbyMetaEmbedFields(dgs.Tracking, room, region, dgs.GetNumDetectedPlayers(), dgs.GetCountLinked())

	listResp := dgs.ToEmojiEmbedFields(emojis)
	listResp = append(gameInfoFields, listResp...)

	color := 15158332 //red
	desc := ""
	if dgs.Linked {
		desc = dgs.makeDescription()
		color = 3066993
	} else {
		desc = fmt.Sprintf("❌**No capture linked! Click the link in your DMs to connect!**❌")
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

func gamePlayMessage(dgs *DiscordGameState, emojis AlivenessEmojis) *discordgo.MessageEmbed {
	// add the player list
	//guild.UserDataLock.Lock()
	room, region := dgs.GetRoomRegion()
	gameInfoFields := lobbyMetaEmbedFields(dgs.Tracking, room, region, dgs.GetNumDetectedPlayers(), dgs.GetCountLinked())
	listResp := dgs.ToEmojiEmbedFields(emojis)
	listResp = append(gameInfoFields, listResp...)
	//guild.UserDataLock.Unlock()
	var color int

	phase := dgs.GetPhase()

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
		Description: dgs.makeDescription(),
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

func (dgs *DiscordGameState) makeDescription() string {
	buf := bytes.NewBuffer([]byte{})
	if !dgs.Running {
		buf.WriteString("\n**Bot is Paused!**\n\n")
	}

	author := dgs.GameStateMsg.LeaderID
	if author != "" {
		buf.WriteString("<@" + author + "> is running an Among Us game!\nThe game is happening in ")
	}

	buf.WriteString(dgs.Tracking.ToDescString())

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
