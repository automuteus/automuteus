package discord

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func guildMemberMuteAndDeafen(s *discordgo.Session, guildID string, userID string, mute bool, deaf bool) (err error) {
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
	//if we aren't tracking ANY channels, we should default to true (the most predictable behavior for lazy users ;) )
	if channelID == "" || len(trackedChannels) == 0 {
		return true
	}
	for _, v := range trackedChannels {
		if v.channelID == channelID {
			return true
		}
	}
	return false
}
