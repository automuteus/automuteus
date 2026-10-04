package tokenprovider

import (
	"strconv"
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/bwmarrin/discordgo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestWorkersPresent(t *testing.T) {
	tp := &TokenProvider{memberships: map[string]*workerMembership{}}
	if _, _, known := tp.WorkersPresent("g"); known {
		t.Fatal("no workers configured, but the count was reported as known")
	}

	ready := func(key string, guilds ...*discordgo.Guild) {
		tp.memberships[key] = &workerMembership{guilds: map[string]bool{}}
		tp.workerReady(key, &discordgo.Ready{Guilds: guilds})
	}
	tp.configuredWorkers = 3
	ready("a", &discordgo.Guild{ID: "g"})
	if _, _, known := tp.WorkersPresent("g"); known {
		t.Fatal("the count was reported as known while workers were still starting")
	}
	ready("b", &discordgo.Guild{ID: "g", Unavailable: true}) // an outage must not read as the worker having left
	ready("c", &discordgo.Guild{ID: "other"})

	if present, total, known := tp.WorkersPresent("g"); !known || present != 2 || total != 3 {
		t.Fatalf("WorkersPresent = %d/%d known=%v, want 2/3 known", present, total, known)
	}

	tp.memberships["c"].connected = false
	if _, _, known := tp.WorkersPresent("g"); known {
		t.Fatal("a disconnected worker's inventory was trusted")
	}
	tp.memberships["c"].connected = true
	tp.memberships["c"].ready = false
	if _, _, known := tp.WorkersPresent("g"); known {
		t.Fatal("a worker's inventory was trusted before READY")
	}
}

func TestWorkerGuildMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	tp := &TokenProvider{memberships: map[string]*workerMembership{}, metrics: server.NewMetrics(reg)}
	tp.memberships["a"] = &workerMembership{guilds: map[string]bool{}}
	tp.memberships["b"] = &workerMembership{guilds: map[string]bool{}}

	tp.workerReady("a", &discordgo.Ready{User: &discordgo.User{Username: "amu4"},
		Guilds: []*discordgo.Guild{{ID: "1"}, {ID: "2"}}})
	tp.workerReady("b", &discordgo.Ready{User: &discordgo.User{Username: "amu1", PublicFlags: discordgo.UserFlagVerifiedBot},
		Guilds: []*discordgo.Guild{{ID: "1"}}})
	tp.workerGuildCreate("a", &discordgo.GuildCreate{Guild: &discordgo.Guild{ID: "3"}})
	tp.workerGuildDelete("b", &discordgo.GuildDelete{Guild: &discordgo.Guild{ID: "1"}})

	want := `
# HELP automuteus_worker_guild_limit Most servers Discord lets each worker bot join. Absent for verified bots, which have no limit.
# TYPE automuteus_worker_guild_limit gauge
automuteus_worker_guild_limit{worker="amu4"} 100
# HELP automuteus_worker_guilds Servers each worker bot is a member of, as last seen by this process's gateway for it.
# TYPE automuteus_worker_guilds gauge
automuteus_worker_guilds{worker="amu1"} 0
automuteus_worker_guilds{worker="amu4"} 3
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), "automuteus_worker_guilds", "automuteus_worker_guild_limit"); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerStatus(t *testing.T) {
	tp := &TokenProvider{memberships: map[string]*workerMembership{}}
	ready := func(key string, user *discordgo.User, guilds int) {
		tp.memberships[key] = &workerMembership{guilds: map[string]bool{}}
		e := &discordgo.Ready{User: user}
		for i := 0; i < guilds; i++ {
			e.Guilds = append(e.Guilds, &discordgo.Guild{ID: strconv.Itoa(i)})
		}
		tp.workerReady(key, e)
	}
	ready("full", &discordgo.User{ID: "amu4"}, UnverifiedGuildLimit)
	ready("room", &discordgo.User{ID: "amu5"}, UnverifiedGuildLimit-1)
	ready("flagged", &discordgo.User{ID: "amu1", PublicFlags: discordgo.UserFlagVerifiedBot}, UnverifiedGuildLimit)
	ready("unflagged", &discordgo.User{ID: "amu2"}, UnverifiedGuildLimit+1) // more servers than allowed: verified

	tests := []struct {
		botID, guild            string
		member, full, wantKnown bool
	}{
		{"amu4", "0", true, true, true},
		{"amu5", "0", true, false, true},
		{"amu5", "elsewhere", false, false, true},
		{"amu1", "0", true, false, true},
		{"amu2", "0", true, false, true},
		{"amu3", "0", false, false, false},
	}
	for _, tt := range tests {
		member, full, known := tp.WorkerStatus(tt.botID, tt.guild)
		if member != tt.member || full != tt.full || known != tt.wantKnown {
			t.Errorf("WorkerStatus(%s, %s) = member %v full %v known %v, want %v %v %v",
				tt.botID, tt.guild, member, full, known, tt.member, tt.full, tt.wantKnown)
		}
	}

	tp.memberships["full"].connected = false
	if _, _, known := tp.WorkerStatus("amu4", "0"); known {
		t.Error("a disconnected worker's status was trusted")
	}
}
