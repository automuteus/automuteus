package discord

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func helpResponse() string {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("Among Us Bot command reference:\n")
	buf.WriteString("Having issues or have suggestions? Join the discord at https://discord.gg/ZkqZSWF !\n")
	buf.WriteString(fmt.Sprintf("`%s help` (`%s h`): Print help info and command usage.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s list` (`%s l`): Print the currently tracked players, and their in-game status (Beta).\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s track` (`%s t`): Tell Bot to use a single voice channel for mute/unmute, and ignore other players. Ex: `%s t Voice channel name`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s bcast` (`%s b`): Tell Bot to broadcast the room code and region. Ex: `%s b ABCD asia` or `%s b ABCD na`\n", CommandPrefix, CommandPrefix, CommandPrefix, CommandPrefix))
	return buf.String()
}

func (guild *GuildState) broadcastResponse(args []string) (string, error) {
	buf := bytes.NewBuffer([]byte{})
	code, region := "", ""
	//just the room code
	code = strings.ToUpper(args[0])

	if len(args) > 1 {
		region = strings.ToLower(args[1])
		switch region {
		case "na":
			fallthrough
		case "us":
			fallthrough
		case "usa":
			fallthrough
		case "north":
			region = "North America"
		case "eu":
			fallthrough
		case "europe":
			region = "Europe"
		case "as":
			fallthrough
		case "asia":
			region = "Asia"
		}
	}
	guild.UserDataLock.RLock()
	for _, player := range guild.UserData {
		if player.tracking {
			buf.WriteString(fmt.Sprintf("<@!%s> ", player.user.userID))
		}
	}
	guild.UserDataLock.RUnlock()
	buf.WriteString(fmt.Sprintf("\nThe Room Code is **%s**\n", code))

	if region == "" {
		buf.WriteString("I wasn't told the Region, though :cry:")
	} else {
		buf.WriteString(fmt.Sprintf("The Region is **%s**\n", region))
	}
	return buf.String(), nil
}

//TODO update original message, not post new one
//TODO delete player messages relating to this
//TODO print the tracked again
func playerListResponse(users map[string]UserData) string {
	buf := bytes.NewBuffer([]byte{})

	buf.WriteString("Player List:\n")
	for _, player := range users {
		if player.tracking {
			if player.auData != nil {
				emoji := AlivenessColoredEmojis[player.auData.IsAlive][player.auData.Color]
				buf.WriteString(fmt.Sprintf("<:%s:%s> <@!%s>: %s\n", emoji.Name, emoji.ID, player.user.userID, player.auData.Name))
			} else {
				buf.WriteString(fmt.Sprintf(":x: <@!%s>: Use `.au link @%s <in-game name>`\n", player.user.userID, player.user.userName))
			}

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
			return fmt.Sprintf("Now tracking \"%s\" Voice Channel for Automute (for ghosts? %v)!", c.Name, forGhosts)
		}
	}
	return fmt.Sprintf("No channel found by the name %s!\n", channelName)
}

//TODO implement a separate way to link by color (color is more volatile)
//TODO implement a way to match in-game names to discord ones WITHOUT the link command
func (guild *GuildState) linkPlayerResponse(args []string, allAuData *[]AmongUserData) string {
	userID, err := extractUserIDFromMention(args[0])
	if err != nil {
		return fmt.Sprintf("Invalid mention format for \"%s\"", args[0])
	}

	inGameName := strings.ToLower(args[1])
	for color, auData := range *allAuData {
		log.Println(strings.ToLower(auData.Name))
		if strings.ToLower(auData.Name) == inGameName {
			if user, ok := guild.UserData[userID]; ok {
				user.auData = &(*allAuData)[color] //point to the single copy in memory
				guild.UserData[userID] = user
				log.Printf("Linked %s to %s", args[0], user.auData.ToString())
				return fmt.Sprintf("Successfully linked player to existing game data!")
			}
			return fmt.Sprintf("No user found with userID %s", userID)
		}
	}
	return fmt.Sprintf(":x: No in-game name was found matching %s!\n", inGameName)
}

// TODO:
func gameStateResponse(guild *GuildState) string {
	return guild.ToString()
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
