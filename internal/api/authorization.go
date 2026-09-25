package api

import "github.com/bwmarrin/discordgo"

// GuildAction names API capabilities, not Discord OAuth scopes.
type GuildAction string

const (
	ReadGame        GuildAction = "game:read"
	ReadPremium     GuildAction = "premium:read"
	ReadSettings    GuildAction = "settings:read"
	WriteSettings   GuildAction = "settings:write"
	ReadBotPresence GuildAction = "bot:read"
	// ReadStats is the guild statistics page. Any member may see it.
	ReadStats GuildAction = "stats:read"
	// ResetStats deletes a guild's recorded games, or any player's part in them. It takes the same permissions as
	// changing the settings. Players may also reset their own part; see AllowsUserStatsReset.
	ResetStats GuildAction = "stats:reset"
)

// verifiesLive reports whether an action must be authorized against Discord on every request rather than from
// the short-lived access cache, so a revoked permission stops a change at once.
func (a GuildAction) verifiesLive() bool {
	return a == WriteSettings || a == ResetStats
}

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

// settingsPermissions are the Discord permissions that allow changing the bot's settings: Administrator or
// Manage Server, Discord's own bar for configuring integrations. The bot's slash-command gate uses the same
// bits, so the two surfaces never disagree about who may edit.
const settingsPermissions = discordgo.PermissionAdministrator | discordgo.PermissionManageServer

// AllowsGuildAction is the fail-closed user API policy. Even owners and
// administrators require verified membership in the exact target guild.
// The bot's stored admin user and operator role lists never grant access here.
// Authentication is deliberately separate; this function does not verify tokens.
func AllowsGuildAction(access VerifiedGuildAccess, guildID string, action GuildAction) bool {
	if access.UserID == "" || guildID == "" || access.GuildID != guildID || !access.Member {
		return false
	}
	switch action {
	case ReadGame, ReadSettings, ReadPremium, ReadBotPresence, ReadStats:
		return true
	case WriteSettings, ResetStats:
		return access.Owner || access.Permissions&settingsPermissions != 0
	default:
		return false
	}
}

// AllowsUserStatsReset is the policy for removing one player from a guild's games: anyone who may reset the guild's
// stats may reset any player's, and any member may reset their own. Either way only this guild's games change.
func AllowsUserStatsReset(access VerifiedGuildAccess, guildID, userID string) bool {
	if AllowsGuildAction(access, guildID, ResetStats) {
		return true
	}
	return userID != "" && access.UserID == userID && AllowsGuildAction(access, guildID, ReadStats)
}
