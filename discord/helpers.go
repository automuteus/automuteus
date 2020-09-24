package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"log"
)

func (guild *GuildState) resetTrackedMembers(dg *discordgo.Session) {

	g := guild.verifyVoiceStateChanges(dg)

	for _, voiceState := range g.VoiceStates {

		guild.UserDataLock.RLock()
		if userData, ok := guild.UserData[voiceState.UserID]; ok {

			//only issue a change if the user isn't in the right state already
			if !voiceState.Mute || !voiceState.Deaf || !userData.NicknamesMatch() {

				//only issue the req to discord if we're not waiting on another one
				if !userData.IsPendingVoiceUpdate() {
					guild.UserDataLock.RUnlock()
					//wait until it goes through
					userData.SetPendingVoiceUpdate(true)

					go guild.updateUserInMap(voiceState.UserID, userData)

					go guildMemberReset(dg, guild.ID, userData)

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

func guildMemberReset(s *discordgo.Session, guildID string, userData game.UserData) {
	guildMemberUpdate(s, guildID, userData.GetID(), UserPatchParameters{false, false, userData.GetOriginalNickName()})
}

type UserPatchParameters struct {
	Deaf bool   `json:"deaf"`
	Mute bool   `json:"mute"`
	Nick string `json:"nick"`
}

func guildMemberUpdate(s *discordgo.Session, guildID string, userID string, params UserPatchParameters) {
	g, err := s.Guild(guildID)
	if err != nil {
		log.Println(err)
	}

	//we can't nickname the owner, and we shouldn't nickname with an empty string...
	if params.Nick == "" || g.OwnerID == userID {
		guildMemberUpdateNoNick(s, guildID, userID, params)
	} else {
		log.Printf("Issuing update request to discord for userID %s with mute=%v deaf=%v nick=%s\n", userID, params.Mute, params.Deaf, params.Nick)

		_, err := s.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), params, discordgo.EndpointGuildMember(guildID, ""))
		if err != nil {
			log.Println("Failed to change nickname for user: move the bot up in your Roles")
			log.Println(err)
			guildMemberUpdateNoNick(s, guildID, userID, params)
		}
	}
}

func guildMemberUpdateNoNick(s *discordgo.Session, guildID string, userID string, params UserPatchParameters) {
	log.Printf("Issuing update request to discord for userID %s with mute=%v deaf=%v\n", userID, params.Mute, params.Deaf)
	newParams := struct {
		Deaf bool `json:"deaf"`
		Mute bool `json:"mute"`
	}{params.Deaf, params.Mute}
	_, err := s.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), newParams, discordgo.EndpointGuildMember(guildID, ""))
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
