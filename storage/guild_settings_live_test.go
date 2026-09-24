package storage

// These tests need a disposable database named by TEST_POSTGRES_URL. They
// create and delete their own guild records, so never point them at a real
// deployment.

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/jackc/pgx/v4/pgxpool"
)

type liveEnv struct {
	ctx   context.Context
	pool  *pgxpool.Pool
	store *StorageInterface
}

func newLiveEnv(t *testing.T) *liveEnv {
	t.Helper()
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
	if err := ApplyGuildSettingsSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return &liveEnv{ctx: ctx, pool: pool, store: NewPostgresStorage(pool)}
}

// guild reserves a test-specific guild ID and removes its records afterwards.
func (e *liveEnv) guild(t *testing.T, suffix string) (id, hash string) {
	t.Helper()
	id = t.Name() + "/" + suffix
	hash = string(rediskey.HashGuildID(id))
	cleanup := func() {
		e.pool.Exec(e.ctx, "DELETE FROM guild_settings WHERE guild_hash = $1", hash)
	}
	cleanup()
	t.Cleanup(cleanup)
	return id, hash
}

func (e *liveEnv) rowExists(t *testing.T, hash string) bool {
	t.Helper()
	var exists bool
	if err := e.pool.QueryRow(e.ctx, "SELECT EXISTS (SELECT 1 FROM guild_settings WHERE guild_hash = $1)", hash).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
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
	id, hash := e.guild(t, "guild")

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
