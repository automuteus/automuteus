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
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type fakeStore struct {
	err    error
	calls  int
	notice *notice.Notice
}

func (s *fakeStore) ActiveNotice(context.Context) (*notice.Notice, error) {
	s.calls++
	return s.notice, s.err
}
func (s *fakeStore) RaiseNotice(_ context.Context, n notice.Notice, ttl time.Duration) error {
	s.calls++
	if s.err != nil {
		return s.err
	}
	if ttl > 0 {
		n.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	s.notice = &n
	return nil
}
func (s *fakeStore) ClearNotice(context.Context) error {
	s.calls++
	s.notice = nil
	return s.err
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
func (s *fakeStore) Settings(context.Context, string) (*settings.GuildSettings, error) {
	s.calls++
	return settings.MakeGuildSettings(), s.err
}
func (s *fakeStore) Premium(context.Context, string) (premium.PremiumRecord, error) {
	s.calls++
	return premium.PremiumRecord{Tier: premium.SelfHostTier, Days: premium.NoExpiryCode}, s.err
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
		{"/game/state?guildID=123456789012345678&connectCode=ABCDEFGH", true, `"running":true`},
		{"/game/roomcode?connectCode=ABCDEFGH", true, `"roomCode":"ABCDEF"`},
		{"/guild/settings?guildID=123456789012345678", true, `"language":"en"`},
		{"/guild/premium?guildID=123456789012345678", true, `"tier":`},
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
	for _, path := range []string{"/game/state", "/game/roomcode", "/guild/settings", "/guild/premium"} {
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
		`{"severity":"warning","message":"   "}`,
		`{"severity":"warning","message":"x","ttlSeconds":-1}`,
		`{"severity":"warning","message":"` + strings.Repeat("a", 501) + `"}`,
	} {
		if w := adminRequest(t, r, http.MethodPost, "/admin/notice", body, "test-password"); w.Code != 400 {
			t.Errorf("body %q: got %d, want 400", body, w.Code)
		}
	}
	if s.notice != nil {
		t.Fatal("invalid requests raised a notice")
	}

	w := adminRequest(t, r, http.MethodPost, "/admin/notice", `{"severity":"Warning","message":" DB maintenance ","ttlSeconds":600}`, "test-password")
	if w.Code != 200 || s.notice == nil || s.notice.Severity != notice.Warning || s.notice.Message != "DB maintenance" || s.notice.Source != "admin-api" || s.notice.ExpiresAt == 0 {
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
