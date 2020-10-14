package discord

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
)

// when querying for the member list we need to specify a size
// too high reduces performance
// too low increases the chance of the member we want not being in the list
// ideally this should be adjusted on a per-server basis
const MemberQuerySize = 1000

type UserPatchParameters struct {
	GuildID  string
	Userdata UserData
	Deaf     bool
	Mute     bool
	Nick     string
}

func guildMemberUpdate(s *discordgo.Session, params UserPatchParameters) {
	g, err := s.Guild(params.GuildID)
	if err != nil {
		log.Println(err)
	}

	//we can't nickname the owner, and we shouldn't nickname with an empty string...
	if params.Nick == "" || g.OwnerID == params.Userdata.GetID() {
		guildMemberUpdateNoNick(s, params)
	} else {
		newParams := struct {
			Deaf bool   `json:"deaf"`
			Mute bool   `json:"mute"`
			Nick string `json:"nick"`
		}{params.Deaf, params.Mute, params.Nick}
		log.Printf("Issuing update request to discord for userID %s with mute=%v deaf=%v nick=%s\n", params.Userdata.GetID(), params.Mute, params.Deaf, params.Nick)

		_, err := s.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(params.GuildID, params.Userdata.GetID()), newParams, discordgo.EndpointGuildMember(params.GuildID, ""))
		if err != nil {
			log.Println("Failed to change nickname for user: move the bot up in your Roles")
			log.Println(err)
			guildMemberUpdateNoNick(s, params)
		}
	}
}

func guildMemberUpdateNoNick(s *discordgo.Session, params UserPatchParameters) {
	log.Printf("Issuing update request to discord for userID %s with mute=%v deaf=%v\n", params.Userdata.GetID(), params.Mute, params.Deaf)
	newParams := struct {
		Deaf bool `json:"deaf"`
		Mute bool `json:"mute"`
	}{params.Deaf, params.Mute}
	_, err := s.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(params.GuildID, params.Userdata.GetID()), newParams, discordgo.EndpointGuildMember(params.GuildID, ""))
	if err != nil {
		log.Println(err)
	}
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

func getMemberFromString(s *discordgo.Session, GuildID string, input string) string {
	// find which member the user was referencing in their message
	// TODO increase performance by caching member list for when function called more than once
	// first check if is mentionned
	ID, err := extractUserIDFromMention(input)
	if err == nil {
		return ID
	}
	members, _ := s.GuildMembers(GuildID, "", MemberQuerySize)
	for _, member := range members {
		if input == member.User.ID || input == strings.ToLower(member.Nick) || input == strings.ToLower(member.User.Username) ||
			input == strings.ToLower(member.User.Username)+"#"+member.User.Discriminator {
			return member.User.ID
		}
	}
	return ""
}

func getRoleFromString(s *discordgo.Session, GuildID string, input string) string {
	// find which role the user was referencing in their message
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
	//add some "randomness" with the current time
	h.Write([]byte(time.Now().String()))
	hashed := strings.ToUpper(hex.EncodeToString(h.Sum(nil))[0:8])
	//TODO replace common problematic characters?
	return strings.ReplaceAll(strings.ReplaceAll(hashed, "I", "1"), "O", "0")
}
