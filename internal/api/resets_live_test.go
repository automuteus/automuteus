package api

import (
	"context"
	"os"
	"testing"

	pgstorage "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/jackc/pgx/v4/pgxpool"
)

// TestLiveStatsResets checks the reset statements against a real Postgres: a player reset stays inside its guild
// and keeps the games, and a guild reset takes the games' players and events with it by cascade.
func TestLiveStatsResets(t *testing.T) {
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

	const guild, other = uint64(900000000000000005), uint64(900000000000000006)
	const alice, bob = uint64(3001), uint64(3002)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = ANY($1)", []uint64{guild, other})
		pool.Exec(ctx, "DELETE FROM users WHERE user_id = ANY($1)", []uint64{alice, bob})
	})
	pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = ANY($1)", []uint64{guild, other})
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	count := func(query string, args ...interface{}) int64 {
		t.Helper()
		var n int64
		if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for _, g := range []uint64{guild, other} {
		exec("INSERT INTO guilds (guild_id, guild_name, premium) VALUES ($1, 'reset test', 0)", g)
	}
	for _, u := range []uint64{alice, bob} {
		exec("INSERT INTO users (user_id, opt) VALUES ($1, true) ON CONFLICT DO NOTHING", u)
	}
	// Two games in the guild and one in the other, each with both players and an event.
	for _, g := range []uint64{guild, guild, other} {
		var gameID int64
		if err := pool.QueryRow(ctx, "INSERT INTO games (guild_id, connect_code, start_time, win_type, end_time) VALUES ($1, 'ABCDEFGH', 100, 0, 200) RETURNING game_id", g).Scan(&gameID); err != nil {
			t.Fatal(err)
		}
		for _, u := range []uint64{alice, bob} {
			exec("INSERT INTO users_games VALUES ($1, $2, $3, 'p', 0, 0, true)", u, g, gameID)
		}
		exec("INSERT INTO game_events (user_id, game_id, event_time, event_type) VALUES ($1, $2, 150, 0)", alice, gameID)
	}
	gid, oid, aid := "900000000000000005", "900000000000000006", "3001"

	removed, err := pgstorage.ResetGuildUserStats(ctx, pool, gid, aid)
	if err != nil || removed != 2 {
		t.Fatalf("player reset removed %d: %v", removed, err)
	}
	if n := count("SELECT COUNT(*) FROM users_games WHERE user_id = $1", alice); n != 1 {
		t.Errorf("alice has %d games left, want her 1 in the other guild", n)
	}
	if n := count("SELECT COUNT(*) FROM users_games WHERE guild_id = $1", guild); n != 2 {
		t.Errorf("guild has %d player rows left, want bob's 2", n)
	}
	if n := count("SELECT COUNT(*) FROM games WHERE guild_id = $1", guild); n != 2 {
		t.Errorf("player reset deleted games: %d left", n)
	}

	deleted, err := pgstorage.ResetGuildStats(ctx, pool, gid)
	if err != nil || deleted != 2 {
		t.Fatalf("guild reset deleted %d: %v", deleted, err)
	}
	if n := count("SELECT COUNT(*) FROM users_games WHERE guild_id = $1", guild); n != 0 {
		t.Errorf("%d player rows survived the guild reset", n)
	}
	if n := count("SELECT COUNT(*) FROM game_events e JOIN games g USING (game_id) WHERE g.guild_id = $1", guild); n != 0 {
		t.Errorf("%d events survived the guild reset", n)
	}
	if n := count("SELECT COUNT(*) FROM users_games WHERE guild_id = $1", other); n != 2 {
		t.Errorf("other guild has %d player rows, want 2", n)
	}
	if n := count("SELECT COUNT(*) FROM game_events e JOIN games g USING (game_id) WHERE g.guild_id = $1", other); n != 1 {
		t.Errorf("other guild has %d events, want 1", n)
	}
	if again, err := pgstorage.ResetGuildStats(ctx, pool, oid); err != nil || again != 1 {
		t.Fatalf("other guild reset deleted %d: %v", again, err)
	}
}
