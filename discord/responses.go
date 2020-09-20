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
	buf.WriteString(fmt.Sprintf("`%s start` or `%s s`: Start the game in this text channel. Accepts Room code and Region as arguments. Ex: `.au start CODE eu`. Also works for restarting.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s track` or `%s t`: Instruct bot to only use the provided voice channel for automute. Ex: `%s t <vc_name>`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s link` or `%s l`: Manually link a player to their in-game name or color. Ex: `%s l @player cyan` or `%s l @player bob`\n", CommandPrefix, CommandPrefix, CommandPrefix, CommandPrefix))
	return buf.String()
}

//TODO Kaividian mentioned this format might be weird? How to properly @mention a player? <!@ vs <@ for ex...
func (guild *GuildState) playerListResponse(users map[string]UserData) string {
	buf := bytes.NewBuffer([]byte{})

	sorted := make([]string, 12)

	//buf.WriteString("Player List:\n")
	for _, player := range users {
		if player.auData != nil {
			emoji := guild.StatusEmojis[player.auData.IsAlive][player.auData.Color]
			sorted[player.auData.Color] = fmt.Sprintf("%s <@!%s>: %s\n", emoji.FormatForInline(), player.user.userID, player.auData.Name)
		}

	}
	for i := 0; i < 12; i++ {
		if sorted[i] != "" {
			buf.WriteString(sorted[i])
		}
	}
	return buf.String()
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
		return str
	}

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
	//guild.AmongUsDataLock.Lock()
	//defer guild.AmongUsDataLock.Unlock()

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
func gameStateResponse(guild *GuildState) string {
	// we need to generate the messages based on the state of the game
	messages := map[game.Phase]func(guild *GuildState) string{
		game.LOBBY:   lobbyMessage,
		game.TASKS:   gamePlayMessage,
		game.DISCUSS: gamePlayMessage,
	}
	return messages[guild.GamePhase](guild)
}

func lobbyMessage(g *GuildState) string {
	buf := bytes.NewBuffer([]byte{})

	//buf.WriteString("Lobby is open!\n")
	if g.LinkCode != "" {
		alarmFormatted := ":x:"
		if v, ok := g.SpecialEmojis["alarm"]; ok {
			alarmFormatted = v.FormatForInline()
		}

		buf.WriteString(fmt.Sprintf("%s **No capture is linked! Use the guildID %s to connect!** %s\n", alarmFormatted, g.LinkCode, alarmFormatted))
	}
	buf.WriteString(fmt.Sprintf("\nLobby Code: **%s** Region: **%s**\n", g.Room, g.Region)) // maybe this is a toggle?
	buf.WriteString("Tracking: **")
	if len(g.Tracking) == 0 {
		buf.WriteString(fmt.Sprintf("Any Voice channel!**\n"))
	} else {
		i := 0
		for _, v := range g.Tracking {
			buf.WriteString(fmt.Sprintf("%s", v.channelName))
			if v.forGhosts {
				buf.WriteString("(ghosts)")
			}
			if i < len(g.Tracking)-1 {
				buf.WriteString(" or ")
			}
			i++
		}
		buf.WriteString("**\n")
	}

	listResp := g.playerListResponse(g.UserData)
	if len(listResp) > 0 {
		buf.WriteString(fmt.Sprintf("\nTracked Player List:\n"))
		buf.WriteString(listResp)
	}

	buf.WriteString("\nReact to this message with your in-game color once you join the game!")

	return buf.String()
}

func gamePlayMessage(guild *GuildState) string {
	buf := bytes.NewBuffer([]byte{})

	buf.WriteString("Game is running!\n\n")
	// add the player list
	//guild.UserDataLock.Lock()
	buf.WriteString(guild.playerListResponse(guild.UserData))
	//guild.UserDataLock.Unlock()

	//guild.AmongUsDataLock.RLock()
	buf.WriteString(fmt.Sprintf("\nCurrent Phase: **%s**\n", guild.GamePhase.ToString()))
	//guild.AmongUsDataLock.RUnlock()

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
