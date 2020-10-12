package storage

import (
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"sync"
)

type GuildSettings struct {
	CommandPrefix         string `json:"commandPrefix"`
	DefaultTrackedChannel string `json:"defaultTrackedChannel"`

	AdminUserIDs          []string        `json:"adminIDs"`
	PermissionedRoleIDs   []string        `json:"permissionRoleIDs"`
	Delays                game.GameDelays `json:"delays"`
	VoiceRules            game.VoiceRules `json:"voiceRules"`
	ApplyNicknames        bool            `json:"applyNicknames"`
	UnmuteDeadDuringTasks bool            `json:"unmuteDeadDuringTasks"`

	lock sync.RWMutex
}

func MakeGuildSettings() GuildSettings {
	return GuildSettings{
		CommandPrefix:         ".au",
		DefaultTrackedChannel: "",
		AdminUserIDs:          []string{},
		PermissionedRoleIDs:   []string{},
		Delays:                game.MakeDefaultDelays(),
		VoiceRules:            game.MakeMuteAndDeafenRules(),
		ApplyNicknames:        false,
		UnmuteDeadDuringTasks: false,
		lock:                  sync.RWMutex{},
	}
}

func (gs *GuildSettings) EmptyAdminAndRolePerms() bool {
	return len(gs.AdminUserIDs) == 0 && len(gs.PermissionedRoleIDs) == 0
}

func (gs *GuildSettings) HasAdminPerms(mem *discordgo.Member) bool {
	if len(gs.AdminUserIDs) == 0 {
		return false
	}

	for _, v := range gs.AdminUserIDs {
		if v == mem.User.ID {
			return true
		}
	}
	return false
}

func (gs *GuildSettings) HasRolePerms(mem *discordgo.Member) bool {
	if len(gs.PermissionedRoleIDs) == 0 {
		return false
	}

	for _, role := range mem.Roles {
		for _, testRole := range gs.PermissionedRoleIDs {
			if testRole == role {
				return true
			}
		}
	}
	return false
}

func (gs *GuildSettings) GetCommandPrefix() string {
	return gs.CommandPrefix
}
