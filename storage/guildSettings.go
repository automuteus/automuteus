package storage

import (
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/locale"
)

type GuildSettings struct {
	CommandPrefix         string `json:"commandPrefix"`
	DefaultTrackedChannel string `json:"defaultTrackedChannel"`
	Language              string `json:"language"`

	AdminUserIDs          []string        `json:"adminIDs"`
	PermissionRoleIDs     []string        `json:"permissionRoleIDs"`
	Delays                game.GameDelays `json:"delays"`
	VoiceRules            game.VoiceRules `json:"voiceRules"`
	ApplyNicknames        bool            `json:"applyNicknames"`
	UnmuteDeadDuringTasks bool            `json:"unmuteDeadDuringTasks"`

	lock sync.RWMutex
}

func MakeGuildSettings() *GuildSettings {
	return &GuildSettings{
		CommandPrefix:         ".au",
		DefaultTrackedChannel: "",
		Language:              locale.DefaultLang,
		AdminUserIDs:          []string{},
		PermissionRoleIDs:     []string{},
		Delays:                game.MakeDefaultDelays(),
		VoiceRules:            game.MakeMuteAndDeafenRules(),
		ApplyNicknames:        false,
		UnmuteDeadDuringTasks: false,
		lock:                  sync.RWMutex{},
	}
}

func (gs *GuildSettings) LocalizeMessage(args ...interface{}) string {
	args = append(args, gs.GetLanguage())
	return locale.LocalizeMessage(args...)
}

func (gs *GuildSettings) HasAdminPerms(user *discordgo.User) bool {
	if len(gs.AdminUserIDs) == 0 || user == nil {
		return false
	}

	for _, v := range gs.AdminUserIDs {
		if v == user.ID {
			return true
		}
	}
	return false
}

func (gs *GuildSettings) HasRolePerms(mem *discordgo.Member) bool {
	if len(gs.PermissionRoleIDs) == 0 {
		return false
	}

	for _, role := range mem.Roles {
		for _, testRole := range gs.PermissionRoleIDs {
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

func (gs *GuildSettings) SetCommandPrefix(p string) {
	gs.CommandPrefix = p
}

func (gs *GuildSettings) GetAdminUserIDs() []string {
	return gs.AdminUserIDs
}

func (gs *GuildSettings) SetAdminUserIDs(ids []string) {
	gs.AdminUserIDs = ids
}

func (gs *GuildSettings) GetPermissionRoleIDs() []string {
	return gs.PermissionRoleIDs
}

func (gs *GuildSettings) SetPermissionRoleIDs(ids []string) {
	gs.PermissionRoleIDs = ids
}

func (gs *GuildSettings) GetUnmuteDeadDuringTasks() bool {
	return gs.UnmuteDeadDuringTasks
}

func (gs *GuildSettings) SetUnmuteDeadDuringTasks(v bool) {
	gs.UnmuteDeadDuringTasks = v
}

func (gs *GuildSettings) GetDefaultTrackedChannel() string {
	return gs.DefaultTrackedChannel
}

func (gs *GuildSettings) SetDefaultTrackedChannel(c string) {
	gs.DefaultTrackedChannel = c
}

func (gs *GuildSettings) GetLanguage() string {
	return gs.Language
}

func (gs *GuildSettings) SetLanguage(l string) {
	gs.Language = l
}

func (gs *GuildSettings) GetApplyNicknames() bool {
	return gs.ApplyNicknames
}

func (gs *GuildSettings) SetApplyNicknames(v bool) {
	gs.ApplyNicknames = v
}

func (gs *GuildSettings) GetDelay(oldPhase, newPhase game.Phase) int {
	return gs.Delays.GetDelay(oldPhase, newPhase)
}

func (gs *GuildSettings) SetDelay(oldPhase, newPhase game.Phase, v int) {
	gs.Delays.Delays[oldPhase.ToString()][newPhase.ToString()] = v
}

func (gs *GuildSettings) GetVoiceRule(isMute bool, phase game.Phase, alive string) bool {
	if isMute {
		return gs.VoiceRules.MuteRules[phase.ToString()][alive]
	} else {
		return gs.VoiceRules.DeafRules[phase.ToString()][alive]
	}
}

func (gs *GuildSettings) SetVoiceRule(isMute bool, phase game.Phase, alive string, val bool) {
	if isMute {
		gs.VoiceRules.MuteRules[phase.ToString()][alive] = val
	} else {
		gs.VoiceRules.DeafRules[phase.ToString()][alive] = val
	}
}

func (gs *GuildSettings) GetVoiceState(alive bool, tracked bool, phase game.Phase) (bool, bool) {
	return gs.VoiceRules.GetVoiceState(alive, tracked, phase)
}
