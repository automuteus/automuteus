package storage

// This test needs a disposable database named by TEST_POSTGRES_URL. It adds
// columns to the stats tables, so never point it at a real deployment.

import (
	"context"
	"os"
	"strconv"
	"testing"

	schema "github.com/automuteus/automuteus/v8/storage"
	"github.com/jackc/pgx/v4/pgxpool"
)

// TestReadsTolerateNewColumns covers a newer release adding columns to the stats tables while this one is still
// running: every read has to keep working, and so does inserting a game.
func TestReadsTolerateNewColumns(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set a disposable TEST_POSTGRES_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
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

	const guildID, userID uint64 = 900000000000000001, 900000000000000002
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
	if err := db.AddEvent(&PostgresGameEvent{UserID: &uid, GameID: int64(gameID), EventTime: 2, EventType: 1, Payload: "{}"}); err != nil {
		t.Fatal(err)
	}

	if g, err := db.GetGuildForDownload(guildID); err != nil || g == nil || g.GuildID != guildID {
		t.Errorf("GetGuildForDownload = %+v, %v", g, err)
	}
	if u, err := db.GetUserByString(strconv.FormatUint(userID, 10)); err != nil || u.UserID != userID {
		t.Errorf("GetUserByString = %+v, %v", u, err)
	}
	if games, err := db.GetGamesForGuild(guildID); err != nil || len(games) != 1 {
		t.Errorf("GetGamesForGuild = %+v, %v", games, err)
	}
	if events, err := db.GetGamesEventsForGuild(guildID); err != nil || len(events) != 1 {
		t.Errorf("GetGamesEventsForGuild = %+v, %v", events, err)
	}
}
