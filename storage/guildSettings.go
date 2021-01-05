package storage

import (
	"github.com/automuteus/utils/pkg/game"
	"os"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/locale"
)

const DefaultLeaderboardSize = 3
const DefaultLeaderboardMin = 3

type GuildSettings struct {
	AdminUserIDs             []string        `json:"adminIDs"`
	PermissionRoleIDs        []string        `json:"permissionRoleIDs"`
	CommandPrefix            string          `json:"commandPrefix"`
	Language                 string          `json:"language"`
	VoiceRules               game.VoiceRules `json:"voiceRules"`
	MapVersion               string          `json:"mapVersion"`
	Delays                   game.GameDelays `json:"delays"`
	DeleteGameSummaryMinutes int             `json:"deleteGameSummary"`
	lock                     sync.RWMutex
	UnmuteDeadDuringTasks    bool   `json:"unmuteDeadDuringTasks"`
	AutoRefresh              bool   `json:"autoRefresh"`
	MatchSummaryChannelID    string `json:"matchSummaryChannelID"`
	LeaderboardMention       bool   `json:"leaderboardMention"`
	LeaderboardSize          int    `json:"leaderboardSize"`
	LeaderboardMin           int    `json:"leaderboardMin"`
	MuteSpectator            bool   `json:"muteSpectator"`
}

func MakeGuildSettings() *GuildSettings {
	prefix := os.Getenv("AUTOMUTEUS_GLOBAL_PREFIX")
	if prefix == "" {
		prefix = ".au"
	}
	return &GuildSettings{
		CommandPrefix:            prefix,
		Language:                 locale.DefaultLang,
		AdminUserIDs:             []string{},
		PermissionRoleIDs:        []string{},
		Delays:                   game.MakeDefaultDelays(),
		VoiceRules:               game.MakeMuteAndDeafenRules(),
		UnmuteDeadDuringTasks:    false,
		DeleteGameSummaryMinutes: 0, //-1 for never delete the match summary
		AutoRefresh:              false,
		MapVersion:               "simple",
		MatchSummaryChannelID:    "",
		LeaderboardMention:       true,
		LeaderboardSize:          3,
		LeaderboardMin:           3,
		MuteSpectator:            false,
		lock:                     sync.RWMutex{},
	}
}

func (gs *GuildSettings) LocalizeMessage(args ...interface{}) string {
	args = append(args, gs.GetLanguage())
	return locale.LocalizeMessage(args...)
}

func (gs *GuildSettings) HasAdminPerms(user *discordgo.User) bool {
	if user == nil {
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

func (gs *GuildSettings) GetDeleteGameSummaryMinutes() int {
	return gs.DeleteGameSummaryMinutes
}

func (gs *GuildSettings) SetDeleteGameSummaryMinutes(num int) {
	gs.DeleteGameSummaryMinutes = num
}

func (gs *GuildSettings) SetMatchSummaryChannelID(id string) {
	gs.MatchSummaryChannelID = id
}

func (gs *GuildSettings) GetMatchSummaryChannelID() string {
	return gs.MatchSummaryChannelID
}

func (gs *GuildSettings) GetAutoRefresh() bool {
	return gs.AutoRefresh
}

func (gs *GuildSettings) SetAutoRefresh(n bool) {
	gs.AutoRefresh = n
}

func (gs *GuildSettings) GetLeaderboardMention() bool {
	return gs.LeaderboardMention
}

func (gs *GuildSettings) SetLeaderboardMention(v bool) {
	gs.LeaderboardMention = v
}

func (gs *GuildSettings) GetLeaderboardSize() int {
	if gs.LeaderboardSize < 1 {
		return DefaultLeaderboardSize
	}
	return gs.LeaderboardSize
}

func (gs *GuildSettings) SetLeaderboardSize(v int) {
	gs.LeaderboardSize = v
}

func (gs *GuildSettings) GetLeaderboardMin() int {
	if gs.LeaderboardMin < 1 {
		return DefaultLeaderboardMin
	}
	return gs.LeaderboardMin
}

func (gs *GuildSettings) SetLeaderboardMin(v int) {
	gs.LeaderboardMin = v
}

func (gs *GuildSettings) GetMuteSpectator() bool {
	return gs.MuteSpectator
}

func (gs *GuildSettings) SetMuteSpectator(behavior bool) {
	gs.MuteSpectator = behavior
}

func (gs *GuildSettings) GetMapVersion() string {
	if gs.MapVersion == "" {
		return "simple"
	}
	return gs.MapVersion
}

func (gs *GuildSettings) SetMapVersion(n string) {
	gs.MapVersion = n
}

func (gs *GuildSettings) SetUnmuteDeadDuringTasks(v bool) {
	gs.UnmuteDeadDuringTasks = v
}

func (gs *GuildSettings) GetLanguage() string {
	return gs.Language
}

func (gs *GuildSettings) SetLanguage(l string) {
	gs.Language = l
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
	}
	return gs.VoiceRules.DeafRules[phase.ToString()][alive]
}

func (gs *GuildSettings) SetVoiceRule(isMute bool, phase game.Phase, alive string, val bool) {
	if isMute {
		gs.VoiceRules.MuteRules[phase.ToString()][alive] = val
	}
	gs.VoiceRules.DeafRules[phase.ToString()][alive] = val
}

func (gs *GuildSettings) GetVoiceState(alive bool, tracked bool, phase game.Phase) (bool, bool) {
	return gs.VoiceRules.GetVoiceState(alive, tracked, phase)
}
