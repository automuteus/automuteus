package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type listerFunc func(context.Context, string) ([]UserGuild, error)

func (f listerFunc) ListGuilds(ctx context.Context, token string) ([]UserGuild, error) {
	return f(ctx, token)
}

func TestDiscordListGuildsPages(t *testing.T) {
	var afters []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		after := r.URL.Query().Get("after")
		afters = append(afters, after)
		n := 200
		if after != "" {
			n = 1
		}
		guilds := make([]string, n)
		for i := range guilds {
			guilds[i] = fmt.Sprintf(`{"id":"%d","name":"g%d","icon":null,"owner":false,"permissions":"8"}`, 100000000000000000+len(afters)*1000+i, i)
		}
		fmt.Fprint(w, "["+strings.Join(guilds, ",")+"]")
	}))
	defer srv.Close()
	v := &discordVerifier{client: srv.Client(), baseURL: srv.URL}
	guilds, err := v.ListGuilds(context.Background(), "token")
	if err != nil || len(guilds) != 201 {
		t.Fatalf("got %d guilds, err=%v", len(guilds), err)
	}
	if len(afters) != 2 || afters[0] != "" || afters[1] != "100000000000001199" {
		t.Fatalf("pagination cursors %v", afters)
	}
	if guilds[200].ID != "100000000000002000" || guilds[200].Icon != nil || guilds[200].Permissions != "8" {
		t.Fatalf("last guild %+v", guilds[200])
	}
}

func TestDiscordListGuildsFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       error
	}{
		{"revoked", `{}`, 401, errInvalidToken},
		{"missing scope", `{}`, 403, errDiscordForbidden},
		{"bad ID", `[{"id":"guild","permissions":"0"}]`, 200, errDiscordUnavailable},
		{"bad permissions", `[{"id":"123456789012345678","permissions":"-1"}]`, 200, errDiscordUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			v := &discordVerifier{client: srv.Client(), baseURL: srv.URL}
			if guilds, err := v.ListGuilds(context.Background(), "token"); err != tc.want || guilds != nil {
				t.Fatalf("guilds=%v err=%v", guilds, err)
			}
		})
	}
}

func TestUserGuildsTagsEachGuild(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const present, played, both, neither = "100000000000000001", "100000000000000002", "100000000000000003", "100000000000000004"
	icon := "abc"
	lister := listerFunc(func(_ context.Context, token string) ([]UserGuild, error) {
		if token != "valid" {
			t.Fatal("incorrect token")
		}
		return []UserGuild{
			{ID: present, Name: "present", Owner: true, Permissions: "0"},
			{ID: played, Name: "played", Icon: &icon, Permissions: "8"},
			{ID: both, Name: "both", Permissions: "32"},
			{ID: neither, Name: "neither", Permissions: "0"},
		}, nil
	})
	s := &fakeStore{
		botGuilds:   map[string]bool{present: true, both: true},
		statsGuilds: map[string]bool{played: true, both: true},
	}
	r := NewRouter(Config{GuildLister: lister}, s)
	w := bearerRequest(r, "/user/guilds", "valid")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
	var entries []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil || len(entries) != 4 {
		t.Fatalf("body %s", w.Body)
	}
	for i, want := range []struct {
		id         string
		bot, stats bool
	}{{present, true, false}, {played, false, true}, {both, true, true}, {neither, false, false}} {
		e := entries[i]
		if e["id"] != want.id || e["botPresent"] != want.bot || e["hasStats"] != want.stats {
			t.Fatalf("entry %d: %v", i, e)
		}
	}
	if entries[0]["owner"] != true || entries[0]["icon"] != nil || entries[1]["icon"] != "abc" || entries[1]["permissions"] != "8" {
		t.Fatalf("Discord fields not passed through: %s", w.Body)
	}
}

func TestUserGuildsRequiresUserToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &fakeStore{}
	lister := listerFunc(func(context.Context, string) ([]UserGuild, error) {
		t.Fatal("listed without a token")
		return nil, nil
	})
	r := NewRouter(Config{GuildLister: lister, AdminPassword: "test-password"}, s)
	for _, auth := range []bool{false, true} {
		if w := request(t, r, "/user/guilds", auth); w.Code != 401 || w.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("basic auth %v: %d", auth, w.Code)
		}
	}
	if s.calls != 0 {
		t.Fatalf("unauthenticated requests accessed storage %d times", s.calls)
	}
}

func TestUserGuildsFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	one := []UserGuild{{ID: "123456789012345678", Name: "g", Permissions: "0"}}
	for _, tc := range []struct {
		name      string
		listErr   error
		store     fakeStore
		status    int
		wantCalls int
	}{
		{"revoked", errInvalidToken, fakeStore{}, 401, 0},
		{"missing scope", errDiscordForbidden, fakeStore{}, 403, 0},
		{"Discord unavailable", errDiscordUnavailable, fakeStore{}, 503, 0},
		{"Redis unavailable", nil, fakeStore{botErr: errors.New("private redis detail")}, 503, 1},
		{"Postgres unavailable", nil, fakeStore{statsErr: errors.New("private postgres detail")}, 503, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &tc.store
			lister := listerFunc(func(context.Context, string) ([]UserGuild, error) {
				if tc.listErr != nil {
					return nil, tc.listErr
				}
				return one, nil
			})
			w := bearerRequest(NewRouter(Config{GuildLister: lister}, s), "/user/guilds", "token")
			if w.Code != tc.status || s.calls != tc.wantCalls || strings.Contains(w.Body.String(), "private") {
				t.Fatalf("got %d %s after %d store calls", w.Code, w.Body, s.calls)
			}
		})
	}
}

func TestUserGuildsWithoutLister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	v := verifierFunc(func(context.Context, string, string) (VerifiedGuildAccess, error) {
		return VerifiedGuildAccess{}, nil
	})
	if w := bearerRequest(NewRouter(Config{GuildVerifier: v}, &fakeStore{}), "/user/guilds", "token"); w.Code != 501 {
		t.Fatalf("got %d", w.Code)
	}
}
