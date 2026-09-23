package storage

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

//go:embed guild_settings.sql
var GuildSettingsSchema string

const settingsTimeout = 5 * time.Second

type settingsDB interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

type RedisParameters struct {
	Addr     string
	Username string
	Password string
}

// StorageInterface persists guild settings in Postgres. A guild with no row
// uses the built-in defaults. When a legacy Redis client is supplied, a guild
// missing from Postgres is read from its old Redis record on first access, and
// that record is moved into Postgres and removed from Redis at the same time.
type StorageInterface struct {
	db     settingsDB
	legacy *redis.Client
}

// NewPostgresStorage borrows both connections; it closes neither. legacy may
// be nil once no deployment has settings left in Redis.
func NewPostgresStorage(db settingsDB, legacy *redis.Client) *StorageInterface {
	return &StorageInterface{db: db, legacy: legacy}
}

// ApplyGuildSettingsSchema is idempotent and safe to run from several
// processes at once; the schema takes an advisory lock.
func ApplyGuildSettingsSchema(ctx context.Context, db settingsDB) error {
	_, err := db.Exec(ctx, GuildSettingsSchema)
	return err
}

var settingsColumns = []string{
	"admin_user_ids",
	"permission_role_ids",
	"language",
	"voice_rules",
	"map_version",
	"delays",
	"delete_game_summary_minutes",
	"unmute_dead_during_tasks",
	"auto_refresh",
	"match_summary_channel_id",
	"leaderboard_mention",
	"leaderboard_size",
	"leaderboard_min",
	"mute_spectator",
	"display_room_code",
}

var (
	// selectColumns is what a read returns: every setting plus the row version, in one snapshot.
	selectColumns  = append(append([]string{}, settingsColumns...), "version")
	selectSettings = "SELECT " + strings.Join(selectColumns, ", ") + " FROM guild_settings WHERE guild_hash = $1"
	insertSettings = buildInsert()
	upsertSettings = insertSettings + " ON CONFLICT (guild_hash) DO UPDATE SET " + buildUpdateList() + ", version = guild_settings.version + 1, updated_at = now()"
	insertIfAbsent = insertSettings + " ON CONFLICT (guild_hash) DO NOTHING"
)

func buildInsert() string {
	placeholders := make([]string, 0, len(settingsColumns)+1)
	for i := range settingsColumns {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+2))
	}
	return "INSERT INTO guild_settings (guild_hash, " + strings.Join(settingsColumns, ", ") + ") VALUES ($1, " + strings.Join(placeholders, ", ") + ")"
}

func buildUpdateList() string {
	assignments := make([]string, 0, len(settingsColumns))
	for _, column := range settingsColumns {
		assignments = append(assignments, column+" = EXCLUDED."+column)
	}
	return strings.Join(assignments, ", ")
}

// LoadGuildSettings returns an error for storage or decoding failures rather
// than substituting defaults, so callers never act on fabricated settings.
func (s *StorageInterface) LoadGuildSettings(ctx context.Context, guildID string) (*settings.GuildSettings, error) {
	sett, _, err := s.LoadGuildSettingsVersion(ctx, guildID)
	return sett, err
}

// LoadGuildSettingsVersion is LoadGuildSettings plus the version of the row the
// settings came from, read in the same query so the two can never disagree; a
// conditional write with that version therefore only succeeds if the row is
// exactly what was read. A guild with no row reports NoSettingsRow.
func (s *StorageInterface) LoadGuildSettingsVersion(ctx context.Context, guildID string) (*settings.GuildSettings, SettingsVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, settingsTimeout)
	defer cancel()
	hash := string(rediskey.HashGuildID(guildID))

	result, version, err := s.loadPostgres(ctx, hash)
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, version, err
	}
	if s.legacy == nil {
		return settings.MakeGuildSettings(), NoSettingsRow, nil
	}

	key := rediskey.GuildSettings(rediskey.HashedID(hash))
	blob, err := s.legacy.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		// No legacy record either. But a concurrent first reader may have moved
		// it into Postgres between our two reads (its insert happens before its
		// Redis delete), so look at Postgres once more before concluding that
		// the guild has never changed a setting.
		result, version, err := s.loadPostgres(ctx, hash)
		if errors.Is(err, pgx.ErrNoRows) {
			return settings.MakeGuildSettings(), NoSettingsRow, nil
		}
		return result, version, err
	}
	if err != nil {
		return nil, NoSettingsRow, fmt.Errorf("read legacy settings: %w", err)
	}
	legacy, err := decodeLegacySettings(blob, false)
	if err != nil {
		// The old reader treated an unreadable record as defaults. Keep that,
		// and leave the record in place so the sweep can report it.
		log.Printf("Legacy Redis settings for guild %s are unreadable; using defaults: %v\n", guildID, err)
		return settings.MakeGuildSettings(), NoSettingsRow, nil
	}
	inserted, err := importLegacySettings(ctx, s.db, s.legacy, key, hash, legacy)
	if err != nil {
		// The Redis record is still there, so the next read retries the move.
		log.Printf("Could not move legacy settings for guild %s to Postgres: %v\n", guildID, err)
		if inserted {
			// The row exists now (only the Redis delete failed) and a fresh row is at version 1.
			return legacy, 1, nil
		}
		return legacy, NoSettingsRow, nil
	}
	if !inserted {
		// Another writer created the row first; it is authoritative.
		return s.loadPostgres(ctx, hash)
	}
	return legacy, 1, nil
}

// SetGuildSettings replaces the stored settings with its own timeout. Callers
// that have a request context should use SetGuildSettingsContext instead.
func (s *StorageInterface) SetGuildSettings(guildID string, sett *settings.GuildSettings) error {
	return s.SetGuildSettingsContext(context.Background(), guildID, sett)
}

// SetGuildSettingsContext replaces the stored settings, bounded by ctx and the
// storage timeout, whichever ends first. Documents equal to the built-in
// defaults are stored as NULL. It does not validate: the slash commands check
// each field as it is set, and the API validates the whole document before
// calling this.
func (s *StorageInterface) SetGuildSettingsContext(ctx context.Context, guildID string, sett *settings.GuildSettings) error {
	if sett == nil {
		return errors.New("nil guild settings")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, settingsTimeout)
	defer cancel()
	hash := string(rediskey.HashGuildID(guildID))
	args, err := settingsArgs(hash, sett)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx, upsertSettings, args...); err != nil {
		return err
	}
	// Postgres is read first, so a legacy record that outlives this delete is
	// inert; it is only removed here to keep Redis tidy.
	if err := s.deleteLegacy(ctx, hash); err != nil {
		log.Println(err)
	}
	return nil
}

// DeleteGuildSettings returns the guild to the built-in defaults. The legacy
// record goes first: if it outlived the row, the next read would revive it.
func (s *StorageInterface) DeleteGuildSettings(guildID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), settingsTimeout)
	defer cancel()
	hash := string(rediskey.HashGuildID(guildID))
	if err := s.deleteLegacy(ctx, hash); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, "DELETE FROM guild_settings WHERE guild_hash = $1", hash)
	return err
}

func (s *StorageInterface) deleteLegacy(ctx context.Context, hash string) error {
	if s.legacy == nil {
		return nil
	}
	if err := s.legacy.Del(ctx, rediskey.GuildSettings(rediskey.HashedID(hash))).Err(); err != nil {
		return fmt.Errorf("delete legacy settings: %w", err)
	}
	return nil
}

func (s *StorageInterface) loadPostgres(ctx context.Context, hash string) (*settings.GuildSettings, SettingsVersion, error) {
	var (
		result     settings.GuildSettings
		voiceRules []byte
		delays     []byte
		version    int64
	)
	err := s.db.QueryRow(ctx, selectSettings, hash).Scan(
		&result.AdminUserIDs,
		&result.PermissionRoleIDs,
		&result.Language,
		&voiceRules,
		&result.MapVersion,
		&delays,
		&result.DeleteGameSummaryMinutes,
		&result.UnmuteDeadDuringTasks,
		&result.AutoRefresh,
		&result.MatchSummaryChannelID,
		&result.LeaderboardMention,
		&result.LeaderboardSize,
		&result.LeaderboardMin,
		&result.MuteSpectator,
		&result.DisplayRoomCode,
		&version,
	)
	if err != nil {
		return nil, NoSettingsRow, err
	}
	if voiceRules == nil {
		result.VoiceRules = game.MakeMuteAndDeafenRules()
	} else if err := json.Unmarshal(voiceRules, &result.VoiceRules); err != nil {
		return nil, NoSettingsRow, fmt.Errorf("decode voice rules: %w", err)
	}
	if delays == nil {
		result.Delays = game.MakeDefaultDelays()
	} else if err := json.Unmarshal(delays, &result.Delays); err != nil {
		return nil, NoSettingsRow, fmt.Errorf("decode delays: %w", err)
	}
	if version < 1 {
		return nil, NoSettingsRow, fmt.Errorf("invalid settings version %d", version)
	}
	return &result, SettingsVersion(version), nil
}

// settingsArgs orders values to match settingsColumns, after guild_hash.
func settingsArgs(hash string, sett *settings.GuildSettings) ([]interface{}, error) {
	voiceRules, err := documentUnlessDefault(sett.VoiceRules, game.MakeMuteAndDeafenRules())
	if err != nil {
		return nil, err
	}
	delays, err := documentUnlessDefault(sett.Delays, game.MakeDefaultDelays())
	if err != nil {
		return nil, err
	}
	return []interface{}{
		hash,
		sett.AdminUserIDs,
		sett.PermissionRoleIDs,
		sett.Language,
		voiceRules,
		sett.MapVersion,
		delays,
		sett.DeleteGameSummaryMinutes,
		sett.UnmuteDeadDuringTasks,
		sett.AutoRefresh,
		sett.MatchSummaryChannelID,
		sett.LeaderboardMention,
		sett.LeaderboardSize,
		sett.LeaderboardMin,
		sett.MuteSpectator,
		sett.DisplayRoomCode,
	}, nil
}

// documentUnlessDefault returns nil (SQL NULL) when value serializes exactly
// like the built-in default, and the JSON document otherwise.
func documentUnlessDefault(value, defaultValue interface{}) (interface{}, error) {
	blob, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	defaultBlob, err := json.Marshal(defaultValue)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(blob, defaultBlob) {
		return nil, nil
	}
	return blob, nil
}

// decodeLegacySettings decodes a Redis record into a zero struct exactly as
// the old reader did, so fields absent from old records keep their zero
// values instead of acquiring today's defaults. strict additionally rejects
// unknown fields, which the typed columns would otherwise silently drop.
func decodeLegacySettings(blob []byte, strict bool) (*settings.GuildSettings, error) {
	trimmed := bytes.TrimSpace(blob)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, errors.New("settings must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	if strict {
		decoder.DisallowUnknownFields()
	}
	var result settings.GuildSettings
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, errors.New("unexpected data after settings object")
	}
	return &result, nil
}

// importLegacySettings inserts a legacy record if the guild has no row yet,
// then removes the Redis record. Postgres is never overwritten: a row that
// already exists was written by the new code and is authoritative.
func importLegacySettings(ctx context.Context, db settingsDB, client *redis.Client, key, hash string, sett *settings.GuildSettings) (bool, error) {
	args, err := settingsArgs(hash, sett)
	if err != nil {
		return false, err
	}
	tag, err := db.Exec(ctx, insertIfAbsent, args...)
	if err != nil {
		return false, err
	}
	if err := client.Del(ctx, key).Err(); err != nil {
		return tag.RowsAffected() == 1, fmt.Errorf("delete legacy settings: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
