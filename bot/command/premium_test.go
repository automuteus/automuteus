package command

import (
	"reflect"
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
)

const (
	amu1 = "780323275624546304"
	amu2 = "769022114229125181"
	amu3 = "780323801173983262"
	amu4 = "780589033033302036"
	amu5 = "780589278195220480"
)

// statuses builds a BotStatus from the bots in the server and the bots that are full.
func statuses(members, full []string) BotStatus {
	return func(id string) (bool, bool) {
		return contains(members, id), contains(full, id)
	}
}

func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func TestOfferedBots(t *testing.T) {
	tests := []struct {
		name          string
		tier          premium.Tier
		members, full []string
		want          []OfferedBot
	}{
		{name: "gold, nothing known", tier: premium.GoldTier,
			want: []OfferedBot{{ID: amu1}, {ID: amu2}, {ID: amu3}}},
		{name: "gold skips full bots", tier: premium.GoldTier, full: []string{amu2, amu3},
			want: []OfferedBot{{ID: amu1}, {ID: amu4}, {ID: amu5}}},
		{name: "gold keeps bots it already has", tier: premium.GoldTier, members: []string{amu1, amu4, amu5},
			want: []OfferedBot{{ID: amu1, Member: true}, {ID: amu4, Member: true}, {ID: amu5, Member: true}}},
		{name: "a full bot already in the server still counts", tier: premium.GoldTier, members: []string{amu4}, full: []string{amu4},
			want: []OfferedBot{{ID: amu4, Member: true}, {ID: amu1}, {ID: amu2}}},
		{name: "gold when too many are full", tier: premium.GoldTier, full: []string{amu2, amu3, amu4, amu5},
			want: []OfferedBot{{ID: amu1}}},
		{name: "silver gets amu1", tier: premium.SilverTier, want: []OfferedBot{{ID: amu1}}},
		{name: "silver keeps the bot it has", tier: premium.SilverTier, members: []string{amu3},
			want: []OfferedBot{{ID: amu3, Member: true}}},
		{name: "bronze", tier: premium.BronzeTier},
		{name: "trial", tier: premium.TrialTier},
		{name: "self-hosted", tier: premium.SelfHostTier},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := OfferedBots(tt.tier, statuses(tt.members, tt.full))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OfferedBots = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestInvitesResponse(t *testing.T) {
	sett := settings.MakeGuildSettings()

	embed := invitesResponse(premium.GoldTier, statuses([]string{amu1}, []string{amu2}), sett)
	if len(embed.Fields) != 3 {
		t.Fatalf("Gold has %d fields, want 3: %+v", len(embed.Fields), embed.Fields)
	}
	if strings.Contains(embed.Fields[0].Value, "client_id=") {
		t.Errorf("bot already in the server has an invite link: %q", embed.Fields[0].Value)
	}
	for i, id := range []string{amu3, amu4} {
		if v := embed.Fields[i+1].Value; !strings.Contains(v, "client_id="+id) {
			t.Errorf("field %d = %q, want an invite for %s", i+2, v, id)
		}
	}

	embed = invitesResponse(premium.GoldTier, statuses(nil, []string{amu2, amu3, amu4, amu5}), sett)
	if len(embed.Fields) != 2 || !strings.Contains(embed.Fields[1].Value, "server limit") {
		t.Errorf("Gold with four full bots = %+v, want amu1 and a capacity note", embed.Fields)
	}

	if embed := invitesResponse(premium.BronzeTier, nil, sett); len(embed.Fields) != 0 {
		t.Errorf("Bronze was offered invites: %+v", embed.Fields)
	}
}
