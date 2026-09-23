package bot

import (
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

func TestCommandAccess(t *testing.T) {
	const owner, adminUser, someone = "100000000000000001", "100000000000000002", "100000000000000003"
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
	none, roles, admins, both := with(nil, nil), with(nil, []string{operatorRole}), with([]string{adminUser}, nil), with([]string{adminUser}, []string{operatorRole})

	for _, tc := range []struct {
		name         string
		sett         *settings.GuildSettings
		member       *discordgo.Member
		admin, games bool
	}{
		{"owner always", both, member(owner, 0), true, true},
		{"discord administrator always", both, member(someone, discordgo.PermissionAdministrator), true, true},
		{"administrator bit among others", both, member(someone, discordgo.PermissionManageServer|discordgo.PermissionAdministrator), true, true},
		{"manage server alone is not enough", both, member(someone, discordgo.PermissionManageServer), false, false},
		{"nothing configured: everyone", none, member(someone, 0), true, true},
		{"roles only: holder is admin and operator", roles, member(someone, 0, operatorRole), true, true},
		{"roles only: non-holder is locked out", roles, member(someone, 0), false, false},
		{"admins only: admin user", admins, member(adminUser, 0), true, true},
		{"admins only: other member still controls games", admins, member(someone, 0), false, true},
		{"both: admin user without the role still controls games", both, member(adminUser, 0), true, true},
		{"both: role holder is operator, not admin", both, member(someone, 0, operatorRole), false, true},
		{"both: plain member is locked out", both, member(someone, 0), false, false},
		{"missing member", both, nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			admin, games := commandAccess(tc.sett, owner, tc.member)
			if admin != tc.admin || games != tc.games {
				t.Fatalf("commandAccess = admin %v, games %v; want admin %v, games %v", admin, games, tc.admin, tc.games)
			}
		})
	}
}
