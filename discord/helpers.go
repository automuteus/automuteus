package discord

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	galactus_client "github.com/automuteus/galactus/pkg/client"
	"github.com/automuteus/utils/pkg/game"
	"log"
	"math/rand"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type UserPatchParameters struct {
	GuildID  string
	Userdata UserData
	Deaf     bool
	Mute     bool
}

func getPhaseFromString(input string) game.Phase {
	if len(input) == 0 {
		return game.UNINITIALIZED
	}

	switch strings.ToLower(input) {
	case "lobby":
		fallthrough
	case "l":
		return game.LOBBY
	case "task":
		fallthrough
	case "t":
		fallthrough
	case "tasks":
		fallthrough
	case "game":
		fallthrough
	case "g":
		return game.TASKS
	case "discuss":
		fallthrough
	case "disc":
		fallthrough
	case "d":
		fallthrough
	case "discussion":
		return game.DISCUSS
	default:
		return game.UNINITIALIZED
	}
}

func getRoleFromString(galactus *galactus_client.GalactusClient, guildID string, input string) string {
	// find which role the User was referencing in their message
	// first check if is mentionned
	ID, err := extractRoleIDFromMention(input)
	if err == nil {
		return ID
	}
	roles, _ := galactus.GetGuildRoles(guildID)
	for _, role := range roles {
		if input == role.ID || input == strings.ToLower(role.Name) {
			return role.ID
		}
	}
	return ""
}

func generateConnectCode(guildID string) string {
	h := sha256.New()
	h.Write([]byte(guildID))

	// add some randomness
	h.Write([]byte(fmt.Sprintf("%f", rand.Float64())))
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil))[0:8])
}

var urlregex = regexp.MustCompile(`^http(?P<secure>s?)://(?P<host>[\w.-]+)(?::(?P<port>\d+))?/?$`)

func formCaptureURL(url, connectCode string) (hyperlink, minimalURL string) {
	if match := urlregex.FindStringSubmatch(url); match != nil {
		secure := match[urlregex.SubexpIndex("secure")] == "s"
		host := match[urlregex.SubexpIndex("host")]
		port := ":" + match[urlregex.SubexpIndex("port")]

		if port == ":" {
			if secure {
				port = ":443"
			} else {
				port = ":80"
			}
		}

		insecure := "?insecure"
		protocol := "http://"
		if secure {
			insecure = ""
			protocol = "https://"
		}

		hyperlink = fmt.Sprintf("aucapture://%s%s/%s%s", host, port, connectCode, insecure)
		minimalURL = fmt.Sprintf("%s%s%s", protocol, host, port)
	} else {
		hyperlink = "Invalid HOST provided (should resemble something like `http://localhost:8123`)"
		minimalURL = "Invalid HOST provided"
	}
	return
}

func mentionByUserID(userID string) string {
	return "<@!" + userID + ">"
}

func sendMessageDM(galactus *galactus_client.GalactusClient, userID string, message *discordgo.MessageEmbed) *discordgo.Message {
	dmChannel, err := galactus.CreateUserChannel(userID)
	if err != nil {
		log.Println(err)
	}
	m, err := galactus.SendChannelMessageEmbed(dmChannel.ID, message)
	if err != nil {
		log.Println(err)
	}
	return m
}

func addReaction(galactus *galactus_client.GalactusClient, channelID, messageID, emojiID string) {
	err := galactus.AddReaction(channelID, messageID, emojiID)
	if err != nil {
		log.Println(err)
	}
}

func removeAllReactions(galactus *galactus_client.GalactusClient, channelID, messageID string) {
	err := galactus.RemoveAllReactions(channelID, messageID)
	if err != nil {
		log.Println(err)
	}
}

func removeReaction(galactus *galactus_client.GalactusClient, channelID, messageID, emojiNameOrID, userID string) {
	err := galactus.RemoveReaction(channelID, messageID, emojiNameOrID, userID)
	if err != nil {
		log.Println(err)
	}
}

func matchIDCode(connectCode string, matchID int64) string {
	return fmt.Sprintf("%s:%d", connectCode, matchID)
}
