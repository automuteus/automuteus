package task

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/bwmarrin/discordgo"
)

type UserModify struct {
	UserID uint64 `json:"userID"`
	Mute   bool   `json:"mute"`
	Deaf   bool   `json:"deaf"`
}

type UserModifyRequest struct {
	Premium premium.Tier `json:"premium"`
	Users   []UserModify `json:"users"`
}

type ModifyTask struct {
	GuildID    uint64      `json:"guildID"`
	UserID     uint64      `json:"userID"`
	Parameters PatchParams `json:"parameters"`
	TaskID     string      `json:"taskID"`
}

// NewModifyTask builds a mute/deafen task with a random ID, so that acks for distinct tasks can never be confused
// even when the same user is modified repeatedly in quick succession.
func NewModifyTask(guildID, userID uint64, params PatchParams) ModifyTask {
	var id [8]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return ModifyTask{
		GuildID:    guildID,
		UserID:     userID,
		Parameters: params,
		TaskID:     hex.EncodeToString(id[:]),
	}
}

type PatchParams struct {
	Deaf bool `json:"deaf"`
	Mute bool `json:"mute"`
}

func ApplyMuteDeaf(sess *discordgo.Session, guildID, userID string, mute, deaf bool) error {
	p := PatchParams{
		Deaf: deaf,
		Mute: mute,
	}

	_, err := sess.RequestWithBucketID("PATCH", discordgo.EndpointGuildMember(guildID, userID), p, discordgo.EndpointGuildMember(guildID, ""))
	return err
}

// a response indicating how the mutes/deafens were issued, and if ratelimits occurred
type MuteDeafenSuccessCounts struct {
	Worker    int64 `json:"worker"`
	Capture   int64 `json:"capture"`
	Official  int64 `json:"official"`
	RateLimit int64 `json:"ratelimit"`
}
