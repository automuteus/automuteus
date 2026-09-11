package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type fakeStore struct {
	err   error
	calls int
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
