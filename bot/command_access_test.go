package bot

import (
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

func TestCommandAccess(t *testing.T) {
	const owner, legacyAdmin, someone = "100000000000000001", "100000000000000002", "100000000000000003"
	const operatorRole = "200000000000000001"
	member := func(id string, permissions int64, roles ...string) *discordgo.Member {
		return &discordgo.Member{User: &discordgo.User{ID: id}, Permissions: permissions, Roles: roles}
	}
	with := func(adminIDs, roleIDs []string) *settings.GuildSettings {
		s := settings.MakeGuildSettings()
		s.SetAdminUserIDs(adminIDs)
		s.SetPermissionRoleIDs(roleIDs)
		return s
	}
	none, roles, legacy := with(nil, nil), with(nil, []string{operatorRole}), with([]string{legacyAdmin}, []string{operatorRole})

	for _, tc := range []struct {
		name         string
		sett         *settings.GuildSettings
		member       *discordgo.Member
		admin, games bool
	}{
		{"owner always", roles, member(owner, 0), true, true},
		{"discord administrator always", roles, member(someone, discordgo.PermissionAdministrator), true, true},
		{"manage server always", roles, member(someone, discordgo.PermissionManageServer), true, true},
		{"manage server among other bits", roles, member(someone, discordgo.PermissionManageChannels|discordgo.PermissionManageServer), true, true},
		{"other management permissions are not enough", roles, member(someone, discordgo.PermissionManageChannels|discordgo.PermissionManageRoles|discordgo.PermissionKickMembers), false, false},
		{"no roles configured: everyone operates, nobody else administers", none, member(someone, 0), false, true},
		{"role holder operates but does not administer", roles, member(someone, 0, operatorRole), false, true},
		{"non-holder is locked out of games", roles, member(someone, 0), false, false},
		{"legacy admin user ID grants nothing", legacy, member(legacyAdmin, 0), false, false},
		{"legacy admin user ID with the role only operates", legacy, member(legacyAdmin, 0, operatorRole), false, true},
		{"missing member", roles, nil, false, false},
		{"missing user", roles, &discordgo.Member{Permissions: discordgo.PermissionAdministrator}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			admin, games := commandAccess(tc.sett, owner, tc.member)
			if admin != tc.admin || games != tc.games {
				t.Fatalf("commandAccess = admin %v, games %v; want admin %v, games %v", admin, games, tc.admin, tc.games)
			}
		})
	}
}
