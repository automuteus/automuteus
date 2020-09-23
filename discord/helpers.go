package discord

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func guildMemberUpdate(s *discordgo.Session, guildID string, userID string, mute bool, deaf bool, nick string) {
	log.Printf("Issuing update request to discord for userID %s with mute=%v deaf=%v nick=%s\n", userID, mute, deaf, nick)
	data := struct {
		Deaf bool   `json:"deaf"`
		Mute bool   `json:"mute"`
		Nick string `json:"nick"`
	}{deaf, mute, nick}

	_, err := s.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), data, discordgo.EndpointGuildMember(guildID, ""))
	if err != nil {
		log.Println(err)
	}
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
