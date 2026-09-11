package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	pgstorage "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
)

// DataStore borrows the API process's connections. It reads shared data directly
// and never constructs a Bot or connects to the Discord gateway.
type DataStore struct {
	redis    *redis.Client
	postgres *pgxpool.Pool
	settings *storage.StorageInterface
	config   Config
}

func NewStore(client *redis.Client, pool *pgxpool.Pool, config Config) *DataStore {
	return &DataStore{redis: client, postgres: pool, settings: storage.NewPostgresStorage(pool, client), config: config}
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

func (s *DataStore) Settings(ctx context.Context, guildID string) (*settings.GuildSettings, error) {
	return s.settings.LoadGuildSettings(ctx, guildID)
}

func (s *DataStore) Premium(ctx context.Context, guildID string) (premium.PremiumRecord, error) {
	if err := ctx.Err(); err != nil {
		return premium.PremiumRecord{}, err
	}
	pg := pgstorage.PsqlInterface{Pool: s.postgres}
	tier, days, err := pg.GetGuildOrUserPremiumStatus(s.config.Official, nil, guildID, "")
	return premium.PremiumRecord{Tier: tier, Days: days}, err
}
