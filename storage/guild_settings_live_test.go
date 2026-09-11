package storage

// These tests need disposable databases named by TEST_POSTGRES_URL and
// TEST_REDIS_ADDR. They create and delete their own guild records and scan
// the legacy settings namespace, so never point them at a real deployment.

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
)

type liveEnv struct {
	ctx   context.Context
	pool  *pgxpool.Pool
	redis *redis.Client
	store *StorageInterface
}

func newLiveEnv(t *testing.T) *liveEnv {
	t.Helper()
	url, addr := os.Getenv("TEST_POSTGRES_URL"), os.Getenv("TEST_REDIS_ADDR")
	if url == "" || addr == "" {
		t.Skip("set disposable TEST_POSTGRES_URL and TEST_REDIS_ADDR")
	}
	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := ApplyGuildSettingsSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	r := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { r.Close() })
	return &liveEnv{ctx: ctx, pool: pool, redis: r, store: NewPostgresStorage(pool, r)}
}

// guild reserves a test-specific guild ID and removes its records afterwards.
func (e *liveEnv) guild(t *testing.T, suffix string) (id, hash, key string) {
	t.Helper()
	id = t.Name() + "/" + suffix
	hash = string(rediskey.HashGuildID(id))
	key = rediskey.GuildSettings(rediskey.HashedID(hash))
	cleanup := func() {
		e.redis.Del(e.ctx, key)
		e.pool.Exec(e.ctx, "DELETE FROM guild_settings WHERE guild_hash = $1", hash)
	}
	cleanup()
	t.Cleanup(cleanup)
	return id, hash, key
}

func (e *liveEnv) rowExists(t *testing.T, hash string) bool {
	t.Helper()
	var exists bool
	if err := e.pool.QueryRow(e.ctx, "SELECT EXISTS (SELECT 1 FROM guild_settings WHERE guild_hash = $1)", hash).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
}

func (e *liveEnv) keyExists(t *testing.T, key string) bool {
	t.Helper()
	n, err := e.redis.Exists(e.ctx, key).Result()
	if err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func (e *liveEnv) load(t *testing.T, id string) *settings.GuildSettings {
	t.Helper()
	sett, err := e.store.LoadGuildSettings(e.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return sett
}

func TestLiveRoundTrip(t *testing.T) {
	e := newLiveEnv(t)
	id, hash, _ := e.guild(t, "guild")

	assertSettingsEqual(t, settings.MakeGuildSettings(), e.load(t, id))
	if e.rowExists(t, hash) {
		t.Fatal("reading an unknown guild created a row")
	}

	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, blob string }{
		{"custom", string(fixture)},
		{"legacy", `{"language":"en","adminIDs":[]}`},
		{"zero", `{}`},
		{"explicit false and zero", `{"autoRefresh":false,"leaderboardMention":false,"leaderboardSize":0,"delays":{"delays":{"LOBBY":{"TASKS":0}}},"voiceRules":{"MuteRules":{"TASKS":{"alive":false}}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var expected settings.GuildSettings
			if err := json.Unmarshal([]byte(tc.blob), &expected); err != nil {
				t.Fatal(err)
			}
			if err := e.store.SetGuildSettings(id, &expected); err != nil {
				t.Fatal(err)
			}
			got := e.load(t, id)
			assertSettingsEqual(t, &expected, got)
			if got.GetLeaderboardSize() != expected.GetLeaderboardSize() || got.GetDisplayRoomCode() != expected.GetDisplayRoomCode() {
				t.Fatal("getter behavior changed")
			}
			// Each read must return an independently mutable object.
			got.SetLanguage("changed")
			assertSettingsEqual(t, &expected, e.load(t, id))
		})
	}

	// Saving the defaults stores NULL documents, and one custom delay
	// stores a full document while voice rules stay NULL.
	if err := e.store.SetGuildSettings(id, settings.MakeGuildSettings()); err != nil {
		t.Fatal(err)
	}
	var voiceNull, delaysNull bool
	nullQuery := "SELECT voice_rules IS NULL, delays IS NULL FROM guild_settings WHERE guild_hash = $1"
	if err := e.pool.QueryRow(e.ctx, nullQuery, hash).Scan(&voiceNull, &delaysNull); err != nil {
		t.Fatal(err)
	}
	if !voiceNull || !delaysNull {
		t.Fatal("default documents were stored")
	}
	assertSettingsEqual(t, settings.MakeGuildSettings(), e.load(t, id))
	custom := settings.MakeGuildSettings()
	custom.SetDelay(0, 1, 42)
	if err := e.store.SetGuildSettings(id, custom); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(e.ctx, nullQuery, hash).Scan(&voiceNull, &delaysNull); err != nil {
		t.Fatal(err)
	}
	if !voiceNull || delaysNull {
		t.Fatal("custom delays were not stored as a document")
	}
	assertSettingsEqual(t, custom, e.load(t, id))

	// Reapplying the schema changes nothing.
	if err := ApplyGuildSettingsSchema(e.ctx, e.pool); err != nil {
		t.Fatal(err)
	}
	assertSettingsEqual(t, custom, e.load(t, id))

	if err := e.store.DeleteGuildSettings(id); err != nil {
		t.Fatal(err)
	}
	if e.rowExists(t, hash) {
		t.Fatal("delete left the row")
	}
	assertSettingsEqual(t, settings.MakeGuildSettings(), e.load(t, id))
}

func TestLiveLazyMigrationOnFirstRead(t *testing.T) {
	e := newLiveEnv(t)
	id, hash, key := e.guild(t, "guild")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.redis.Set(e.ctx, key, fixture, 0).Err(); err != nil {
		t.Fatal(err)
	}

	assertSettingsEqual(t, loadFixture(t), e.load(t, id))
	if !e.rowExists(t, hash) {
		t.Fatal("legacy record was not moved to Postgres")
	}
	if e.keyExists(t, key) {
		t.Fatal("legacy record was not removed from Redis")
	}
	// The second read is served by Postgres and sees the same settings.
	assertSettingsEqual(t, loadFixture(t), e.load(t, id))
}

func TestLiveConcurrentFirstReads(t *testing.T) {
	e := newLiveEnv(t)
	id, hash, key := e.guild(t, "guild")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.redis.Set(e.ctx, key, fixture, 0).Err(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make([]*settings.GuildSettings, 8)
	errs := make([]error, len(results))
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = e.store.LoadGuildSettings(e.ctx, id)
		}(i)
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		assertSettingsEqual(t, loadFixture(t), results[i])
	}
	if !e.rowExists(t, hash) || e.keyExists(t, key) {
		t.Fatal("concurrent first reads did not complete the move")
	}
}

func TestLiveExistingRowBeatsLegacyRecord(t *testing.T) {
	e := newLiveEnv(t)
	id, _, key := e.guild(t, "guild")
	current := settings.MakeGuildSettings()
	current.SetLanguage("de")
	if err := e.store.SetGuildSettings(id, current); err != nil {
		t.Fatal(err)
	}
	// A leftover legacy record must never shadow settings written by the new code.
	if err := e.redis.Set(e.ctx, key, `{"language":"fr"}`, 0).Err(); err != nil {
		t.Fatal(err)
	}
	assertSettingsEqual(t, current, e.load(t, id))

	// Writes and deletes clear the leftover so it cannot resurface later.
	if err := e.store.SetGuildSettings(id, current); err != nil {
		t.Fatal(err)
	}
	if e.keyExists(t, key) {
		t.Fatal("set left the legacy record")
	}
	if err := e.redis.Set(e.ctx, key, `{"language":"fr"}`, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := e.store.DeleteGuildSettings(id); err != nil {
		t.Fatal(err)
	}
	if e.keyExists(t, key) {
		t.Fatal("delete left the legacy record")
	}
	assertSettingsEqual(t, settings.MakeGuildSettings(), e.load(t, id))
}

func TestLiveUnreadableLegacyRecordUsesDefaultsAndIsRetained(t *testing.T) {
	e := newLiveEnv(t)
	id, hash, key := e.guild(t, "guild")
	if err := e.redis.Set(e.ctx, key, `{"language":42}`, 0).Err(); err != nil {
		t.Fatal(err)
	}
	assertSettingsEqual(t, settings.MakeGuildSettings(), e.load(t, id))
	if e.rowExists(t, hash) {
		t.Fatal("unreadable record was imported")
	}
	if !e.keyExists(t, key) {
		t.Fatal("unreadable record was deleted")
	}
}

func TestLiveSweep(t *testing.T) {
	e := newLiveEnv(t)
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	// Four records: one to move, one whose guild already has a row, one with
	// an unknown field, and one under a malformed key.
	freshID, freshHash, freshKey := e.guild(t, "fresh")
	existingID, existingHash, existingKey := e.guild(t, "existing")
	_, _, unknownKey := e.guild(t, "unknown")
	badKey := legacySettingsPrefix + "not-a-hash"
	t.Cleanup(func() { e.redis.Del(e.ctx, badKey) })

	current := settings.MakeGuildSettings()
	current.SetLanguage("de")
	if err := e.store.SetGuildSettings(existingID, current); err != nil {
		t.Fatal(err)
	}
	for key, blob := range map[string]string{
		freshKey:    string(fixture),
		existingKey: `{"language":"fr"}`,
		unknownKey:  `{"futureSetting":true}`,
		badKey:      `{}`,
	} {
		if err := e.redis.Set(e.ctx, key, blob, 0).Err(); err != nil {
			t.Fatal(err)
		}
	}

	report, err := SweepLegacySettings(e.ctx, e.redis, nil, true)
	if err == nil || report.Examined != 4 || report.Inserted != 0 || report.Deleted != 0 || len(report.Failures) != 2 {
		t.Fatalf("dry run: %+v, %v", report, err)
	}
	if e.rowExists(t, freshHash) || !e.keyExists(t, freshKey) {
		t.Fatal("dry run wrote data")
	}

	report, err = SweepLegacySettings(e.ctx, e.redis, e.store, false)
	if err == nil || report.Examined != 4 || report.Inserted != 1 || report.Deleted != 2 || len(report.Failures) != 2 {
		t.Fatalf("sweep: %+v, %v", report, err)
	}
	if report.Failures[unknownKey] == "" || report.Failures[badKey] == "" {
		t.Fatalf("wrong failures reported: %v", report.Failures)
	}
	assertSettingsEqual(t, loadFixture(t), e.load(t, freshID))
	assertSettingsEqual(t, current, e.load(t, existingID))
	if e.keyExists(t, freshKey) || e.keyExists(t, existingKey) {
		t.Fatal("moved records were not removed from Redis")
	}
	if !e.keyExists(t, unknownKey) || !e.keyExists(t, badKey) {
		t.Fatal("failed records were removed from Redis")
	}
	if !e.rowExists(t, freshHash) || !e.rowExists(t, existingHash) {
		t.Fatal("rows missing after sweep")
	}

	// A second sweep only sees the retained failures.
	report, err = SweepLegacySettings(e.ctx, e.redis, e.store, false)
	if err == nil || report.Examined != 2 || report.Inserted != 0 || report.Deleted != 0 || len(report.Failures) != 2 {
		t.Fatalf("second sweep: %+v, %v", report, err)
	}
}

func TestLiveConcurrentSchemaApplication(t *testing.T) {
	e := newLiveEnv(t)
	results := make(chan error, 4)
	for i := 0; i < cap(results); i++ {
		go func() { results <- ApplyGuildSettingsSchema(e.ctx, e.pool) }()
	}
	for i := 0; i < cap(results); i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}
