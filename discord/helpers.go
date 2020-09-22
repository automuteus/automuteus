package discord

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func guildMemberDeafenAndMute(s *discordgo.Session, guildID string, userID string, deaf bool, mute bool) (err error) {
	log.Printf("Issuing mute=%v deaf=%v request to discord\n", mute, deaf)
	data := struct {
		Deaf bool `json:"deaf"`
		Mute bool `json:"mute"`
	}{deaf, mute}

	_, err = s.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), data, discordgo.EndpointGuildMember(guildID, ""))
	return
}

func guildMemberMute(session *discordgo.Session, guildID, userID string, mute bool) (err error) {
	log.Printf("Issuing mute=%v request to discord\n", mute)
	data := struct {
		Mute bool `json:"mute"`
	}{mute}

	_, err = session.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), data, discordgo.EndpointGuildMember(guildID, ""))
	return
}

func isVoiceChannelTracked(channelID string, trackedChannels map[string]Tracking) bool {
	for _, v := range trackedChannels {
		if v.channelID == channelID {
			return true
		}
	}
	return false
}
