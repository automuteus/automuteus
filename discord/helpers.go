package discord

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"log"
	"strings"
	"time"
)

func (guild *GuildState) resetTrackedMembers(dg *discordgo.Session) {

	g := guild.verifyVoiceStateChanges(dg)

	for _, voiceState := range g.VoiceStates {

		userData, err := guild.UserData.GetUser(voiceState.UserID)
		if err == nil {
			//only issue a change if the user isn't in the right state already
			if !voiceState.Mute || !voiceState.Deaf || !userData.NicknamesMatch() {

				//check the userdata here so we don't spam undeafen to music bots...
				if userData.IsLinked() && !userData.IsPendingVoiceUpdate() {

					//wait until it goes through
					userData.SetPendingVoiceUpdate(true)

					guild.UserData.UpdateUserData(voiceState.UserID, userData)

					go guildMemberReset(dg, guild.PersistentGuildData.GuildID, userData)
				}
			}
		} else { //the user doesn't exist in our userdata cache; add them
			guild.checkCacheAndAddUser(g, dg, voiceState.UserID)
		}
	}
}

func guildMemberReset(s *discordgo.Session, guildID string, userData game.UserData) {
	guildMemberUpdate(s, UserPatchParameters{guildID, userData.GetID(), false, false, userData.GetOriginalNickName(), 0})
}

type UserPatchParameters struct {
	GuildID string
	UserID  string
	Deaf    bool
	Mute    bool
	Nick    string
	Delay   int
}

func guildMemberUpdate(s *discordgo.Session, params UserPatchParameters) {
	g, err := s.Guild(params.GuildID)
	if err != nil {
		log.Println(err)
	}

	if params.Delay > 0 {
		time.Sleep(time.Duration(params.Delay) * time.Second)
	}

	//we can't nickname the owner, and we shouldn't nickname with an empty string...
	if params.Nick == "" || g.OwnerID == params.UserID {
		guildMemberUpdateNoNick(s, params)
	} else {
		newParams := struct {
			Deaf bool   `json:"deaf"`
			Mute bool   `json:"mute"`
			Nick string `json:"nick"`
		}{params.Deaf, params.Mute, params.Nick}
		log.Printf("Issuing update request to discord for userID %s with mute=%v deaf=%v nick=%s\n", params.UserID, params.Mute, params.Deaf, params.Nick)

		_, err := s.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(params.GuildID, params.UserID), newParams, discordgo.EndpointGuildMember(params.GuildID, ""))
		if err != nil {
			log.Println("Failed to change nickname for user: move the bot up in your Roles")
			log.Println(err)
			guildMemberUpdateNoNick(s, params)
		}
	}
}

func guildMemberUpdateNoNick(s *discordgo.Session, params UserPatchParameters) {
	log.Printf("Issuing update request to discord for userID %s with mute=%v deaf=%v\n", params.UserID, params.Mute, params.Deaf)
	newParams := struct {
		Deaf bool `json:"deaf"`
		Mute bool `json:"mute"`
	}{params.Deaf, params.Mute}
	_, err := s.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(params.GuildID, params.UserID), newParams, discordgo.EndpointGuildMember(params.GuildID, ""))
	if err != nil {
		log.Println(err)
	}
}

func getPhaseFromArgs(args []string) game.Phase {
	if len(args) == 0 {
		return game.UNINITIALIZED
	}

	phase := strings.ToLower(args[0])
	switch phase {
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
func getRoomAndRegionFromArgs(args []string) (string, string) {
	if len(args) == 0 {
		return "Unprovided", "Unprovided"
	}
	room := strings.ToUpper(args[0])
	if len(args) == 1 {
		return room, "Unprovided"
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

func generateConnectCode(guildID string) string {
	h := sha256.New()
	h.Write([]byte(guildID))
	//add some "randomness" with the current time
	h.Write([]byte(time.Now().String()))
	hashed := strings.ToUpper(hex.EncodeToString(h.Sum(nil))[0:6])
	//TODO replace common problematic characters?
	return strings.ReplaceAll(strings.ReplaceAll(hashed, "I", "1"), "O", "0")
}
