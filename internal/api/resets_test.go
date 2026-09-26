package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

const resetUser = "223456789012345678"

func postReset(r http.Handler, path, token, ifMatch string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

var resetPaths = []string{
	"/guild/stats/reset?guildID=" + writeGuild,
	"/guild/user/reset?guildID=" + writeGuild + "&userID=" + resetUser,
	"/guild/settings/reset?guildID=" + writeGuild,
}

func TestResets_DeniedCallersNeverReachStorage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		access VerifiedGuildAccess
		token  string
		status int
	}{
		{"moderator without a settings permission", memberAccess, "valid", 403},
		{"plain member", VerifiedGuildAccess{UserID: "999", GuildID: writeGuild, Member: true}, "valid", 403},
		{"owner of another guild", VerifiedGuildAccess{UserID: "999", GuildID: "323456789012345678", Member: true, Owner: true}, "valid", 403},
		{"invalid token", ownerAccess, "expired", 401},
		{"no token", ownerAccess, "", 401},
	} {
		for _, path := range resetPaths {
			t.Run(tc.name+" "+path, func(t *testing.T) {
				s := &fakeStore{}
				r, _ := writeRouter(s, tc.access)
				if w := postReset(r, path, tc.token, ""); w.Code != tc.status {
					t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body)
				}
				if s.calls != 0 {
					t.Fatal("denied request reached storage")
				}
			})
		}
	}
}

// A player may reset their own stats in a guild they belong to, and nothing else.
func TestResets_PlayersMayResetOnlyThemselves(t *testing.T) {
	self := VerifiedGuildAccess{UserID: resetUser, GuildID: writeGuild, Member: true}
	s := &fakeStore{}
	r, verifierCalls := writeRouter(s, self)
	for i := 0; i < 2; i++ {
		if w := postReset(r, resetPaths[1], "valid", ""); w.Code != 200 {
			t.Fatalf("own reset: %d %s", w.Code, w.Body)
		}
	}
	if *verifierCalls != 2 {
		t.Errorf("verifier called %d times, want 2: membership is checked live", *verifierCalls)
	}
	if strings.Join(s.resets, ",") != writeGuild+"/"+resetUser+","+writeGuild+"/"+resetUser {
		t.Fatalf("resets %v", s.resets)
	}
	for _, tc := range []struct {
		name   string
		access VerifiedGuildAccess
		path   string
	}{
		{"another player", self, "/guild/user/reset?guildID=" + writeGuild + "&userID=323456789012345678"},
		{"the whole guild", self, resetPaths[0]},
		{"the settings", self, resetPaths[2]},
		{"after leaving the guild", VerifiedGuildAccess{UserID: resetUser, GuildID: writeGuild}, resetPaths[1]},
		{"with no user named", self, "/guild/user/reset?guildID=" + writeGuild},
	} {
		s := &fakeStore{}
		r, _ := writeRouter(s, tc.access)
		if w := postReset(r, tc.path, "valid", ""); w.Code != 403 {
			t.Errorf("%s: %d %s", tc.name, w.Code, w.Body)
		}
		if s.calls != 0 {
			t.Errorf("%s reached storage", tc.name)
		}
	}
}

func TestResets_ManagersMayResetAndAreVerifiedEveryTime(t *testing.T) {
	manager := memberAccess
	manager.Permissions = discordgo.PermissionManageServer
	for name, access := range map[string]VerifiedGuildAccess{"owner": ownerAccess, "administrator": adminAccess, "manage server": manager} {
		for _, path := range resetPaths {
			s := &fakeStore{}
			r, verifierCalls := writeRouter(s, access)
			for i := 0; i < 2; i++ {
				if w := postReset(r, path, "valid", ""); w.Code != 200 {
					t.Fatalf("%s %s: status %d: %s", name, path, w.Code, w.Body)
				}
			}
			// A permission removed in Discord must stop the next reset, so the access cache is never used.
			if *verifierCalls != 2 {
				t.Errorf("%s %s: verifier called %d times, want 2", name, path, *verifierCalls)
			}
		}
	}
}

func TestResets_InvalidIDsBeforeStorage(t *testing.T) {
	s := &fakeStore{}
	r, _ := writeRouter(s, ownerAccess)
	for _, path := range []string{
		"/guild/stats/reset?guildID=abc",
		"/guild/user/reset?guildID=" + writeGuild,
		"/guild/user/reset?guildID=" + writeGuild + "&userID=42",
		"/guild/settings/reset?guildID=",
	} {
		if w := postReset(r, path, "valid", ""); w.Code != 400 && w.Code != 401 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	if s.calls != 0 {
		t.Fatal("invalid request reached storage")
	}
}

func TestResetStats_DeletesAndReports(t *testing.T) {
	s := &fakeStore{}
	r, _ := writeRouter(s, ownerAccess)
	w := postReset(r, "/guild/stats/reset?guildID="+writeGuild, "valid", "")
	var got StatsReset
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil || got != (StatsReset{GuildID: writeGuild, Games: 5}) {
		t.Fatalf("guild reset: %d %s", w.Code, w.Body)
	}
	w = postReset(r, "/guild/user/reset?guildID="+writeGuild+"&userID="+resetUser, "valid", "")
	got = StatsReset{}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil || got != (StatsReset{GuildID: writeGuild, UserID: resetUser, Games: 2}) {
		t.Fatalf("player reset: %d %s", w.Code, w.Body)
	}
	if strings.Join(s.resets, ",") != writeGuild+","+writeGuild+"/"+resetUser {
		t.Fatalf("resets %v", s.resets)
	}
	// the other replicas are told to drop the guild too, once per reset
	if len(s.announced) != 2 || len(s.announced[0]) != 1 || s.announced[0][0] != writeGuild || len(s.announced[1]) != 1 || s.announced[1][0] != writeGuild {
		t.Fatalf("announced %v, want the guild twice", s.announced)
	}
}

func TestResetStats_FailedAnnouncementDoesNotFailTheReset(t *testing.T) {
	s := &fakeStore{announceErr: errors.New("redis: connection refused")}
	r, _ := writeRouter(s, ownerAccess)
	w := postReset(r, "/guild/stats/reset?guildID="+writeGuild, "valid", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "redis") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if len(s.announced) != 1 {
		t.Fatalf("announced %v, want one attempt", s.announced)
	}
}

func TestResetStats_StoreFailureIs500WithoutDetail(t *testing.T) {
	s := &fakeStore{resetErr: errors.New("postgres: connection refused")}
	r, _ := writeRouter(s, ownerAccess)
	for _, path := range resetPaths[:2] {
		if w := postReset(r, path, "valid", ""); w.Code != 500 || strings.Contains(w.Body.String(), "postgres") {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
}

// After a reset the stats pages must not keep showing the deleted games until the cache expires.
func TestResetStats_ForgetsTheGuildsCachedDocuments(t *testing.T) {
	const other = "323456789012345678"
	s := &fakeStore{}
	v := verifierFunc(func(_ context.Context, _, guild string) (VerifiedGuildAccess, error) {
		return VerifiedGuildAccess{UserID: "999", GuildID: guild, Member: true, Owner: true}, nil
	})
	r := NewRouter(Config{GuildVerifier: v, StatsCacheTTL: time.Hour}, s)
	load := func() {
		t.Helper()
		for _, path := range []string{
			"/guild/stats?guildID=" + writeGuild,
			"/guild/stats?guildID=" + other,
			"/guild/match?guildID=" + writeGuild + "&matchID=42",
			"/guild/user?guildID=" + writeGuild + "&userID=" + resetUser,
		} {
			if w := bearerRequest(r, path, "token"); w.Code != 200 {
				t.Fatalf("%s: %d %s", path, w.Code, w.Body)
			}
		}
	}
	load()
	load()
	if s.statsCalls != 2 || s.matchCalls != 1 || s.userCalls != 1 {
		t.Fatalf("not cached: stats %d match %d user %d", s.statsCalls, s.matchCalls, s.userCalls)
	}
	for i, path := range resetPaths[:2] {
		if w := postReset(r, path, "token", ""); w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		load()
		// Only the reset guild is built again; the other guild stays cached.
		if s.statsCalls != 3+i || s.matchCalls != 2+i || s.userCalls != 2+i {
			t.Fatalf("after %s: stats %d match %d user %d", path, s.statsCalls, s.matchCalls, s.userCalls)
		}
	}
}

// A build that was already reading when the reset landed answers its callers but is not kept.
func TestListCache_ForgetDiscardsAFetchInFlight(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	builds := 0
	var mu sync.Mutex
	c := newListCache(time.Hour, nil, func(context.Context, string) (int, error) {
		mu.Lock()
		builds++
		n := builds
		mu.Unlock()
		if n == 1 {
			close(started)
			<-release
		}
		return n, nil
	})
	done := make(chan int)
	go func() {
		v, _ := c.get(context.Background(), "g")
		done <- v
	}()
	<-started
	c.forget(func(string) bool { return true })
	close(release)
	if v := <-done; v != 1 {
		t.Fatalf("in-flight caller got %d", v)
	}
	if v, _ := c.get(context.Background(), "g"); v != 2 {
		t.Fatalf("stale build was cached: got %d", v)
	}
	if v, _ := c.get(context.Background(), "g"); v != 2 {
		t.Fatalf("fresh build was not cached: got %d", v)
	}
}

// A change in one guild must not throw away another guild's build. A large guild's rollup outlives the requests
// that want it, and if every game finished elsewhere discarded it, the guild would 503 and rebuild forever.
func TestListCache_ForgetOfAnotherKeyKeepsAFetchInFlight(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var builds int32
	c := newListCache(time.Hour, nil, func(_ context.Context, key string) (string, error) {
		n := atomic.AddInt32(&builds, 1)
		if key == "a" && n == 1 {
			close(started)
			<-release
		}
		return fmt.Sprintf("%s%d", key, n), nil
	})
	done := make(chan string)
	go func() {
		v, _ := c.get(context.Background(), "a")
		done <- v
	}()
	<-started
	// b's own change: its entry and any build of b or of its matches and players, never a's.
	forgetB := func(key string) bool { return key == "b" || strings.HasPrefix(key, "b/") }
	c.forget(forgetB)
	if _, err := c.get(context.Background(), "b"); err != nil {
		t.Fatal(err)
	}
	c.forget(forgetB)
	close(release)
	if v := <-done; v != "a1" {
		t.Fatalf("in-flight caller got %q", v)
	}
	if v, _ := c.get(context.Background(), "a"); v != "a1" {
		t.Fatalf("a's build was discarded by b's changes: got %q", v)
	}
	if v, _ := c.get(context.Background(), "b"); v != "b3" {
		t.Fatalf("b was not rebuilt after its own change: got %q", v)
	}
	if n := atomic.LoadInt32(&builds); n != 3 {
		t.Fatalf("builds = %d, want a once and b twice", n)
	}
}

// A forget that matches the key of a build in flight, whether for that guild or for every guild, discards it.
func TestListCache_ForgetMatchingTheKeyDiscardsAFetchInFlight(t *testing.T) {
	for name, match := range map[string]func(string) bool{
		"guild": func(key string) bool { return key == "g" },
		"all":   func(string) bool { return true },
	} {
		t.Run(name, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			var builds int32
			c := newListCache(time.Hour, nil, func(context.Context, string) (int32, error) {
				n := atomic.AddInt32(&builds, 1)
				if n == 1 {
					close(started)
					<-release
				}
				return n, nil
			})
			done := make(chan struct{})
			go func() {
				defer close(done)
				c.get(context.Background(), "g")
			}()
			<-started
			c.forget(match)
			close(release)
			<-done
			if v, _ := c.get(context.Background(), "g"); v != 2 {
				t.Fatalf("stale build was cached: got %d", v)
			}
		})
	}
}

// The leaderboard minimum is built into every stats document, so a settings write that moves it must drop the
// guild's documents here and tell the other replicas, or the boards show the old cut-off for the rest of the TTL.
func TestSettingsWrite_LeaderboardMinChangeDropsAndAnnouncesStats(t *testing.T) {
	t.Run("PATCH", func(t *testing.T) {
		s := &fakeStore{}
		r, _ := writeRouter(s, ownerAccess)
		statsPath := "/guild/stats?guildID=" + writeGuild
		userPath := "/guild/user?guildID=" + writeGuild + "&userID=223456789012345678"
		for _, path := range []string{statsPath, userPath, statsPath} {
			if w := bearerRequest(r, path, "valid"); w.Code != 200 {
				t.Fatalf("%s: %d %s", path, w.Code, w.Body)
			}
		}
		if s.statsCalls != 1 || s.userCalls != 1 {
			t.Fatalf("builds before the change: guild %d, user %d; want one each", s.statsCalls, s.userCalls)
		}
		// a change to another setting leaves the documents cached
		if w := patchSettings(r, "valid", `{"unmuteDeadDuringTasks":true}`); w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		bearerRequest(r, statsPath, "valid")
		if s.statsCalls != 1 || len(s.announced) != 0 {
			t.Fatalf("an unrelated setting: builds %d, announced %v; want the rollup kept and nothing announced", s.statsCalls, s.announced)
		}
		// echoing the stored minimum back is not a change either
		if w := patchSettings(r, "valid", `{"leaderboardMin":3}`); w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		bearerRequest(r, statsPath, "valid")
		if s.statsCalls != 1 || len(s.announced) != 0 {
			t.Fatalf("the same minimum: builds %d, announced %v; want the rollup kept and nothing announced", s.statsCalls, s.announced)
		}
		s.stored = s.saved
		if w := patchSettings(r, "valid", `{"leaderboardMin":5}`); w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		if len(s.announced) != 1 || len(s.announced[0]) != 1 || s.announced[0][0] != writeGuild {
			t.Fatalf("announced %v, want the guild once", s.announced)
		}
		for _, path := range []string{statsPath, userPath} {
			if w := bearerRequest(r, path, "valid"); w.Code != 200 {
				t.Fatalf("%s: %d %s", path, w.Code, w.Body)
			}
		}
		if s.statsCalls != 2 || s.userCalls != 2 {
			t.Fatalf("builds after the change: guild %d, user %d; want both rebuilt", s.statsCalls, s.userCalls)
		}
	})

	t.Run("reset", func(t *testing.T) {
		changed := settings.MakeGuildSettings()
		changed.SetLeaderboardMin(5)
		s := &fakeStore{stored: changed}
		r, _ := writeRouter(s, ownerAccess)
		statsPath := "/guild/stats?guildID=" + writeGuild
		bearerRequest(r, statsPath, "valid")
		if w := postReset(r, "/guild/settings/reset?guildID="+writeGuild, "valid", ""); w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		if len(s.announced) != 1 || len(s.announced[0]) != 1 || s.announced[0][0] != writeGuild {
			t.Fatalf("announced %v, want the guild once", s.announced)
		}
		bearerRequest(r, statsPath, "valid")
		if s.statsCalls != 2 {
			t.Fatalf("builds = %d, want the rollup rebuilt with the default minimum", s.statsCalls)
		}
		// a reset of a guild already at the default minimum changes no document
		s.stored = s.saved
		if w := postReset(r, "/guild/settings/reset?guildID="+writeGuild, "valid", ""); w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		bearerRequest(r, statsPath, "valid")
		if s.statsCalls != 2 || len(s.announced) != 1 {
			t.Fatalf("a reset at the default: builds %d, announced %v; want the rollup kept and no new announcement", s.statsCalls, s.announced)
		}
	})
}

func TestResetSettings_StoresDefaultsAndBumpsVersion(t *testing.T) {
	changed := settings.MakeGuildSettings()
	changed.Language = "de"
	changed.MatchSummaryChannelID = "423456789012345678"
	// A guild whose premium lapsed may still reset: the defaults need no premium.
	s := &fakeStore{stored: changed, version: 4, premium: &premium.PremiumRecord{Tier: premium.FreeTier}}
	r, _ := writeRouter(s, ownerAccess)
	w := postReset(r, "/guild/settings/reset?guildID="+writeGuild, "valid", `"4"`)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if s.saved == nil || s.savedGuild != writeGuild || s.saved.Language != settings.MakeGuildSettings().Language || s.saved.MatchSummaryChannelID != "" {
		t.Fatalf("defaults not saved: %+v", s.saved)
	}
	if w.Header().Get("ETag") != `"5"` || s.version != 5 {
		t.Errorf("ETag %s, version %d", w.Header().Get("ETag"), s.version)
	}
	if s.premiumCalls != 0 {
		t.Error("reset checked premium")
	}
	if resp := decodeSettings(t, w); resp.Language != settings.MakeGuildSettings().Language {
		t.Errorf("response is not the defaults: %+v", resp)
	}
}

func TestResetSettings_Refusals(t *testing.T) {
	t.Run("stale If-Match", func(t *testing.T) {
		s := &fakeStore{version: 4}
		r, _ := writeRouter(s, ownerAccess)
		w := postReset(r, "/guild/settings/reset?guildID="+writeGuild, "valid", `"3"`)
		if w.Code != 412 || s.saved != nil || w.Header().Get("ETag") != `"4"` {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	})
	t.Run("rate limited", func(t *testing.T) {
		s := &fakeStore{writeRetryAfter: 90 * time.Second}
		r, _ := writeRouter(s, ownerAccess)
		w := postReset(r, "/guild/settings/reset?guildID="+writeGuild, "valid", "")
		if w.Code != 429 || s.saved != nil || w.Header().Get("Retry-After") != "90" {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	})
	t.Run("storage failure", func(t *testing.T) {
		s := &fakeStore{saveErr: errFakeStore}
		r, _ := writeRouter(s, ownerAccess)
		if w := postReset(r, "/guild/settings/reset?guildID="+writeGuild, "valid", ""); w.Code != 503 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	})
}
