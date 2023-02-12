package discord

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/automuteus/automuteus/v7/pkg/premium"
	"github.com/bwmarrin/discordgo"
	"time"
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

const IDLength = 10

func NewModifyTask(guildID, userID uint64, params PatchParams) ModifyTask {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d", guildID)))
	h.Write([]byte(fmt.Sprintf("%d", userID)))
	h.Write([]byte(fmt.Sprintf("%d", time.Now().Unix())))
	return ModifyTask{
		GuildID:    guildID,
		UserID:     userID,
		Parameters: params,
		TaskID:     hex.EncodeToString(h.Sum(nil))[0:IDLength],
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
