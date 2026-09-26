package notice

import (
	"context"
	"encoding/json"

	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/go-redis/redis/v8"
)

// StatsChanged announces that the recorded match history behind some guilds' stats pages changed: a match was
// recorded or aborted, a guild or player was reset, or a user opted out (which touches every guild they played in).
// It is published on its own channel, separate from platform events, so the bot shards never have to read it.
type StatsChanged struct {
	// GuildIDs names the guilds affected. Empty means every guild.
	GuildIDs []string `json:"guildIDs,omitempty"`
	// Origin identifies the publisher, so a subscriber that already dropped its own caches before announcing can
	// skip the echo instead of dropping them a second time. Empty (the bot's announcements) is never skipped.
	Origin string `json:"origin,omitempty"`
}

// All reports whether the change may affect any guild.
func (c StatsChanged) All() bool {
	return len(c.GuildIDs) == 0
}

// AnnounceStatsChanged tells stats caches that the named guilds' history changed, or every guild's when none is
// named. It is fire-and-forget: a subscriber that is reconnecting misses it, and its cache TTL covers that.
func AnnounceStatsChanged(ctx context.Context, client *redis.Client, guildIDs ...string) error {
	return PublishStatsChanged(ctx, client, StatsChanged{GuildIDs: guildIDs})
}

// PublishStatsChanged is AnnounceStatsChanged for a publisher that wants to name itself in Origin.
func PublishStatsChanged(ctx context.Context, client *redis.Client, change StatsChanged) error {
	b, err := json.Marshal(change)
	if err != nil {
		return err
	}
	return client.Publish(ctx, rediskey.StatsChangedChannel, b).Err()
}

// SubscribeStatsChanged returns a subscription to stats change announcements. Callers must Close it.
func SubscribeStatsChanged(ctx context.Context, client *redis.Client) *redis.PubSub {
	return client.Subscribe(ctx, rediskey.StatsChangedChannel)
}

// DecodeStatsChanged parses one published announcement.
func DecodeStatsChanged(b []byte) (*StatsChanged, error) {
	var c StatsChanged
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
