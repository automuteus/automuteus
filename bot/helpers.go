package bot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/automuteus/automuteus/v8/pkg/capture"
	"log"
	"math/rand"
	"os"
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

func formCaptureURL(url, connectCode string) (hyperlink, apiHyperlink, minimalURL string) {
	return capture.FormCaptureURL(url, os.Getenv("API_SERVER_URL"), connectCode)
}

func sendEmbedWithComponents(s *discordgo.Session, channelID string, message *discordgo.MessageEmbed, components []discordgo.MessageComponent) *discordgo.Message {
	complexMsg := discordgo.MessageSend{
		Content:         "",
		Embeds:          nil,
		TTS:             false,
		Components:      components,
		Files:           nil,
		AllowedMentions: nil,
		Reference:       nil,
		File:            nil,
		Embed:           message,
	}
	msg, err := s.ChannelMessageSendComplex(channelID, &complexMsg)
	if err != nil {
		log.Println(err)
	}
	return msg
}

func editMessageEmbed(s *discordgo.Session, channelID string, messageID string, message *discordgo.MessageEmbed) *discordgo.Message {
	me := discordgo.NewMessageEdit(channelID, messageID).SetEmbed(message)
	msg, err := s.ChannelMessageEditComplex(me)
	if err != nil {
		log.Println("Error when attempting to edit complex message", err)
	}
	return msg
}

func matchIDCode(connectCode string, matchID int64) string {
	return fmt.Sprintf("%s:%d", connectCode, matchID)
}
