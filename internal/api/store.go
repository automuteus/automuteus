package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/locale"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	pgstorage "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	"time"
)

// DataStore borrows the API process's connections. It reads shared data directly
// and never constructs a Bot or connects to the Discord gateway.
type DataStore struct {
	redis    *redis.Client
	postgres *pgxpool.Pool
	// stats is the pool again, typed so tests can substitute a mock for the statistics queries.
	stats    pgxscan.Querier
	settings *storage.StorageInterface
	// profiles resolves stats-page users through Discord with the bot's credentials; nil leaves only cached names.
	profiles ProfileFetcher
	config   Config
}

func NewStore(client *redis.Client, pool *pgxpool.Pool, config Config) *DataStore {
	profiles := config.ProfileFetcher
	if profiles == nil && config.BotToken != "" {
		profiles = newDiscordChannelVerifier(config.BotToken)
	}
	return &DataStore{redis: client, postgres: pool, stats: pool, settings: storage.NewPostgresStorage(pool, client), profiles: profiles, config: config}
}

func (s *DataStore) ActiveNotice(ctx context.Context) (*notice.Notice, error) {
	return notice.Active(ctx, s.redis)
}

func (s *DataStore) RaiseNotice(ctx context.Context, n notice.Notice) error {
	return notice.Raise(ctx, s.redis, n)
}

func (s *DataStore) ClearNotice(ctx context.Context) error {
	return notice.Clear(ctx, s.redis)
}

// BotInGuild checks the same Redis set the bot adds to on GuildCreate and removes from on GuildDelete, so the
// API never needs a Discord session or bot token for this answer.
func (s *DataStore) BotInGuild(ctx context.Context, guildID string) (bool, error) {
	if err := discord.ValidateSnowflake(guildID); err != nil {
		return false, err
	}
	return s.redis.SIsMember(ctx, rediskey.TotalGuildsSet, string(rediskey.HashGuildID(guildID))).Result()
}

func (s *DataStore) Ping(ctx context.Context) error {
	if err := s.redis.Ping(ctx).Err(); err != nil {
		return err
	}
	return s.postgres.Ping(ctx)
}

func (s *DataStore) Info(ctx context.Context) (Info, error) {
	info := Info{Version: s.config.Version, Commit: s.config.Commit}
	var err error
	if info.TotalGuilds, err = rediskey.CountGuilds(ctx, s.redis); err != nil {
		return info, err
	}
	if info.ActiveGames, err = rediskey.CountActiveGames(ctx, s.redis, rediskey.ActiveGameTimeoutSeconds); err != nil {
		return info, err
	}
	if info.TotalUsers, err = rediskey.CountTotalUsers(ctx, s.redis, s.postgres); err != nil {
		return info, err
	}
	info.TotalGames, err = rediskey.CountTotalGames(ctx, s.redis, s.postgres)
	return info, err
}

func (s *DataStore) GameState(ctx context.Context, guildID, connectCode string) (json.RawMessage, error) {
	key, err := s.redis.Get(ctx, rediskey.ConnectCodePtr(guildID, connectCode)).Result()
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, redis.Nil
	}
	blob, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(blob)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return nil, errors.New("invalid stored game state")
	}
	// Return the bot's serialized document directly, including future fields and
	// exact numeric IDs, without coupling the API to bot runtime state types.
	return json.RawMessage(blob), nil
}

func (s *DataStore) RoomCode(ctx context.Context, connectCode string) (string, error) {
	return s.redis.Get(ctx, rediskey.RoomCodesForConnCode(connectCode)).Result()
}

func (s *DataStore) Settings(ctx context.Context, guildID string) (*settings.GuildSettings, storage.SettingsVersion, error) {
	return s.settings.LoadGuildSettingsVersion(ctx, guildID)
}

// SetSettings persists a complete settings document for one guild. It is the
// last line of defence before storage: whatever the handler checked, a document
// that fails settings.Validate (against the embedded language set) or a guild ID
// that is not a snowflake is refused here, so an invalid document can never
// reach Postgres through the API. A settings.ValidationErrors is returned in
// that case so the caller can tell client mistakes from storage failures. The
// write is conditional on expected, the version returned by Settings; a
// concurrent change is storage.ErrSettingsConflict and nothing is written.
func (s *DataStore) SetSettings(ctx context.Context, guildID string, sett *settings.GuildSettings, expected storage.SettingsVersion) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := discord.ValidateSnowflake(guildID); err != nil {
		return errors.New("invalid guild ID")
	}
	if err := sett.Validate(locale.GetLanguages()); err != nil {
		return err
	}
	return s.settings.SetGuildSettingsIfVersion(ctx, guildID, sett, expected)
}

// settingsWriteBudget increments the guild's counter, starts the window on the first hit, and returns the count
// and the seconds left in the window, atomically, so a crash between INCR and EXPIRE can never leave a guild
// locked out for good.
var settingsWriteBudget = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('EXPIRE', KEYS[1], ARGV[1]) end
return {n, redis.call('TTL', KEYS[1])}
`)

// ReserveSettingsWrite fails closed: if Redis cannot be reached the write is refused rather than allowed
// unmetered, matching how the bot treats Redis everywhere else.
func (s *DataStore) ReserveSettingsWrite(ctx context.Context, guildID string) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	key := rediskey.APISettingsWriteLimit(guildID)
	raw, err := settingsWriteBudget.Run(ctx, s.redis, []string{key}, int(SettingsWriteWindow.Seconds())).Result()
	if err != nil {
		return 0, fmt.Errorf("settings write budget: %w", err)
	}
	reply, ok := raw.([]interface{})
	if !ok || len(reply) != 2 {
		return 0, fmt.Errorf("settings write budget: unexpected reply %v", reply)
	}
	count, countOK := reply[0].(int64)
	ttl, ttlOK := reply[1].(int64)
	if !countOK || !ttlOK {
		return 0, fmt.Errorf("settings write budget: unexpected reply %v", reply)
	}
	if count <= SettingsWriteLimit {
		return 0, nil
	}
	if ttl <= 0 {
		// The key has no expiry (it should always have one); refuse for a full window rather than forever.
		ttl = int64(SettingsWriteWindow.Seconds())
	}
	return time.Duration(ttl) * time.Second, nil
}

func (s *DataStore) Premium(ctx context.Context, guildID string) (premium.PremiumRecord, error) {
	if err := ctx.Err(); err != nil {
		return premium.PremiumRecord{}, err
	}
	pg := pgstorage.PsqlInterface{Pool: s.postgres}
	tier, days, err := pg.GetGuildOrUserPremiumStatus(s.config.Official, nil, guildID, "")
	return premium.PremiumRecord{Tier: tier, Days: days}, err
}
