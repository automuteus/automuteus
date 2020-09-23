package discord

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func (guild *GuildState) resetTrackedMembers(dg *discordgo.Session) {

	g := guild.verifyVoiceStateChanges(dg)

	for _, voiceState := range g.VoiceStates {

		guild.UserDataLock.RLock()
		if userData, ok := guild.UserData[voiceState.UserID]; ok {

			//only issue a change if the user isn't in the right state already
			if !voiceState.Mute || !voiceState.Deaf {

				//only issue the req to discord if we're not waiting on another one
				if !userData.pendingVoiceUpdate {
					guild.UserDataLock.RUnlock()
					//wait until it goes through
					userData.pendingVoiceUpdate = true

					go guild.updateUserInMap(voiceState.UserID, userData)

					go guildMemberReset(dg, guild.ID, userData.user)

					guild.UserDataLock.RLock()
				}

			}
		} else { //the user doesn't exist in our userdata cache; add them
			guild.UserDataLock.RUnlock()

			guild.addFullUserToMap(g, voiceState.UserID)

			guild.UserDataLock.RLock()

		}
		guild.UserDataLock.RUnlock()
	}
}

func guildMemberReset(s *discordgo.Session, guildID string, user User) {
	guildMemberUpdate(s, guildID, user.userID, false, false, user.originalNick)
}

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
