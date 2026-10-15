package storage

// This test needs a disposable database named by TEST_POSTGRES_URL. It adds
// columns to the stats tables, so never point it at a real deployment.

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/capture"
	schema "github.com/automuteus/automuteus/v8/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestReadsTolerateNewColumns covers a newer release adding columns to the stats tables while this one is still
// running: every read has to keep working, and so does inserting a game.
func TestReadsTolerateNewColumns(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set a disposable TEST_POSTGRES_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := schema.ApplySchemas(ctx, pool, false); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"ALTER TABLE games ADD COLUMN IF NOT EXISTS live_test_extra smallint",
		"ALTER TABLE guilds ADD COLUMN IF NOT EXISTS live_test_extra smallint",
		"ALTER TABLE users ADD COLUMN IF NOT EXISTS live_test_extra smallint",
		"ALTER TABLE game_events ADD COLUMN IF NOT EXISTS live_test_extra smallint",
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}

	// IDs are unique across the live tests that share TEST_POSTGRES_URL; internal/api uses ...001 onward.
	const guildID, userID uint64 = 900000000000000051, 900000000000000052
	cleanup := func() {
		pool.Exec(ctx, "DELETE FROM game_events WHERE game_id IN (SELECT game_id FROM games WHERE guild_id = $1)", guildID)
		pool.Exec(ctx, "DELETE FROM games WHERE guild_id = $1", guildID)
		pool.Exec(ctx, "DELETE FROM users WHERE user_id = $1", userID)
		pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = $1", guildID)
	}
	cleanup()
	t.Cleanup(cleanup)

	db := &PsqlInterface{Pool: pool}
	if _, err := db.EnsureGuildExists(guildID, "live test"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.EnsureUserExists(userID); err != nil {
		t.Fatal(err)
	}
	gameID, err := db.AddInitialGame(&PostgresGame{GuildID: guildID, ConnectCode: "ABCDEFGH", StartTime: 1, WinType: -1, EndTime: -1})
	if err != nil {
		t.Fatal(err)
	}
	uid := userID
	if err := db.AddEvent(&PostgresGameEvent{UserID: &uid, GameID: int64(gameID), EventTime: 2, EventType: int16(capture.Player), Payload: "{}"}); err != nil {
		t.Fatal(err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if g, err := getGuild(conn.Conn(), guildID); err != nil || g == nil || g.GuildID != guildID {
		t.Errorf("getGuild = %+v, %v", g, err)
	}
	if u, err := db.GetUserByString(strconv.FormatUint(userID, 10)); err != nil || u.UserID != userID {
		t.Errorf("GetUserByString = %+v, %v", u, err)
	}
	if g, err := GuildMatch(ctx, pool, guildID, int64(gameID)); err != nil || g == nil {
		t.Errorf("GuildMatch = %+v, %v", g, err)
	}
	if events, err := MatchEvents(ctx, pool, int64(gameID)); err != nil || len(events) != 1 {
		t.Errorf("MatchEvents = %+v, %v", events, err)
	}
}

// TestAddInitialGame_RecordsCaptureKind covers a games table from before capture kinds were recorded gaining the
// columns, and a match recording which capture client it was played with, or nothing when that's unknown.
func TestAddInitialGame_RecordsCaptureKind(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set a disposable TEST_POSTGRES_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := schema.ApplySchemas(ctx, pool, false); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE games DROP COLUMN capture_kind, DROP COLUMN capture_version"); err != nil {
		t.Fatal(err)
	}
	if err := schema.ApplySchemas(ctx, pool, false); err != nil {
		t.Fatalf("upgrading a games table without capture columns: %v", err)
	}

	const guildID uint64 = 900000000000000061
	cleanup := func() {
		pool.Exec(ctx, "DELETE FROM games WHERE guild_id = $1", guildID)
		pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = $1", guildID)
	}
	cleanup()
	t.Cleanup(cleanup)

	db := &PsqlInterface{Pool: pool}
	if _, err := db.EnsureGuildExists(guildID, "live test"); err != nil {
		t.Fatal(err)
	}
	kind, version := capture.KindMod, "1.0.0"
	for _, want := range []PostgresGame{
		{CaptureKind: &kind, CaptureVersion: &version},
		{},
	} {
		g := want
		g.GuildID, g.ConnectCode, g.StartTime, g.WinType, g.EndTime = guildID, "ABCDEFGH", 1, -1, -1
		gameID, err := db.AddInitialGame(&g)
		if err != nil {
			t.Fatal(err)
		}
		var gotKind, gotVersion *string
		err = pool.QueryRow(ctx, "SELECT capture_kind, capture_version FROM games WHERE game_id = $1", gameID).Scan(&gotKind, &gotVersion)
		if err != nil {
			t.Fatal(err)
		}
		if deref(gotKind) != deref(want.CaptureKind) || deref(gotVersion) != deref(want.CaptureVersion) ||
			(gotKind == nil) != (want.CaptureKind == nil) {
			t.Errorf("recorded %v %v, want %v %v", gotKind, gotVersion, want.CaptureKind, want.CaptureVersion)
		}
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
