package bot

import (
	"context"
	"log"
	"log/slog"
	"time"

	"github.com/automuteus/automuteus/v8/internal/prune"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	storageutils "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/bwmarrin/discordgo"
)

// StartPruning begins leaving the servers that stopped using the bot, among those the given shards serve. bots are
// every shard of this process, all started.
func StartPruning(cfg prune.Config, bots []*Bot) *prune.Sweeper {
	first := bots[0]
	return prune.Start(cfg, first.RedisInterface.client, shardGuilds(bots),
		pruneRecords{psql: first.PostgresInterface, official: first.official}, slog.Default())
}

// shardGuilds is prune's view of Discord: the guild caches of this process's shards.
type shardGuilds []*Bot

func (bots shardGuilds) GuildIDs() []string {
	var ids []string
	for _, bot := range bots {
		state := bot.PrimarySession.State
		state.RLock()
		for _, g := range state.Guilds {
			ids = append(ids, g.ID)
		}
		state.RUnlock()
	}
	return ids
}

func (bots shardGuilds) Guild(id string) (prune.Guild, bool) {
	for _, bot := range bots {
		state := bot.PrimarySession.State
		g, err := state.Guild(id)
		if err != nil {
			continue
		}
		// Gateway events rewrite the cached guild in place under this lock.
		state.RLock()
		defer state.RUnlock()
		return prune.Guild{JoinedAt: g.JoinedAt, Unavailable: g.Unavailable}, true
	}
	return prune.Guild{}, false
}

func (bots shardGuilds) Leave(ctx context.Context, id string) error {
	// Leaving is a REST call, so any shard's session can make it; using one keeps a single rate-limit bucket.
	return bots[0].PrimarySession.GuildLeave(id, discordgo.WithContext(ctx),
		discordgo.WithRetryOnRatelimit(false), discordgo.WithRestRetries(0))
}

type pruneRecords struct {
	psql     *storageutils.PsqlInterface
	official bool
}

func (r pruneRecords) HasPremium(ctx context.Context, guildID string) (bool, error) {
	if !r.official {
		// Self-hosted installs report every guild as premium, which would protect them all.
		return false, nil
	}
	tier, days, err := r.psql.GetGuildPremiumStatus(ctx, true, guildID)
	if err != nil {
		return false, err
	}
	return !premium.IsExpired(tier, days), nil
}

func (r pruneRecords) PlayedSince(ctx context.Context, guildID string, since time.Time) (bool, error) {
	return r.psql.GuildPlayedSince(ctx, guildID, since)
}

// MarkGuildUsed protects a guild from being left for inactivity for prune.Grace.
func (redisInterface *RedisInterface) MarkGuildUsed(guildID string) {
	if err := redisInterface.client.Set(ctx, rediskey.GuildRecentUse(guildID), "1", prune.Grace).Err(); err != nil {
		log.Println(err)
	}
}

// leftForInactivity reports whether the bot left the guild itself, rather than being removed from it. An answer
// Redis cannot give counts as yes, since what rides on it is whether to delete the guild's settings.
func (redisInterface *RedisInterface) leftForInactivity(guildID string) bool {
	pruned, err := redisInterface.client.Exists(ctx, rediskey.GuildPruned(guildID)).Result()
	if err != nil {
		log.Println(err)
		return true
	}
	return pruned != 0
}
