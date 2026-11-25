package bot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/internal/prune"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const prunedGuild = "100000000000000002"

// settingsQueries records what the bot asks of the settings table.
type settingsQueries []string

func (q *settingsQueries) Exec(_ context.Context, sql string, _ ...interface{}) (pgconn.CommandTag, error) {
	*q = append(*q, sql)
	return pgconn.NewCommandTag("DELETE 1"), nil
}

func (q *settingsQueries) QueryRow(context.Context, string, ...interface{}) pgx.Row { return nil }

// leavingBot is a bot that is in prunedGuild and has settings stored for it.
func leavingBot(t *testing.T) (*Bot, *miniredis.Miniredis, *settingsQueries) {
	t.Helper()
	mr := miniredis.RunT(t)
	ri := &RedisInterface{}
	if err := ri.Init(storage.RedisParameters{Addr: mr.Addr()}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ri.Close() })
	queries := &settingsQueries{}
	ri.AddUniqueGuildCounter(prunedGuild)
	return &Bot{RedisInterface: ri, StorageInterface: storage.NewPostgresStorage(queries)}, mr, queries
}

func guildDelete(unavailable bool) *discordgo.GuildDelete {
	return &discordgo.GuildDelete{Guild: &discordgo.Guild{ID: prunedGuild, Unavailable: unavailable}}
}

func inGuildCount(t *testing.T, mr *miniredis.Miniredis) int {
	t.Helper()
	if !mr.Exists(rediskey.TotalGuildsSet) {
		return 0
	}
	members, err := mr.Members(rediskey.TotalGuildsSet)
	if err != nil {
		t.Fatal(err)
	}
	return len(members)
}

func TestLeaveGuildRemovedDeletesSettings(t *testing.T) {
	bot, mr, queries := leavingBot(t)
	bot.leaveGuild(nil, guildDelete(false))
	if len(*queries) != 1 || !strings.HasPrefix((*queries)[0], "DELETE FROM guild_settings") {
		t.Errorf("settings queries = %v, want one delete", *queries)
	}
	if n := inGuildCount(t, mr); n != 0 {
		t.Errorf("bot still counted in %d guilds", n)
	}
}

func TestLeaveGuildPrunedKeepsSettings(t *testing.T) {
	bot, mr, queries := leavingBot(t)
	mr.Set(rediskey.GuildPruned(prunedGuild), "1")
	bot.leaveGuild(nil, guildDelete(false))
	if len(*queries) != 0 {
		t.Errorf("settings queries = %v, want none", *queries)
	}
	if n := inGuildCount(t, mr); n != 0 {
		t.Errorf("bot still counted in %d guilds", n)
	}
}

func TestLeaveGuildKeepsSettingsWhenRedisCannotSay(t *testing.T) {
	bot, mr, queries := leavingBot(t)
	mr.SetError("unavailable")
	bot.leaveGuild(nil, guildDelete(false))
	if len(*queries) != 0 {
		t.Errorf("settings queries = %v, want none", *queries)
	}
}

func TestLeaveGuildOutageChangesNothing(t *testing.T) {
	bot, mr, queries := leavingBot(t)
	bot.leaveGuild(nil, guildDelete(true))
	if len(*queries) != 0 {
		t.Errorf("settings queries = %v, want none", *queries)
	}
	if n := inGuildCount(t, mr); n != 1 {
		t.Errorf("bot counted in %d guilds after an outage, want 1", n)
	}
}

func TestMarkGuildUsedLastsTheGracePeriod(t *testing.T) {
	bot, mr, _ := leavingBot(t)
	bot.RedisInterface.MarkGuildUsed(prunedGuild)
	if ttl := mr.TTL(rediskey.GuildRecentUse(prunedGuild)); ttl != prune.Grace {
		t.Errorf("recent-use marker lasts %v, want %v", ttl, prune.Grace)
	}
}

// Each shard caches only its own guilds; the sweep sees them as one list.
func TestShardGuildsReadsEveryShard(t *testing.T) {
	joined := time.Now().Add(-time.Hour).Truncate(time.Second)
	var bots shardGuilds
	for _, g := range []*discordgo.Guild{
		{ID: "1", JoinedAt: joined},
		{ID: "2", Unavailable: true},
	} {
		s, err := discordgo.New("Bot test")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.State.GuildAdd(g); err != nil {
			t.Fatal(err)
		}
		bots = append(bots, &Bot{PrimarySession: s})
	}
	if ids := bots.GuildIDs(); len(ids) != 2 || ids[0] != "1" || ids[1] != "2" {
		t.Fatalf("guild IDs = %v, want [1 2]", ids)
	}
	if g, ok := bots.Guild("1"); !ok || !g.JoinedAt.Equal(joined) || g.Unavailable {
		t.Errorf("guild 1 = %+v, %v", g, ok)
	}
	if g, ok := bots.Guild("2"); !ok || !g.Unavailable || !g.JoinedAt.IsZero() {
		t.Errorf("guild 2 = %+v, %v", g, ok)
	}
	if _, ok := bots.Guild("3"); ok {
		t.Error("reported a guild no shard is in")
	}
}
