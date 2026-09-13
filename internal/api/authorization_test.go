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
		{"member cannot write", member, "guild", WriteSettings, false},
		{"owner writes", owner, "guild", WriteSettings, true},
		{"administrator writes", admin, "guild", WriteSettings, true},
		{"manage server alone cannot write", manager, "guild", WriteSettings, false},
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
