package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
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
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
	"log"
	"strconv"
	"sync"
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
	// profiles resolves stats-page users through Discord with the bot's credentials; nil leaves only cached profiles.
	profiles ProfileFetcher
	config   Config
	// origin names this replica in its stats change announcements, so StatsChanges can skip the echo of what
	// this replica already forgot itself.
	origin string
}

func NewStore(client *redis.Client, pool *pgxpool.Pool, config Config) *DataStore {
	profiles := config.ProfileFetcher
	if profiles == nil && config.BotToken != "" {
		profiles = newDiscordChannelVerifier(config.BotToken)
	}
	return &DataStore{redis: client, postgres: pool, stats: pool, settings: storage.NewPostgresStorage(pool), profiles: profiles, config: config, origin: newOrigin()}
}

// newOrigin is a random ID unique enough to tell replicas apart; it only ever has to differ from theirs.
func newOrigin() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
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

func (s *DataStore) AnnounceStatsChanged(ctx context.Context, guildIDs ...string) error {
	return notice.PublishStatsChanged(ctx, s.redis, notice.StatsChanged{GuildIDs: guildIDs, Origin: s.origin})
}

// StatsChanges subscribes to the bot's stats change announcements and relays them until ctx ends or the Redis
// client closes. This store's own announcements are skipped: the caller forgot before announcing, and forgetting
// again would discard a rebuild already under way. The subscription reconnects by itself; announcements
// published meanwhile are missed, which the stats cache TTL covers.
func (s *DataStore) StatsChanges(ctx context.Context) <-chan notice.StatsChanged {
	sub := notice.SubscribeStatsChanged(ctx, s.redis)
	out := make(chan notice.StatsChanged, 64)
	go func() {
		defer close(out)
		defer sub.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-sub.Channel():
				if !ok {
					return
				}
				change, err := notice.DecodeStatsChanged([]byte(msg.Payload))
				if err != nil {
					log.Printf("malformed stats change announcement: %v", err)
					continue
				}
				if change.Origin != "" && change.Origin == s.origin {
					continue
				}
				select {
				case out <- *change:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

// BotInGuild checks the same Redis set the bot adds to on GuildCreate and removes from on GuildDelete, so the
// API never needs a Discord session or bot token for this answer.
func (s *DataStore) BotInGuild(ctx context.Context, guildID string) (bool, error) {
	if err := discord.ValidateSnowflake(guildID); err != nil {
		return false, err
	}
	return s.redis.SIsMember(ctx, rediskey.TotalGuildsSet, string(rediskey.HashGuildID(guildID))).Result()
}

// BotInGuilds is BotInGuild for many guilds in one pipelined round trip, answering in the order asked. Pipelined
// SISMEMBER rather than SMISMEMBER keeps self-hosts on Redis older than 6.2 working.
func (s *DataStore) BotInGuilds(ctx context.Context, guildIDs []string) ([]bool, error) {
	present := make([]bool, len(guildIDs))
	if len(guildIDs) == 0 {
		return present, nil
	}
	cmds := make([]*redis.BoolCmd, len(guildIDs))
	pipe := s.redis.Pipeline()
	for i, guildID := range guildIDs {
		if err := discord.ValidateSnowflake(guildID); err != nil {
			return nil, err
		}
		cmds[i] = pipe.SIsMember(ctx, rediskey.TotalGuildsSet, string(rediskey.HashGuildID(guildID)))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	for i, cmd := range cmds {
		present[i] = cmd.Val()
	}
	return present, nil
}

// GuildsWithStats reports, in the order asked, which guilds have a finished game for the stats page to show.
func (s *DataStore) GuildsWithStats(ctx context.Context, guildIDs []string) ([]bool, error) {
	ids := make([]uint64, len(guildIDs))
	for i, guildID := range guildIDs {
		id, err := strconv.ParseUint(guildID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("guild ID: %w", err)
		}
		ids[i] = id
	}
	found, err := pgstorage.GuildsWithStats(ctx, s.stats, ids)
	if err != nil {
		return nil, err
	}
	with := make(map[uint64]bool, len(found))
	for _, id := range found {
		with[id] = true
	}
	has := make([]bool, len(ids))
	for i, id := range ids {
		has[i] = with[id]
	}
	return has, nil
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

// subscriptionsUnavailable logs once when premium_subscriptions cannot be read. Self-hosted databases never have
// the payment tables, and the official API role needs SELECT on them (see storage/payments.sql).
var subscriptionsUnavailable sync.Once

func (s *DataStore) Subscription(ctx context.Context, guildID string) (*SubscriptionStatus, error) {
	if !s.config.Official {
		return nil, nil
	}
	id, err := strconv.ParseUint(guildID, 10, 64)
	if err != nil {
		return nil, err
	}
	sub, err := pgstorage.GuildSubscription(ctx, s.stats, id, time.Now())
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "42P01" || pgErr.Code == "42501") { // undefined_table, insufficient_privilege
		subscriptionsUnavailable.Do(func() {
			log.Printf("premium_subscriptions unavailable, premium pages show no subscription status: %v", err)
		})
		return nil, nil
	}
	if err != nil || sub == nil {
		return nil, err
	}
	return &SubscriptionStatus{Status: sub.Status, EndsAt: sub.EndsAt().Unix(), Inherited: sub.Inherited}, nil
}
