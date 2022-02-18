package discord

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

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

func sendMessageEmbed(s *discordgo.Session, channelID string, message *discordgo.MessageEmbed) *discordgo.Message {
	msg, err := s.ChannelMessageSendEmbed(channelID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

func editMessageEmbed(s *discordgo.Session, channelID string, messageID string, message *discordgo.MessageEmbed) *discordgo.Message {
	msg, err := s.ChannelMessageEditEmbed(channelID, messageID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

func deleteMessage(s *discordgo.Session, channelID string, messageID string) {
	err := s.ChannelMessageDelete(channelID, messageID)
	if err != nil {
		log.Println(err)
	}
}

func addReaction(s *discordgo.Session, channelID, messageID, emojiID string) {
	err := s.MessageReactionAdd(channelID, messageID, emojiID)
	if err != nil {
		log.Println(err)
	}
}

func removeAllReactions(s *discordgo.Session, channelID, messageID string) {
	err := s.MessageReactionsRemoveAll(channelID, messageID)
	if err != nil {
		log.Println(err)
	}
}

func removeReaction(s *discordgo.Session, channelID, messageID, emojiNameOrID, userID string) {
	err := s.MessageReactionRemove(channelID, messageID, emojiNameOrID, userID)
	if err != nil {
		log.Println(err)
	}
}

func matchIDCode(connectCode string, matchID int64) string {
	return fmt.Sprintf("%s:%d", connectCode, matchID)
}
