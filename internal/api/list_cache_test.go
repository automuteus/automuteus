package api

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestListCache_ReusesWithinTTLAndRefetchesAfter(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	var calls int32
	cache := newListCache(30*time.Second, func() time.Time { return now }, func(_ context.Context, guildID string) ([]GuildRole, error) {
		atomic.AddInt32(&calls, 1)
		return []GuildRole{{ID: guildID, Name: "r"}}, nil
	})
	for i := 0; i < 5; i++ {
		if list, err := cache.get(context.Background(), writeGuild); err != nil || len(list) != 1 || list[0].ID != writeGuild {
			t.Fatalf("get = %v, %v", list, err)
		}
	}
	if _, err := cache.get(context.Background(), "223456789012345678"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("fetches = %d, want one per guild", calls)
	}
	now = now.Add(29 * time.Second)
	cache.get(context.Background(), writeGuild)
	if calls != 2 {
		t.Fatalf("fetches = %d, want the entry still fresh at 29s", calls)
	}
	now = now.Add(2 * time.Second)
	cache.get(context.Background(), writeGuild)
	if calls != 3 {
		t.Fatalf("fetches = %d, want a refetch after the TTL", calls)
	}
}

func TestListCache_ErrorsAreNotCachedAndDisabledCacheAlwaysFetches(t *testing.T) {
	var calls int32
	fail := true
	fetch := func(context.Context, string) ([]GuildChannel, error) {
		atomic.AddInt32(&calls, 1)
		if fail {
			return nil, errChannelUnavailable
		}
		return []GuildChannel{{ID: textChannelHere}}, nil
	}
	cache := newListCache(time.Minute, nil, fetch)
	if _, err := cache.get(context.Background(), writeGuild); !errors.Is(err, errChannelUnavailable) {
		t.Fatalf("err = %v", err)
	}
	fail = false
	if list, err := cache.get(context.Background(), writeGuild); err != nil || len(list) != 1 {
		t.Fatalf("after a failure the next call must fetch again: %v, %v", list, err)
	}
	if calls != 2 {
		t.Fatalf("fetches = %d", calls)
	}

	calls = 0
	disabled := newListCache(-1, nil, fetch)
	for i := 0; i < 3; i++ {
		disabled.get(context.Background(), writeGuild)
	}
	if calls != 3 {
		t.Fatalf("disabled cache fetched %d times, want 3", calls)
	}
}

func TestListCache_CollapsesConcurrentFetches(t *testing.T) {
	var calls int32
	release := make(chan struct{})
	cache := newListCache(time.Minute, nil, func(context.Context, string) ([]GuildRole, error) {
		atomic.AddInt32(&calls, 1)
		<-release
		return []GuildRole{{ID: modRole}}, nil
	})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if list, err := cache.get(context.Background(), writeGuild); err != nil || len(list) != 1 {
				t.Errorf("get = %v, %v", list, err)
			}
		}()
	}
	// Let every goroutine reach the shared call before it returns.
	for atomic.LoadInt32(&calls) == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls != 1 {
		t.Fatalf("fetches = %d, want 1 for 20 concurrent callers", calls)
	}
}

// The wrappers are what the routes see; PATCH keeps the raw lister.
func TestListCache_WrappersServeTheRoutes(t *testing.T) {
	var roleCalls, channelCalls int32
	roles := cachedRoleLister{newListCache(time.Minute, nil, func(ctx context.Context, g string) ([]GuildRole, error) {
		atomic.AddInt32(&roleCalls, 1)
		return fakeRoles(nil)(ctx, g)
	})}
	channels := cachedChannelLister{newListCache(time.Minute, nil, func(ctx context.Context, g string) ([]GuildChannel, error) {
		atomic.AddInt32(&channelCalls, 1)
		return fakeChannelList(nil)(ctx, g)
	})}
	for i := 0; i < 3; i++ {
		if list, err := roles.ListRoles(context.Background(), writeGuild); err != nil || len(list) != 2 {
			t.Fatal(list, err)
		}
		if list, err := channels.ListChannels(context.Background(), writeGuild); err != nil || len(list) != 2 {
			t.Fatal(list, err)
		}
	}
	if roleCalls != 1 || channelCalls != 1 {
		t.Fatalf("fetches = %d roles, %d channels; want 1 each", roleCalls, channelCalls)
	}
}

// A caller that stops waiting gets errStillBuilding at once; the fetch it started is not cancelled with it, and
// what it fetches is kept for the next caller.
func TestListCache_BuildOutlivesACallerThatGaveUp(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var fetchErr atomic.Value
	var builds int32
	c := newListCache(time.Hour, nil, func(ctx context.Context, _ string) (int, error) {
		atomic.AddInt32(&builds, 1)
		close(started)
		<-release
		if err := ctx.Err(); err != nil {
			fetchErr.Store(err)
		}
		return 42, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.get(ctx, "g"); !errors.Is(err, errStillBuilding) {
		t.Fatalf("err = %v, want errStillBuilding", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("the build never started")
	}
	close(release)

	deadline := time.Now().Add(time.Second)
	for {
		v, err := c.get(context.Background(), "g")
		if err == nil && v == 42 && atomic.LoadInt32(&builds) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("get = %v, %v after %d builds; want 42 from the detached build", v, err, builds)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := fetchErr.Load(); err != nil {
		t.Fatalf("the caller's cancellation reached the fetch: %v", err)
	}
}

// The fetch has its own deadline, so a runaway build cannot hold Postgres forever.
func TestListCache_BuildHasItsOwnDeadline(t *testing.T) {
	c := newListCache(time.Hour, nil, func(ctx context.Context, _ string) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})
	c.setBuildTimeout(20 * time.Millisecond)
	start := time.Now()
	if _, err := c.get(context.Background(), "g"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the build's deadline", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("the build ran past its deadline")
	}
}
