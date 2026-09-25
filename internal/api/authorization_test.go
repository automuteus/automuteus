package api

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestAllowsGuildAction(t *testing.T) {
	member := VerifiedGuildAccess{UserID: "user", GuildID: "guild", Member: true}
	owner := member
	owner.Owner = true
	admin := member
	admin.Permissions = discordgo.PermissionAdministrator
	manager := member
	manager.Permissions = discordgo.PermissionManageServer
	moderator := member
	moderator.Permissions = discordgo.PermissionManageChannels | discordgo.PermissionManageRoles | discordgo.PermissionKickMembers | discordgo.PermissionBanMembers
	departed := admin
	departed.Member = false
	anonymous := owner
	anonymous.UserID = ""

	tests := []struct {
		name   string
		access VerifiedGuildAccess
		guild  string
		action GuildAction
		want   bool
	}{
		{"member reads game", member, "guild", ReadGame, true},
		{"member reads settings", member, "guild", ReadSettings, true},
		{"member reads bot presence", member, "guild", ReadBotPresence, true},
		{"member reads stats", member, "guild", ReadStats, true},
		{"departed member cannot read stats", departed, "guild", ReadStats, false},
		{"departed member cannot read bot presence", departed, "guild", ReadBotPresence, false},
		{"member cannot write", member, "guild", WriteSettings, false},
		{"owner writes", owner, "guild", WriteSettings, true},
		{"administrator writes", admin, "guild", WriteSettings, true},
		{"manage server writes", manager, "guild", WriteSettings, true},
		{"other management permissions cannot write", moderator, "guild", WriteSettings, false},
		{"member cannot reset stats", member, "guild", ResetStats, false},
		{"moderator cannot reset stats", moderator, "guild", ResetStats, false},
		{"owner resets stats", owner, "guild", ResetStats, true},
		{"manage server resets stats", manager, "guild", ResetStats, true},
		{"cross guild admin reset denied", admin, "other", ResetStats, false},
		{"cross guild read denied", member, "other", ReadGame, false},
		{"cross guild admin write denied", admin, "other", WriteSettings, false},
		{"departed admin cannot read", departed, "guild", ReadSettings, false},
		{"departed admin cannot write", departed, "guild", WriteSettings, false},
		{"missing identity denied", anonymous, "guild", WriteSettings, false},
		{"missing target denied", owner, "", ReadSettings, false},
		{"unknown action denied even for owner", owner, "guild", GuildAction("admin:notice"), false},
		{"zero value denied", VerifiedGuildAccess{}, "guild", ReadGame, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AllowsGuildAction(tt.access, tt.guild, tt.action); got != tt.want {
				t.Fatalf("AllowsGuildAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllowsUserStatsReset(t *testing.T) {
	const guild, self, other = "guild", "100", "200"
	member := VerifiedGuildAccess{UserID: self, GuildID: guild, Member: true}
	manager := VerifiedGuildAccess{UserID: self, GuildID: guild, Member: true, Permissions: discordgo.PermissionManageServer}
	for _, tc := range []struct {
		name   string
		access VerifiedGuildAccess
		guild  string
		user   string
		want   bool
	}{
		{"member resets themselves", member, guild, self, true},
		{"member cannot reset another player", member, guild, other, false},
		{"manager resets another player", manager, guild, other, true},
		{"departed player cannot reset themselves", VerifiedGuildAccess{UserID: self, GuildID: guild}, guild, self, false},
		{"member of another guild cannot reset themselves here", VerifiedGuildAccess{UserID: self, GuildID: "other", Member: true}, guild, self, false},
		{"no target", member, guild, "", false},
		{"no identity", VerifiedGuildAccess{GuildID: guild, Member: true}, guild, "", false},
	} {
		if got := AllowsUserStatsReset(tc.access, tc.guild, tc.user); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}
