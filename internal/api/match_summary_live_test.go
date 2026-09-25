package api

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/jackc/pgx/v4/pgxpool"
)

// TestLiveMatchSummary runs the match queries against a real Postgres, in particular that jsonb payloads read
// back as text parse the way the bot wrote them.
func TestLiveMatchSummary(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set disposable TEST_POSTGRES_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := storage.ApplySchemas(ctx, pool, false); err != nil {
		t.Fatal(err)
	}

	const guild, otherGuild = uint64(900000000000000011), uint64(900000000000000012)
	const alice, bob = uint64(1011), uint64(1012)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = ANY($1)", []uint64{guild, otherGuild})
		pool.Exec(ctx, "DELETE FROM users WHERE user_id = ANY($1)", []uint64{alice, bob})
	})
	pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = ANY($1)", []uint64{guild, otherGuild})
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	for _, g := range []uint64{guild, otherGuild} {
		exec("INSERT INTO guilds (guild_id, guild_name, premium) VALUES ($1, 'match test', 0)", g)
	}
	for _, u := range []uint64{alice, bob} {
		exec("INSERT INTO users (user_id, opt) VALUES ($1, true) ON CONFLICT DO NOTHING", u)
	}
	var matchID int64
	if err := pool.QueryRow(ctx, "INSERT INTO games (guild_id, connect_code, start_time, win_type, end_time, play_map, region) "+
		"VALUES ($1, 'LIVETEST', 1000, $2, 1400, $3, $4) RETURNING game_id",
		guild, int16(game.HumansByVote), int16(game.AIRSHIP), int16(game.AS)).Scan(&matchID); err != nil {
		t.Fatal(err)
	}
	exec("INSERT INTO users_games VALUES ($1, $2, $3, 'Al', $4, $5, true), ($6, $2, $3, 'Bo', $7, $8, false)",
		alice, guild, matchID, int16(game.Cyan), int16(game.CrewmateRole), bob, int16(game.Coral), int16(game.ImposterRole))
	// Written the way the bot's AddEvent writes them: the capture's payload string, bound straight into jsonb.
	events := []struct {
		user    interface{}
		at      int32
		kind    capture.EventType
		payload string
	}{
		{nil, 1004, capture.State, strconv.Itoa(int(game.TASKS))},
		{alice, 1050, capture.Player, playerPayload(game.DIED, "Al", game.Cyan)},
		{nil, 1060, capture.State, strconv.Itoa(int(game.DISCUSS))},
		{nil, 1061, capture.Lobby, `{"LobbyCode":"XYZ","Region":1,"PlayMap":4}`},
		{bob, 1120, capture.Player, playerPayload(game.EXILED, "Bo", game.Coral)},
		{nil, 1150, capture.Player, playerPayload(game.DIED, "Guest", game.Lime)},
		{nil, 1400, capture.GameOver, `{"GameOverReason":0,"PlayerInfos":[{"Name":"Al","IsImpostor":false},{"Name":"Bo","IsImpostor":true},{"Name":"Guest","IsImpostor":false}]}`},
	}
	for _, e := range events {
		exec("INSERT INTO game_events VALUES (DEFAULT, $1, $2, $3, $4, $5)", e.user, matchID, e.at, int16(e.kind), e.payload)
	}

	guildID, id := strconv.FormatUint(guild, 10), strconv.FormatInt(matchID, 10)
	m, err := buildMatchSummary(ctx, pool, nil, nil, guildID, id, premium.PremiumRecord{Tier: premium.SelfHostTier, Days: premium.NoExpiryCode})
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != "finished" || m.Result != "crewmateVote" || m.Winner != "crewmate" || m.Map != "airship" || m.Region != "as" || m.EndTime != 1400 {
		t.Fatalf("header = %+v", m)
	}
	wantRoster := []MatchPlayer{
		{UserID: "1012", Name: "Bo", Color: "coral", Role: "impostor", Won: false},
		{UserID: "1011", Name: "Al", Color: "cyan", Role: "crewmate", Won: true},
		{Name: "Guest", Color: "lime", Role: "crewmate", Won: true},
	}
	if !reflect.DeepEqual(m.Roster, wantRoster) || !m.RosterComplete {
		t.Fatalf("roster = %+v", m.Roster)
	}
	wantTimeline := &MatchTimeline{Meetings: 1, Deaths: 2, Exiles: 1, Events: []MatchEvent{
		{Offset: 4, Type: "tasks"},
		{Offset: 50, Type: "death", Name: "Al", Color: "cyan", UserID: "1011"},
		{Offset: 60, Type: "discussion"},
		{Offset: 120, Type: "exile", Name: "Bo", Color: "coral", UserID: "1012"},
		{Offset: 150, Type: "death", Name: "Guest", Color: "lime"},
	}}
	if !reflect.DeepEqual(m.Timeline, wantTimeline) {
		t.Fatalf("timeline = %+v", m.Timeline)
	}

	if _, err := buildMatchSummary(ctx, pool, nil, nil, strconv.FormatUint(otherGuild, 10), id, premium.PremiumRecord{Tier: premium.SelfHostTier, Days: premium.NoExpiryCode}); !errors.Is(err, errMatchNotFound) {
		t.Fatalf("another guild read the match: %v", err)
	}
}
