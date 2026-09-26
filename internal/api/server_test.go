package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type fakeStore struct {
	err    error
	calls  int
	notice *notice.Notice

	// savedGuild and saved record the last SetSettings call. saveErr fails only SetSettings; stored, when set,
	// is what Settings returns instead of fresh defaults; writeRetryAfter makes ReserveSettingsWrite refuse.
	savedGuild      string
	saved           *settings.GuildSettings
	saveErr         error
	stored          *settings.GuildSettings
	writeRetryAfter time.Duration
	// version is the row version Settings reports; SetSettings requires it and bumps it.
	version storage.SettingsVersion
	// premium, when set, is what Premium returns instead of self-host; premiumErr fails only Premium.
	premium *premium.PremiumRecord
	// subscription, when set, is what Subscription returns; subscriptionErr fails only Subscription.
	subscription    *SubscriptionStatus
	subscriptionErr error
	premiumErr      error
	premiumCalls    int
	// botAbsent makes BotInGuild report false; botErr fails only BotInGuild.
	botAbsent bool
	botErr    error
	// stats, when set, is what GuildStats returns; statsErr fails only GuildStats.
	stats      *GuildStats
	statsErr   error
	statsCalls int
	// adminStatsCalls counts AdminGuildStats, which answers like GuildStats.
	adminStatsCalls int
	// match, when set, is what MatchSummary returns; matchErr fails only MatchSummary.
	match      *MatchSummary
	matchErr   error
	matchCalls int
	// user, when set, is what UserStats returns; userErr fails only UserStats.
	user      *UserStats
	userErr   error
	userCalls int
	// announced records every AnnounceStatsChanged call; announceErr fails them. changes, when set, is what
	// StatsChanges returns, so a test can feed the router announcements.
	announced   [][]string
	announceErr error
	changes     chan notice.StatsChanged
	// botGuilds and statsGuilds are the guilds BotInGuilds and GuildsWithStats answer true for.
	botGuilds   map[string]bool
	statsGuilds map[string]bool
	// resets records each stats reset as "guild" or "guild/user"; resetErr fails them after recording.
	resets   []string
	resetErr error
}

func (s *fakeStore) ActiveNotice(context.Context) (*notice.Notice, error) {
	s.calls++
	return s.notice, s.err
}
func (s *fakeStore) RaiseNotice(_ context.Context, n notice.Notice) error {
	s.calls++
	if s.err != nil {
		return s.err
	}
	s.notice = &n
	return nil
}
func (s *fakeStore) ClearNotice(context.Context) error {
	s.calls++
	s.notice = nil
	return s.err
}

func (s *fakeStore) AnnounceStatsChanged(_ context.Context, guildIDs ...string) error {
	s.announced = append(s.announced, append([]string(nil), guildIDs...))
	return s.announceErr
}

func (s *fakeStore) StatsChanges(context.Context) <-chan notice.StatsChanged {
	return s.changes
}

func (s *fakeStore) Info(context.Context) (Info, error) {
	s.calls++
	return Info{Version: "test", TotalGuilds: 5}, s.err
}
func (s *fakeStore) GameState(context.Context, string, string) (json.RawMessage, error) {
	s.calls++
	return json.RawMessage(`{"guildID":"123456789012345678","connectCode":"ABCDEFGH","running":true}`), s.err
}
func (s *fakeStore) RoomCode(context.Context, string) (string, error) {
	s.calls++
	return "ABCDEF", s.err
}
func (s *fakeStore) Settings(context.Context, string) (*settings.GuildSettings, storage.SettingsVersion, error) {
	s.calls++
	if s.stored != nil {
		return s.stored, s.version, s.err
	}
	return settings.MakeGuildSettings(), s.version, s.err
}
func (s *fakeStore) SetSettings(_ context.Context, guildID string, sett *settings.GuildSettings, expected storage.SettingsVersion) error {
	s.calls++
	if s.err != nil {
		return s.err
	}
	if s.saveErr != nil {
		return s.saveErr
	}
	if expected != s.version {
		return storage.ErrSettingsConflict
	}
	s.version++
	s.savedGuild, s.saved = guildID, sett
	return nil
}
func (s *fakeStore) ReserveSettingsWrite(context.Context, string) (time.Duration, error) {
	s.calls++
	return s.writeRetryAfter, s.err
}
func (s *fakeStore) Subscription(context.Context, string) (*SubscriptionStatus, error) {
	return s.subscription, s.subscriptionErr
}
func (s *fakeStore) Premium(context.Context, string) (premium.PremiumRecord, error) {
	s.calls++
	s.premiumCalls++
	if s.premiumErr != nil {
		return premium.PremiumRecord{}, s.premiumErr
	}
	if s.premium != nil {
		return *s.premium, s.err
	}
	return premium.PremiumRecord{Tier: premium.SelfHostTier, Days: premium.NoExpiryCode}, s.err
}
func (s *fakeStore) AdminGuildStats(ctx context.Context, guildID string) (GuildStats, error) {
	s.adminStatsCalls++
	return s.GuildStats(ctx, guildID)
}
func (s *fakeStore) GuildStats(_ context.Context, guildID string) (GuildStats, error) {
	s.calls++
	s.statsCalls++
	if s.statsErr != nil {
		return GuildStats{}, s.statsErr
	}
	if s.stats != nil {
		return *s.stats, s.err
	}
	return GuildStats{GuildID: guildID, Summary: GuildStatsSummary{GamesPlayed: 7}, Players: map[string]StatsPlayer{}}, s.err
}
func (s *fakeStore) MatchSummary(_ context.Context, guildID, matchID string) (MatchSummary, error) {
	s.calls++
	s.matchCalls++
	if s.matchErr != nil {
		return MatchSummary{}, s.matchErr
	}
	if s.match != nil {
		return *s.match, s.err
	}
	return MatchSummary{GuildID: guildID, MatchID: matchID, Status: "finished", Roster: []MatchPlayer{}, Players: map[string]StatsPlayer{}}, s.err
}
func (s *fakeStore) UserStats(_ context.Context, guildID, userID string) (UserStats, error) {
	s.calls++
	s.userCalls++
	if s.userErr != nil {
		return UserStats{}, s.userErr
	}
	if s.user != nil {
		return *s.user, s.err
	}
	return UserStats{GuildID: guildID, UserID: userID, RecentMatches: []UserMatch{}, Players: map[string]StatsPlayer{}}, s.err
}
func (s *fakeStore) ResetGuildStats(_ context.Context, guildID string) (int64, error) {
	s.calls++
	s.resets = append(s.resets, guildID)
	return 5, s.resetErr
}
func (s *fakeStore) ResetUserStats(_ context.Context, guildID, userID string) (int64, error) {
	s.calls++
	s.resets = append(s.resets, guildID+"/"+userID)
	return 2, s.resetErr
}
func (s *fakeStore) BotInGuild(context.Context, string) (bool, error) {
	s.calls++
	if s.botErr != nil {
		return false, s.botErr
	}
	return !s.botAbsent, s.err
}
func (s *fakeStore) BotInGuilds(_ context.Context, guildIDs []string) ([]bool, error) {
	s.calls++
	if s.botErr != nil {
		return nil, s.botErr
	}
	present := make([]bool, len(guildIDs))
	for i, id := range guildIDs {
		present[i] = s.botGuilds[id]
	}
	return present, s.err
}
func (s *fakeStore) GuildsWithStats(_ context.Context, guildIDs []string) ([]bool, error) {
	s.calls++
	if s.statsErr != nil {
		return nil, s.statsErr
	}
	has := make([]bool, len(guildIDs))
	for i, id := range guildIDs {
		has[i] = s.statsGuilds[id]
	}
	return has, s.err
}
func (s *fakeStore) Ping(context.Context) error { s.calls++; return s.err }

func request(t *testing.T, router http.Handler, path string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if auth {
		r.SetBasicAuth("admin", "test-password")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func TestRoutesWithoutDiscordSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &fakeStore{}
	r := NewRouter(Config{Version: "test", AdminPassword: "test-password", CaptureHost: "https://galactus.example.com"}, s)
	for _, tc := range []struct {
		path string
		auth bool
		want string
	}{
		{"/bot/info", false, `"totalGuilds":5`},
		{"/bot/commands", false, `"name":"new"`},
		{"/bot/settings/defaults", false, `"language":"en"`},
		{"/game/state?guildID=123456789012345678&connectCode=ABCDEFGH", true, `"running":true`},
		{"/game/roomcode?connectCode=ABCDEFGH", true, `"roomCode":"ABCDEF"`},
		{"/guild/settings?guildID=123456789012345678", true, `"language":"en"`},
		{"/guild/premium?guildID=123456789012345678", true, `"tier":`},
		{"/guild/bot?guildID=123456789012345678", true, `{"present":true}`},
		{"/open/link?connectCode=ABCDEFGH", false, `aucapture:\/\/galactus.example.com:443\/ABCDEFGH`},
		{"/live", false, ""},
		{"/ready", false, ""},
		{"/swagger/doc.json", false, `"version": "test"`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := request(t, r, tc.path, tc.auth)
			if w.Code != 200 || !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("got %d %s", w.Code, w.Body)
			}
		})
	}
}

func TestAuthenticationAndValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &fakeStore{}
	r := NewRouter(Config{AdminPassword: "test-password"}, s)
	for _, path := range []string{"/game/state", "/game/roomcode", "/guild/settings", "/guild/premium", "/guild/bot", "/guild/channel", "/guild/roles"} {
		if w := request(t, r, path, false); w.Code != 401 {
			t.Fatalf("%s: %d", path, w.Code)
		}
		if w := request(t, r, path, true); w.Code != 400 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	for _, code := range []string{"", "short", "waytoolongcode"} {
		w := request(t, r, "/game/roomcode?connectCode="+code, true)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "invalid connect code") {
			t.Fatalf("got %d %s", w.Code, w.Body)
		}
	}
	if s.calls != 0 {
		t.Fatalf("invalid requests accessed storage %d times", s.calls)
	}
}

func TestDependencyFailuresAndHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &fakeStore{err: errors.New("private connection details")}
	r := NewRouter(Config{AdminPassword: "test-password"}, s)
	for _, path := range []string{"/ready", "/guild/settings?guildID=123456789012345678"} {
		w := request(t, r, path, true)
		if w.Code != 503 || strings.Contains(w.Body.String(), s.err.Error()) {
			t.Fatalf("got %d %s", w.Code, w.Body)
		}
	}
	if w := request(t, r, "/live", false); w.Code != 200 {
		t.Fatal("dependency failure made liveness fail")
	}
	s.err = redis.Nil
	if w := request(t, r, "/game/roomcode?connectCode=ABCDEFGH", true); w.Code != 404 {
		t.Fatalf("missing room code: %d", w.Code)
	}
}

func adminRequest(t *testing.T, r http.Handler, method, path, body string, password string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if password != "" {
		req.SetBasicAuth("admin", password)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestNoticeEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &fakeStore{}
	r := NewRouter(Config{AdminPassword: "test-password"}, s)

	if w := adminRequest(t, r, http.MethodPost, "/admin/notice", `{"severity":"warning","message":"x"}`, ""); w.Code != 401 {
		t.Fatalf("unauthenticated post: %d", w.Code)
	}
	if w := adminRequest(t, r, http.MethodGet, "/admin/notice", "", "test-password"); w.Code != 404 {
		t.Fatalf("get with no notice: %d %s", w.Code, w.Body)
	}
	for _, body := range []string{
		`not json`,
		`{"severity":"loud","message":"x"}`,
		`{"severity":"info","message":"x"}`,
		`{"severity":"warning","message":"   "}`,
		`{"severity":"warning","message":"` + strings.Repeat("a", 501) + `"}`,
	} {
		if w := adminRequest(t, r, http.MethodPost, "/admin/notice", body, "test-password"); w.Code != 400 {
			t.Errorf("body %q: got %d, want 400", body, w.Code)
		}
	}
	if s.notice != nil {
		t.Fatal("invalid requests raised a notice")
	}

	w := adminRequest(t, r, http.MethodPost, "/admin/notice", `{"severity":"Warning","message":" DB maintenance "}`, "test-password")
	if w.Code != 200 || s.notice == nil || s.notice.Severity != notice.Warning || s.notice.Message != "DB maintenance" {
		t.Fatalf("raise: %d %s; stored %+v", w.Code, w.Body, s.notice)
	}
	if w := adminRequest(t, r, http.MethodGet, "/admin/notice", "", "test-password"); w.Code != 200 || !strings.Contains(w.Body.String(), `"severity":"warning"`) {
		t.Fatalf("get: %d %s", w.Code, w.Body)
	}
	if w := adminRequest(t, r, http.MethodDelete, "/admin/notice", "", "test-password"); w.Code != 204 || s.notice != nil {
		t.Fatalf("clear: %d; stored %+v", w.Code, s.notice)
	}

	s.err = errors.New("redis down")
	if w := adminRequest(t, r, http.MethodPost, "/admin/notice", `{"severity":"critical","message":"x"}`, "test-password"); w.Code != 503 || strings.Contains(w.Body.String(), "redis down") {
		t.Fatalf("dependency failure: %d %s", w.Code, w.Body)
	}
}

func TestNoticeEndpointsRefuseDefaultPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &fakeStore{}
	r := NewRouter(Config{}, s) // no admin password configured: the default "automuteus" is in effect
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		w := adminRequest(t, r, method, "/admin/notice", `{"severity":"critical","message":"x"}`, "automuteus")
		if w.Code != 403 {
			t.Errorf("%s under default password: got %d, want 403", method, w.Code)
		}
	}
	if s.calls != 0 || s.notice != nil {
		t.Fatal("default password reached the store")
	}
	// reading is still allowed
	if w := adminRequest(t, r, http.MethodGet, "/admin/notice", "", "automuteus"); w.Code != 404 {
		t.Errorf("get under default password: %d", w.Code)
	}
}

func TestNoticeEndpointsRefuseExplicitDefaultPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &fakeStore{}
	r := NewRouter(Config{AdminPassword: "automuteus"}, s)
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		w := adminRequest(t, r, method, "/admin/notice", `{"severity":"critical","message":"x"}`, "automuteus")
		if w.Code != http.StatusForbidden {
			t.Errorf("%s with explicitly configured default password: got %d, want 403", method, w.Code)
		}
	}
	if s.calls != 0 {
		t.Errorf("default password reached the store %d times", s.calls)
	}
}

func TestGuildBotPresence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name  string
		store *fakeStore
		code  int
		want  string
	}{
		{"present", &fakeStore{}, 200, `{"present":true}`},
		{"absent", &fakeStore{botAbsent: true}, 200, `{"present":false}`},
		{"redis down", &fakeStore{botErr: errors.New("private connection details")}, 503, "Unable to check bot membership"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRouter(Config{AdminPassword: "test-password"}, tc.store)
			w := request(t, r, "/guild/bot?guildID=123456789012345678", true)
			if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("got %d %s", w.Code, w.Body)
			}
			if strings.Contains(w.Body.String(), "private connection details") {
				t.Fatal("leaked dependency error")
			}
		})
	}
}

func TestSettingsDefaultsMatchFreshGuild(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Config{AdminPassword: "test-password"}, &fakeStore{})
	w := request(t, r, "/bot/settings/defaults", false)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("got %d %q", w.Code, w.Header().Get("Cache-Control"))
	}
	fresh, err := json.Marshal(settings.MakeGuildSettings())
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(w.Body.String()) != string(fresh) {
		t.Fatalf("defaults drifted from MakeGuildSettings:\n%s\n%s", w.Body.String(), fresh)
	}
	// The fake store returns MakeGuildSettings for a guild with no row, so the two routes must agree.
	if g := request(t, r, "/guild/settings?guildID=123456789012345678", true); strings.TrimSpace(g.Body.String()) != string(fresh) {
		t.Fatalf("guild without settings differs from defaults:\n%s", g.Body.String())
	}
}
