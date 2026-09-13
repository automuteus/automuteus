package api

import "github.com/bwmarrin/discordgo"

// GuildAction names API capabilities, not Discord OAuth scopes.
type GuildAction string

const (
	ReadGame      GuildAction = "game:read"
	ReadPremium   GuildAction = "premium:read"
	ReadSettings  GuildAction = "settings:read"
	WriteSettings GuildAction = "settings:write"
)

// VerifiedGuildAccess must be constructed from trusted Discord responses for
// this user and guild, never from request bodies or client-supplied headers.
// Membership and permissions must be refreshed by the authentication layer.
// Permissions is Discord's guild-level bitfield, parsed without floating point.
type VerifiedGuildAccess struct {
	UserID      string
	GuildID     string
	Member      bool
	Owner       bool
	Permissions int64
}

// AllowsGuildAction is the initial, fail-closed user API policy. Even owners
// and administrators require verified membership in the exact target guild.
// Empty bot admin/operator settings do not grant access under this policy.
// Authentication is deliberately separate; this function does not verify tokens.
func AllowsGuildAction(access VerifiedGuildAccess, guildID string, action GuildAction) bool {
	if access.UserID == "" || guildID == "" || access.GuildID != guildID || !access.Member {
		return false
	}
	switch action {
	case ReadGame, ReadSettings, ReadPremium:
		return true
	case WriteSettings:
		return access.Owner || access.Permissions&discordgo.PermissionAdministrator != 0
	default:
		return false
	}
}
