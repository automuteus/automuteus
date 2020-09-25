package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"log"
	"time"
)

func (guild *GuildState) resetTrackedMembers(dg *discordgo.Session) {

	g := guild.verifyVoiceStateChanges(dg)

	for _, voiceState := range g.VoiceStates {

		userData, err := guild.UserData.GetUser(voiceState.UserID)
		if err == nil {
			//only issue a change if the user isn't in the right state already
			if !voiceState.Mute || !voiceState.Deaf || !userData.NicknamesMatch() {

				//only issue the req to discord if we're not waiting on another one
				if !userData.IsPendingVoiceUpdate() {

					//wait until it goes through
					userData.SetPendingVoiceUpdate(true)

					guild.UserData.UpdateUserData(voiceState.UserID, userData)

					go guildMemberReset(dg, guild.ID, userData)
				}
			}
		} else { //the user doesn't exist in our userdata cache; add them
			guild.addFullUserToMap(g, voiceState.UserID)
		}
	}
}

func guildMemberReset(s *discordgo.Session, guildID string, userData game.UserData) {
	guildMemberUpdate(s, guildID, userData.GetID(), UserPatchParameters{false, false, userData.GetOriginalNickName()}, 0)
}

type UserPatchParameters struct {
	Deaf bool   `json:"deaf"`
	Mute bool   `json:"mute"`
	Nick string `json:"nick"`
}

func guildMemberUpdate(s *discordgo.Session, guildID string, userID string, params UserPatchParameters, delay int) {
	g, err := s.Guild(guildID)
	if err != nil {
		log.Println(err)
	}

	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Second)
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
