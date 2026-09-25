package api

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/go-redis/redis/v8"
)

const (
	profileGuild = "123456789012345678"
	alice        = "223456789012345678"
	bob          = "323456789012345678"
	carol        = "423456789012345678"
)

func TestDiscordProfileFetcher(t *testing.T) {
	stub := (&discordStub{}).
		reply("/guilds/"+profileGuild+"/members/"+alice, 200, `{"nick":"Al","avatar":"0123456789abcdef0123456789abcdef","user":{"id":"`+alice+`","username":"alice","global_name":"Alice","discriminator":"0","avatar":"fedcba9876543210fedcba9876543210"}}`).
		reply("/guilds/"+profileGuild+"/members/"+bob, 404, `{"message":"Unknown Member"}`).
		reply("/users/"+bob, 200, `{"id":"`+bob+`","username":"bob","discriminator":"0","avatar":null}`).
		reply("/guilds/"+profileGuild+"/members/"+carol, 404, `{}`).
		reply("/users/"+carol, 404, `{"message":"Unknown User"}`)
	srv := stub.server(t)
	defer srv.Close()
	v := &discordChannelVerifier{client: srv.Client(), baseURL: srv.URL, token: "bot-token"}

	got, err := v.FetchProfile(context.Background(), profileGuild, alice)
	if err != nil {
		t.Fatal(err)
	}
	want := StatsPlayer{Username: "alice", GlobalName: "Alice", Nickname: "Al",
		Avatar: "https://cdn.discordapp.com/guilds/" + profileGuild + "/users/" + alice + "/avatars/0123456789abcdef0123456789abcdef.png?size=128"}
	if got != want {
		t.Errorf("member = %+v, want %+v", got, want)
	}

	// A user who left the guild still gets a name from their global record, and the default avatar Discord
	// shows for someone who never set one (by ID under the new username system).
	got, err = v.FetchProfile(context.Background(), profileGuild, bob)
	if err != nil {
		t.Fatal(err)
	}
	if want := (StatsPlayer{Username: "bob", Avatar: DefaultAvatarURL(bob, "0")}); got != want {
		t.Errorf("former member = %+v, want %+v", got, want)
	}
	if _, err := v.FetchProfile(context.Background(), profileGuild, carol); !errors.Is(err, errChannelNotFound) {
		t.Errorf("unknown user err = %v, want not found", err)
	}
	if _, err := v.FetchProfile(context.Background(), profileGuild, "nope"); !errors.Is(err, errChannelNotFound) {
		t.Errorf("malformed ID err = %v, want not found", err)
	}
}

func TestRateLimitBucket(t *testing.T) {
	for path, want := range map[string]string{
		"/guilds/" + profileGuild + "/members/" + alice: "/guilds/" + profileGuild + "/members/*",
		"/guilds/" + profileGuild + "/members/" + bob:   "/guilds/" + profileGuild + "/members/*",
		"/guilds/" + profileGuild + "/roles":            "/guilds/" + profileGuild + "/roles",
		"/users/" + alice:                               "/users/*",
		"/users/@me":                                    "/users/@me",
		"/channels/" + alice:                            "/channels/" + alice,
	} {
		if got := rateLimitBucket(path); got != want {
			t.Errorf("bucket(%s) = %s, want %s", path, got, want)
		}
	}
}

// One member lookup being rate limited pauses every member lookup in that guild: Discord's limit is per route
// and guild, and each refused call counts toward the invalid-request limit that blocks the whole host.
func TestProfileCooldownCoversTheGuildsMembers(t *testing.T) {
	stub := (&discordStub{}).
		reply("/guilds/"+profileGuild+"/members/"+alice, 429, `{"message":"You are being rate limited.","retry_after":2.5}`).
		reply("/guilds/"+profileGuild+"/members/"+bob, 200, `{"nick":"","user":{"id":"`+bob+`","username":"bob","discriminator":"0"}}`).
		reply("/users/"+alice, 200, `{"id":"`+alice+`","username":"alice","discriminator":"0"}`)
	srv := stub.server(t)
	defer srv.Close()
	v := &discordChannelVerifier{client: srv.Client(), baseURL: srv.URL, token: "bot-token"}

	if _, err := v.FetchProfile(context.Background(), profileGuild, alice); !errors.Is(err, errChannelUnavailable) {
		t.Fatalf("rate limited lookup err = %v", err)
	}
	before := len(stub.calls)
	if _, err := v.FetchProfile(context.Background(), profileGuild, bob); !errors.Is(err, errChannelUnavailable) {
		t.Fatalf("lookup during cooldown err = %v, want unavailable", err)
	}
	if len(stub.calls) != before {
		t.Fatalf("Discord was called during the cooldown: %v", stub.calls[before:])
	}
	// The user route is a different bucket and is still allowed.
	if err := v.get(context.Background(), "/users/"+alice, &discordUser{}); err != nil {
		t.Fatalf("user route blocked by the member cooldown: %v", err)
	}
}

type stallingFetcher struct{ calls int32 }

func (f *stallingFetcher) FetchProfile(ctx context.Context, _, _ string) (StatsPlayer, error) {
	atomic.AddInt32(&f.calls, 1)
	<-ctx.Done()
	return StatsPlayer{}, errChannelUnavailable
}

// A Discord that never answers must not hold the stats response: the budget expires, the unresolved users are
// left for the page to show by ID, and nothing is cached as a miss.
func TestResolvePlayersGivesUpWithinBudget(t *testing.T) {
	previous := profileFetchBudget
	profileFetchBudget = 50 * time.Millisecond
	t.Cleanup(func() { profileFetchBudget = previous })
	ctx := context.Background()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	f := &stallingFetcher{}
	start := time.Now()
	players := resolvePlayers(ctx, client, f, profileGuild, []string{alice, bob, carol})
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("resolution took %v", elapsed)
	}
	if len(players) != 0 {
		t.Errorf("players = %+v, want none", players)
	}
	for _, id := range []string{alice, bob, carol} {
		if mr.Exists(rediskey.CachedPlayerProfile(id, profileGuild)) {
			t.Errorf("%s was cached after a timeout", id)
		}
	}
	if atomic.LoadInt32(&f.calls) != 3 {
		t.Errorf("fetcher called %d times, want 3", f.calls)
	}
}

func TestAvatarURLsRejectBadHashes(t *testing.T) {
	u := &discordUser{ID: alice, Avatar: "../evil", Discriminator: "0"}
	if got := userAvatarURL(u); got != DefaultAvatarURL(alice, "0") {
		t.Errorf("bad hash produced %s", got)
	}
	if got := DefaultAvatarURL(alice, "1234"); got != "https://cdn.discordapp.com/embed/avatars/4.png" {
		t.Errorf("legacy default = %s", got)
	}
	// 223456789012345678 >> 22, mod 6, is 0.
	if got := DefaultAvatarURL(alice, "0"); got != "https://cdn.discordapp.com/embed/avatars/0.png" {
		t.Errorf("modern default = %s", got)
	}
}

type fakeFetcher struct {
	mu       sync.Mutex
	profiles map[string]StatsPlayer
	err      error
	calls    []string
}

func (f *fakeFetcher) FetchProfile(_ context.Context, _, userID string) (StatsPlayer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, userID)
	if f.err != nil {
		return StatsPlayer{}, f.err
	}
	p, ok := f.profiles[userID]
	if !ok {
		return StatsPlayer{}, errChannelNotFound
	}
	return p, nil
}

func TestResolvePlayers(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	fetcher := &fakeFetcher{profiles: map[string]StatsPlayer{
		alice: {Username: "alice", Nickname: "Al", Avatar: "https://cdn.discordapp.com/embed/avatars/0.png"},
	}}
	// bob and carol are unknown to Discord.

	players := resolvePlayers(ctx, client, fetcher, profileGuild, []string{alice, bob, carol})
	if players[alice] != fetcher.profiles[alice] {
		t.Errorf("alice = %+v", players[alice])
	}
	if len(players) != 1 {
		t.Errorf("unknown users resolved: %+v", players)
	}
	if len(fetcher.calls) != 3 {
		t.Fatalf("Discord asked %d times, want once per user: %v", len(fetcher.calls), fetcher.calls)
	}

	// Second round: alice is served from the profile cache, and the misses for bob and carol are remembered, so
	// Discord is not asked at all.
	fetcher.calls = nil
	players = resolvePlayers(ctx, client, fetcher, profileGuild, []string{alice, bob, carol})
	if players[alice] != fetcher.profiles[alice] || len(players) != 1 {
		t.Errorf("second round = %+v", players)
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("second round asked Discord about %v, want nobody", fetcher.calls)
	}
	if mr.TTL(rediskey.CachedPlayerProfile(alice, profileGuild)) != profileCacheTTL {
		t.Errorf("profile TTL = %v", mr.TTL(rediskey.CachedPlayerProfile(alice, profileGuild)))
	}
	if mr.TTL(rediskey.CachedPlayerProfile(carol, profileGuild)) != profileMissTTL || mr.TTL(rediskey.CachedPlayerProfile(bob, profileGuild)) != profileMissTTL {
		t.Errorf("miss TTL = %v", mr.TTL(rediskey.CachedPlayerProfile(carol, profileGuild)))
	}

	// Discord being down resolves nobody and caches nothing.
	mr.FlushAll()
	down := &fakeFetcher{err: errChannelUnavailable}
	players = resolvePlayers(ctx, client, down, profileGuild, []string{alice, bob})
	if len(players) != 0 {
		t.Errorf("with Discord down = %+v", players)
	}
	if mr.Exists(rediskey.CachedPlayerProfile(bob, profileGuild)) {
		t.Error("an unavailable lookup was cached")
	}

	// Without a fetcher (no bot token) the profile cache is still honoured, as the seed tool fills it.
	cacheProfile(ctx, client, profileGuild, alice, StatsPlayer{Username: "seeded"}, profileCacheTTL)
	players = resolvePlayers(ctx, client, nil, profileGuild, []string{alice, bob, carol})
	if players[alice].Username != "seeded" || len(players) != 1 {
		t.Errorf("without fetcher = %+v", players)
	}

	// No Redis and no fetcher: nothing is known, and nothing panics.
	if got := resolvePlayers(ctx, nil, nil, profileGuild, []string{alice}); len(got) != 0 {
		t.Errorf("bare = %+v", got)
	}
}
