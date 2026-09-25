package api

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
)

// TestLiveGuildsWithStats checks the picker's stats lookup against a real Postgres: only finished, unaborted games
// count, and guilds come back in the order asked whatever order the query returns them in.
func TestLiveGuildsWithStats(t *testing.T) {
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

	const finished, aborted, running, empty, unknown = uint64(900000000000000011), uint64(900000000000000012), uint64(900000000000000013), uint64(900000000000000014), uint64(900000000000000015)
	all := []uint64{finished, aborted, running, empty, unknown}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = ANY($1)", all) })
	pool.Exec(ctx, "DELETE FROM guilds WHERE guild_id = ANY($1)", all)
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	for _, g := range []uint64{finished, aborted, running, empty} {
		exec("INSERT INTO guilds (guild_id, guild_name, premium) VALUES ($1, 'picker test', 0)", g)
	}
	exec("INSERT INTO games (guild_id, connect_code, start_time, win_type, end_time) VALUES ($1, 'AAAAAAAA', 1, $2, 2)", finished, int16(game.HumansByTask))
	exec("INSERT INTO games (guild_id, connect_code, start_time, win_type, end_time) VALUES ($1, 'AAAAAAAA', 1, $2, 2)", aborted, int16(game.Aborted))
	exec("INSERT INTO games (guild_id, connect_code, start_time, win_type, end_time) VALUES ($1, 'AAAAAAAA', 1, $2, -1)", running, int16(game.Unknown))

	store := &DataStore{postgres: pool, stats: pool}
	ids := []string{"900000000000000015", "900000000000000014", "900000000000000013", "900000000000000012", "900000000000000011"}
	has, err := store.GuildsWithStats(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	if want := []bool{false, false, false, false, true}; !reflect.DeepEqual(has, want) {
		t.Fatalf("got %v, want %v", has, want)
	}
	if has, err := store.GuildsWithStats(ctx, nil); err != nil || len(has) != 0 {
		t.Fatalf("empty list: %v %v", has, err)
	}
}

// TestLiveBotInGuilds checks the pipelined presence lookup against the set the bot maintains.
func TestLiveBotInGuilds(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set disposable TEST_REDIS_ADDR")
	}
	ctx := context.Background()
	// Separate Redis DB from the live tests running in other packages.
	r := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	t.Cleanup(func() { r.Close() })
	t.Cleanup(func() { r.Del(ctx, rediskey.TotalGuildsSet) })
	in, out := "900000000000000021", "900000000000000022"
	if err := r.SAdd(ctx, rediskey.TotalGuildsSet, string(rediskey.HashGuildID(in))).Err(); err != nil {
		t.Fatal(err)
	}
	store := &DataStore{redis: r}
	present, err := store.BotInGuilds(ctx, []string{out, in, out})
	if err != nil {
		t.Fatal(err)
	}
	if want := []bool{false, true, false}; !reflect.DeepEqual(present, want) {
		t.Fatalf("got %v, want %v", present, want)
	}
	if _, err := store.BotInGuilds(ctx, []string{"not a guild"}); err == nil {
		t.Fatal("accepted an invalid guild ID")
	}
}
