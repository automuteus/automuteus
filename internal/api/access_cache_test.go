package api

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAccessCache_ReusesWithinTTLAndExpires(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	c := newAccessCache(time.Minute, func() time.Time { return now })
	calls := 0
	v := verifierFunc(func(_ context.Context, token, guild string) (VerifiedGuildAccess, error) {
		calls++
		return VerifiedGuildAccess{UserID: "u", GuildID: guild, Member: true}, nil
	})
	for i := 0; i < 5; i++ {
		a, err := c.verify(context.Background(), v, "tok", "123456789012345678", false)
		if err != nil || !a.Member {
			t.Fatal(a, err)
		}
	}
	if calls != 1 {
		t.Fatalf("verifier called %d times within the TTL, want 1", calls)
	}
	now = now.Add(59 * time.Second)
	c.verify(context.Background(), v, "tok", "123456789012345678", false)
	if calls != 1 {
		t.Fatalf("expired early: %d calls", calls)
	}
	now = now.Add(2 * time.Second)
	c.verify(context.Background(), v, "tok", "123456789012345678", false)
	if calls != 2 {
		t.Fatalf("did not re-verify after the TTL: %d calls", calls)
	}
}

func TestAccessCache_IsolatesTokensAndGuilds(t *testing.T) {
	c := newAccessCache(time.Minute, nil)
	seen := map[string]int{}
	v := verifierFunc(func(_ context.Context, token, guild string) (VerifiedGuildAccess, error) {
		seen[token+"/"+guild]++
		return VerifiedGuildAccess{UserID: token, GuildID: guild, Member: token == "member"}, nil
	})
	for _, pair := range [][2]string{{"member", "1"}, {"member", "2"}, {"other", "1"}, {"member", "1"}, {"other", "1"}} {
		a, _ := c.verify(context.Background(), v, pair[0], pair[1], false)
		if a.Member != (pair[0] == "member") || a.GuildID != pair[1] {
			t.Fatalf("%v answered %+v", pair, a)
		}
	}
	if len(seen) != 3 || seen["member/1"] != 1 || seen["other/1"] != 1 {
		t.Fatalf("lookups = %v", seen)
	}
}

func TestAccessCache_FreshBypassesAndReplaces(t *testing.T) {
	c := newAccessCache(time.Minute, nil)
	member := true
	calls := 0
	v := verifierFunc(func(_ context.Context, _, guild string) (VerifiedGuildAccess, error) {
		calls++
		return VerifiedGuildAccess{UserID: "u", GuildID: guild, Member: member}, nil
	})
	if a, _ := c.verify(context.Background(), v, "tok", "1", false); !a.Member {
		t.Fatal("first read")
	}
	member = false
	if a, _ := c.verify(context.Background(), v, "tok", "1", false); !a.Member {
		t.Fatal("a cached read should still pass within the TTL")
	}
	if a, _ := c.verify(context.Background(), v, "tok", "1", true); a.Member {
		t.Fatal("a fresh check must see the revocation")
	}
	if a, _ := c.verify(context.Background(), v, "tok", "1", false); a.Member {
		t.Fatal("the fresh answer must replace the cached one")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestAccessCache_ErrorsAreNotCached(t *testing.T) {
	c := newAccessCache(time.Minute, nil)
	fail := true
	calls := 0
	v := verifierFunc(func(_ context.Context, _, guild string) (VerifiedGuildAccess, error) {
		calls++
		if fail {
			return VerifiedGuildAccess{}, errDiscordUnavailable
		}
		return VerifiedGuildAccess{UserID: "u", GuildID: guild, Member: true}, nil
	})
	if _, err := c.verify(context.Background(), v, "tok", "1", false); !errors.Is(err, errDiscordUnavailable) {
		t.Fatal(err)
	}
	fail = false
	if a, err := c.verify(context.Background(), v, "tok", "1", false); err != nil || !a.Member {
		t.Fatal("the next request should retry", a, err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestAccessCache_CollapsesConcurrentLookups(t *testing.T) {
	c := newAccessCache(time.Minute, nil)
	var calls int32
	release := make(chan struct{})
	v := verifierFunc(func(_ context.Context, _, guild string) (VerifiedGuildAccess, error) {
		atomic.AddInt32(&calls, 1)
		<-release
		return VerifiedGuildAccess{UserID: "u", GuildID: guild, Member: true}, nil
	})
	var wg sync.WaitGroup
	results := make([]VerifiedGuildAccess, 8)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], _ = c.verify(context.Background(), v, "tok", "1", false)
		}(i)
	}
	// Let every goroutine reach the shared call before releasing it.
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&calls) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("verifier called %d times for one burst, want 1", n)
	}
	for _, r := range results {
		if !r.Member {
			t.Fatal("a follower did not receive the shared answer")
		}
	}
}

func TestAccessCache_DisabledPassesThrough(t *testing.T) {
	calls := 0
	v := verifierFunc(func(_ context.Context, _, guild string) (VerifiedGuildAccess, error) {
		calls++
		return VerifiedGuildAccess{UserID: "u", GuildID: guild, Member: true}, nil
	})
	var nilCache *accessCache
	for _, c := range []*accessCache{nilCache, newAccessCache(0, nil), newAccessCache(-time.Second, nil)} {
		c.verify(context.Background(), v, "tok", "1", false)
		c.verify(context.Background(), v, "tok", "1", false)
	}
	if calls != 6 {
		t.Fatalf("disabled cache still cached: %d calls", calls)
	}
}

// At the router: a burst of reads costs one verification, a write always verifies live, and a revocation is
// honoured by the write immediately and by reads afterwards.
func TestRouter_ReadsCachedWritesLive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	member := true
	calls := 0
	v := verifierFunc(func(_ context.Context, _, guild string) (VerifiedGuildAccess, error) {
		calls++
		return VerifiedGuildAccess{UserID: "u", GuildID: guild, Member: member, Owner: true}, nil
	})
	r := NewRouter(Config{GuildVerifier: v, AdminPassword: "test-password"}, &fakeStore{})
	for _, path := range []string{"/guild/settings", "/guild/premium", "/guild/bot", "/guild/settings"} {
		if w := bearerRequest(r, path+"?guildID="+writeGuild, "valid"); w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	if calls != 1 {
		t.Fatalf("reads verified %d times, want 1", calls)
	}
	if w := patchSettings(r, "valid", `{"autoRefresh":true}`); w.Code != 200 || calls != 2 {
		t.Fatalf("write should verify live: %d, calls %d", w.Code, calls)
	}
	member = false
	if w := patchSettings(r, "valid", `{"autoRefresh":false}`); w.Code != 403 {
		t.Fatalf("revoked write passed: %d", w.Code)
	}
	if w := bearerRequest(r, "/guild/settings?guildID="+writeGuild, "valid"); w.Code != 403 {
		t.Fatalf("read after a refused write still passed: %d", w.Code)
	}
	// A different token is its own entry.
	member = true
	if w := bearerRequest(r, "/guild/settings?guildID="+writeGuild, "other"); w.Code != 200 || calls != 4 {
		t.Fatalf("other token: %d, calls %d", w.Code, calls)
	}
}

func TestAccessCache_RevokedTokenIsEvicted(t *testing.T) {
	for _, definitive := range []error{errInvalidToken, errDiscordForbidden} {
		c := newAccessCache(time.Minute, nil)
		var fail error
		calls := 0
		v := verifierFunc(func(_ context.Context, _, guild string) (VerifiedGuildAccess, error) {
			calls++
			if fail != nil {
				return VerifiedGuildAccess{}, fail
			}
			return VerifiedGuildAccess{UserID: "u", GuildID: guild, Member: true}, nil
		})
		if a, _ := c.verify(context.Background(), v, "tok", "1", false); !a.Member {
			t.Fatal("first read")
		}
		fail = definitive
		if _, err := c.verify(context.Background(), v, "tok", "1", true); !errors.Is(err, definitive) {
			t.Fatalf("fresh check: %v", err)
		}
		if _, err := c.verify(context.Background(), v, "tok", "1", false); !errors.Is(err, definitive) {
			t.Fatalf("a read after a definitive failure reused the cache: %v", err)
		}
		if calls != 3 {
			t.Fatalf("calls = %d, want 3", calls)
		}
		// A transient failure, by contrast, keeps the entry.
		c = newAccessCache(time.Minute, nil)
		fail = nil
		c.verify(context.Background(), v, "tok", "1", false)
		fail = errDiscordUnavailable
		c.verify(context.Background(), v, "tok", "1", true)
		if a, err := c.verify(context.Background(), v, "tok", "1", false); err != nil || !a.Member {
			t.Fatalf("outage evicted a good entry: %+v %v", a, err)
		}
	}
}
