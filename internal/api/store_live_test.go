package api

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/bot"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
)

func TestLiveAPIWithoutBotProcess(t *testing.T) {
	url, addr := os.Getenv("TEST_POSTGRES_URL"), os.Getenv("TEST_REDIS_ADDR")
	if url == "" || addr == "" {
		t.Skip("set disposable TEST_POSTGRES_URL and TEST_REDIS_ADDR")
	}
	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	// Exercise concurrent startup on a fresh DB: the API must not need the bot
	// to initialize the schema before serving requests.
	results := make(chan error, 3)
	for i := 0; i < cap(results); i++ {
		go func() { results <- storage.ApplySchemas(ctx, pool, false) }()
	}
	for i := 0; i < cap(results); i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	// Separate Redis DB from the migration sweep tests running in other packages.
	r := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	defer r.Close()
	config := Config{Version: "api-test", Commit: "test-commit", AdminPassword: "test-password"}
	store := NewStore(r, pool, config)
	router := NewRouter(config, store)
	guildID, code := "987654321012345678", "APITEST1"
	hash := string(rediskey.HashGuildID(guildID))
	key := rediskey.ConnectCodeData(guildID, code)
	pointer := rediskey.ConnectCodePtr(guildID, code)
	legacyKey := rediskey.GuildSettings(rediskey.HashedID(hash))
	defer pool.Exec(ctx, "DELETE FROM guild_settings WHERE guild_hash=$1", hash)
	pool.Exec(ctx, "DELETE FROM guild_settings WHERE guild_hash=$1", hash)
	keys := []string{key, pointer, legacyKey, rediskey.RoomCodesForConnCode(code), rediskey.TotalGuildsSet, rediskey.ActiveGamesZSet, rediskey.TotalUsers, rediskey.TotalGames}
	defer r.Del(ctx, keys...)
	state := bot.NewDiscordGameState(guildID)
	state.ConnectCode = code
	state.Running = true
	state.MatchID = 9007199254740993
	blob, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]interface{}{key: blob, pointer: key, rediskey.RoomCodesForConnCode(code): "ABCDEF"} {
		if err := r.Set(ctx, k, v, 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
	r.SAdd(ctx, rediskey.TotalGuildsSet, hash)
	r.ZAdd(ctx, rediskey.ActiveGamesZSet, &redis.Z{Score: float64(time.Now().Unix()), Member: code})
	// Force /bot/info to exercise SQL count fallback and populate shared caches.
	r.Del(ctx, rediskey.TotalUsers, rediskey.TotalGames)
	w := request(t, router, "/bot/info", false)
	if w.Code != 200 || strings.Contains(w.Body.String(), "shard") || !strings.Contains(w.Body.String(), `"activeGames":1`) {
		t.Fatalf("info: %d %s", w.Code, w.Body)
	}
	if r.Exists(ctx, rediskey.TotalUsers, rediskey.TotalGames).Val() != 2 {
		t.Fatal("counts were not cached")
	}
	w = request(t, router, "/game/state?guildID="+guildID+"&connectCode="+code, true)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != string(blob) {
		t.Fatalf("game state changed: %d %s", w.Code, w.Body)
	}
	w = request(t, router, "/game/roomcode?connectCode="+code, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"roomCode":"ABCDEF"`) {
		t.Fatalf("roomcode: %d %s", w.Code, w.Body)
	}
	sett := settings.MakeGuildSettings()
	sett.SetLanguage("de")
	legacy, err := json.Marshal(sett)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Set(ctx, legacyKey, legacy, 0).Err(); err != nil {
		t.Fatal(err)
	}
	w = request(t, router, "/guild/settings?guildID="+guildID, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"language":"de"`) {
		t.Fatalf("settings: %d %s", w.Code, w.Body)
	}
	if r.Exists(ctx, legacyKey).Val() != 0 {
		t.Fatal("API did not migrate legacy settings on read")
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM guild_settings WHERE guild_hash=$1", hash).Scan(&count); err != nil || count != 1 {
		t.Fatalf("migration row: %d %v", count, err)
	}
	for _, path := range []string{"/guild/premium?guildID=" + guildID, "/ready"} {
		if w := request(t, router, path, true); w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	r.Set(ctx, key, "not json", 0)
	if w := request(t, router, "/game/state?guildID="+guildID+"&connectCode="+code, true); w.Code != 500 {
		t.Fatal("invalid stored state was served")
	}
}
