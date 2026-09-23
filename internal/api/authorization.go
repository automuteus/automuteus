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
	case ReadGame, ReadSettings, ReadPremium, ReadBotPresence:
		return true
	case WriteSettings:
		return access.Owner || access.Permissions&settingsPermissions != 0
	default:
		return false
	}
}
