package discord

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/automuteus/utils/pkg/game"
	"log"
	"math/rand"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/storage"
	"github.com/nicksnyder/go-i18n/v2/i18n"
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

// GetRoomAndRegionFromArgs does what it sounds like
func getRoomAndRegionFromArgs(args []string, sett *storage.GuildSettings) (string, string) {
	roomUnprovided := sett.LocalizeMessage(&i18n.Message{
		ID:    "helpers.getRoomAndRegionFromArgs.roomUnprovided",
		Other: "Unprovided",
	})
	regionUnprovided := sett.LocalizeMessage(&i18n.Message{
		ID:    "helpers.getRoomAndRegionFromArgs.regionUnprovided",
		Other: "Unprovided",
	})

	if len(args) == 0 {
		return roomUnprovided, regionUnprovided
	}
	room := strings.ToUpper(args[0])
	if len(args) == 1 {
		return room, regionUnprovided
	}
	region := strings.ToLower(args[1])
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
	return room, region
}

func getRoleFromString(s *discordgo.Session, GuildID string, input string) string {
	// find which role the User was referencing in their message
	// first check if is mentionned
	ID, err := extractRoleIDFromMention(input)
	if err == nil {
		return ID
	}
	roles, _ := s.GuildRoles(GuildID)
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

	//add some randomness
	h.Write([]byte(fmt.Sprintf("%f", rand.Float64())))
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil))[0:8])
}

var urlregex = regexp.MustCompile(`^http(?P<secure>s?)://(?P<host>[\w.-]+)(?::(?P<port>\d+))?/?$`)

func formCaptureURL(url, connectCode string) (hyperlink, minimalUrl string) {
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
		minimalUrl = fmt.Sprintf("%s%s%s", protocol, host, port)
	} else {
		hyperlink = "Invalid HOST provided (should resemble something like `http://localhost:8123`)"
		minimalUrl = "Invalid HOST provided"
	}
	return
}

func mentionByUserID(userID string) string {
	return "<@!" + userID + ">"
}

// sendMessage provides a single interface to send a message to a channel via discord
func sendMessage(s *discordgo.Session, channelID string, message string) *discordgo.Message {
	msg, err := s.ChannelMessageSend(channelID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

func sendMessageDM(s *discordgo.Session, userID string, message *discordgo.MessageEmbed) *discordgo.Message {
	dmChannel, err := s.UserChannelCreate(userID)
	if err != nil {
		log.Println(err)
	}
	m, err := s.ChannelMessageSendEmbed(dmChannel.ID, message)
	if err != nil {
		log.Println(err)
	}
	return m
}

func sendMessageEmbed(s *discordgo.Session, channelID string, message *discordgo.MessageEmbed) *discordgo.Message {
	msg, err := s.ChannelMessageSendEmbed(channelID, message)
	if err != nil {
		log.Println(err)
	}
	return msg
}

// editMessage provides a single interface to edit a message in a channel via discord
func editMessage(s *discordgo.Session, channelID string, messageID string, message string) *discordgo.Message {
	msg, err := s.ChannelMessageEdit(channelID, messageID, message)
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
