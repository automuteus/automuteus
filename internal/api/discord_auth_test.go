package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDiscordRateLimitRetry(t *testing.T) {
	for _, tc := range []struct {
		name, header, body      string
		secondStatus, wantCalls int
		wantErr                 error
	}{
		{"header cooldown", "0.001", `{}`, 200, 2, nil},
		{"body cooldown", "", `{"retry_after":0.001}`, 200, 2, nil},
		{"retry remains limited", "0.001", `{}`, 429, 2, errDiscordUnavailable},
		{"retry revocation", "0.001", `{}`, 401, 2, errInvalidToken},
		{"long cooldown", "60", `{}`, 200, 1, errDiscordUnavailable},
		{"invalid cooldown", "NaN", `{}`, 200, 1, errDiscordUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 {
					w.Header().Set("Retry-After", tc.header)
					w.WriteHeader(429)
					fmt.Fprint(w, tc.body)
					return
				}
				w.WriteHeader(tc.secondStatus)
				fmt.Fprint(w, `{"id":"123456789012345678"}`)
			}))
			defer srv.Close()
			v := &discordVerifier{client: srv.Client(), baseURL: srv.URL}
			var user map[string]string
			err := v.get(context.Background(), "token", "/users/@me", &user)
			if err != tc.wantErr || calls != tc.wantCalls {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestDiscordRateLimitWaitRespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(429)
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()
	v := &discordVerifier{client: srv.Client(), baseURL: srv.URL}
	// Cancel after the HTTP response has been received and the retry wait begins.
	time.AfterFunc(50*time.Millisecond, cancel)
	start := time.Now()
	var user map[string]string
	err := v.get(ctx, "token", "/users/@me", &user)
	if err != errDiscordUnavailable || time.Since(start) > time.Second {
		t.Fatal("cooldown ignored cancellation")
	}
}

func TestSettingsRequestRecoversFromGuildPickerRateLimit(t *testing.T) {
	const guild = "123456789012345678"
	guildCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/@me" {
			fmt.Fprint(w, `{"id":"223456789012345678"}`)
			return
		}
		guildCalls++
		if guildCalls == 1 {
			w.Header().Set("Retry-After", "0.001")
			w.WriteHeader(429)
			return
		}
		fmt.Fprintf(w, `[{"id":%q,"permissions":"0"}]`, guild)
	}))
	defer srv.Close()
	store := &fakeStore{}
	router := NewRouter(Config{GuildVerifier: &discordVerifier{client: srv.Client(), baseURL: srv.URL}}, store)
	response := bearerRequest(router, "/guild/settings?guildID="+guild, "token")
	if response.Code != 200 || store.calls != 1 || guildCalls != 2 {
		t.Fatalf("status=%d store calls=%d guild calls=%d", response.Code, store.calls, guildCalls)
	}
}

type verifierFunc func(context.Context, string, string) (VerifiedGuildAccess, error)

func (f verifierFunc) VerifyGuild(ctx context.Context, token, guild string) (VerifiedGuildAccess, error) {
	return f(ctx, token, guild)
}

func bearerRequest(r http.Handler, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestBearerRoutes(t *testing.T) {
	const guild = "123456789012345678"
	for _, tc := range []struct {
		name, token string
		access      VerifiedGuildAccess
		err         error
		status      int
	}{
		{"member", "valid", VerifiedGuildAccess{UserID: "user", GuildID: guild, Member: true}, nil, 200},
		{"nonmember", "valid", VerifiedGuildAccess{UserID: "user", GuildID: guild}, nil, 403},
		{"wrong guild", "valid", VerifiedGuildAccess{UserID: "user", GuildID: "other", Member: true, Owner: true}, nil, 403},
		{"expired", "expired", VerifiedGuildAccess{}, errInvalidToken, 401},
		{"missing scope", "valid", VerifiedGuildAccess{}, errDiscordForbidden, 403},
		{"Discord unavailable", "valid", VerifiedGuildAccess{}, errDiscordUnavailable, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeStore{}
			v := verifierFunc(func(_ context.Context, token, target string) (VerifiedGuildAccess, error) {
				if token != tc.token || target != guild {
					t.Fatal("incorrect verification input")
				}
				return tc.access, tc.err
			})
			r := NewRouter(Config{GuildVerifier: v, AdminPassword: "test-password"}, s)
			for _, path := range []string{"/guild/settings?guildID=" + guild, "/guild/premium?guildID=" + guild, "/guild/bot?guildID=" + guild, "/game/state?guildID=" + guild + "&connectCode=ABCDEFGH", "/game/roomcode?guildID=" + guild + "&connectCode=ABCDEFGH"} {
				w := bearerRequest(r, path, tc.token)
				if w.Code != tc.status {
					t.Fatalf("%s: %d %s", path, w.Code, w.Body)
				}
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("missing cache protection")
				}
			}
			if tc.status != 200 && s.calls != 0 {
				t.Fatal("denied request accessed storage")
			}
			before := s.calls
			if w := bearerRequest(r, "/admin/notice", tc.token); w.Code != 401 || s.calls != before {
				t.Fatal("bearer reached platform administration")
			}
		})
	}
}

func TestBearerValidationAndRevocation(t *testing.T) {
	member := true
	calls := 0
	s := &fakeStore{}
	r := NewRouter(Config{AccessCacheTTL: -1, GuildVerifier: verifierFunc(func(_ context.Context, _, guild string) (VerifiedGuildAccess, error) {
		calls++
		return VerifiedGuildAccess{UserID: "user", GuildID: guild, Member: member}, nil
	})}, s)
	for _, path := range []string{"/guild/settings", "/game/roomcode?connectCode=ABCDEFGH", "/guild/settings?guildID=invalid"} {
		if w := bearerRequest(r, path, "valid"); w.Code != 400 {
			t.Fatalf("validation: %d", w.Code)
		}
	}
	if calls != 0 || s.calls != 0 {
		t.Fatal("invalid target reached dependencies")
	}
	path := "/guild/settings?guildID=123456789012345678"
	if w := bearerRequest(r, path, "valid"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	member = false
	if w := bearerRequest(r, path, "valid"); w.Code != 403 || s.calls != 1 {
		t.Fatal("revoked member retained access")
	}
	if w := adminRequest(t, r, http.MethodGet, path, "", "automuteus"); w.Code != 401 {
		t.Fatal("default password bypassed policy")
	}
}

func TestDiscordVerifier(t *testing.T) {
	for _, tc := range []struct {
		name, user, guilds string
		status             int
		want               error
		member             bool
	}{
		{"member", `{"id":"123456789012345678"}`, `[{"id":"223456789012345678","permissions":"9007199254741000","owner":false}]`, 200, nil, true},
		{"absent", `{"id":"123456789012345678"}`, `[]`, 200, nil, false},
		{"invalid token", `{}`, `[]`, 401, errInvalidToken, false},
		{"scope", `{}`, `[]`, 403, errDiscordForbidden, false},
		{"rate limit", `{}`, `[]`, 429, errDiscordUnavailable, false},
		{"outage", `{}`, `[]`, 500, errDiscordUnavailable, false},
		{"malformed identity", `{}`, `[]`, 200, errDiscordUnavailable, false},
		{"malformed JSON", `{"id":"123456789012345678"}`, `[`, 200, errDiscordUnavailable, false},
		{"invalid permissions", `{"id":"123456789012345678"}`, `[{"id":"223456789012345678","permissions":"-1"}]`, 200, errDiscordUnavailable, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer secret" {
					t.Error("token not forwarded")
				}
				w.WriteHeader(tc.status)
				if r.URL.Path == "/users/@me" {
					fmt.Fprint(w, tc.user)
				} else {
					fmt.Fprint(w, tc.guilds)
				}
			}))
			defer srv.Close()
			v := &discordVerifier{client: srv.Client(), baseURL: srv.URL}
			a, err := v.VerifyGuild(context.Background(), "secret", "223456789012345678")
			if err != tc.want || a.Member != tc.member {
				t.Fatalf("got %+v, %v", a, err)
			}
			if tc.member && a.Permissions != 9007199254741000 {
				t.Fatal("permissions lost precision")
			}
		})
	}
}

func TestDiscordVerifierPagination(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/@me" {
			fmt.Fprint(w, `{"id":"123456789012345678"}`)
			return
		}
		calls++
		if r.URL.Query().Get("limit") != "200" {
			t.Error("missing page limit")
		}
		if calls == 1 {
			guilds := make([]map[string]string, 200)
			for i := range guilds {
				guilds[i] = map[string]string{"id": fmt.Sprint(123456789012345678 + int64(i)), "permissions": "0"}
			}
			json.NewEncoder(w).Encode(guilds)
		} else {
			if r.URL.Query().Get("after") != "123456789012345877" {
				t.Error("incorrect cursor")
			}
			fmt.Fprint(w, `[{"id":"223456789012345678","permissions":"8"}]`)
		}
	}))
	defer srv.Close()
	v := &discordVerifier{client: srv.Client(), baseURL: srv.URL}
	a, err := v.VerifyGuild(context.Background(), "token", "223456789012345678")
	if err != nil || !a.Member || calls != 2 {
		t.Fatalf("%+v %v calls=%d", a, err, calls)
	}
}

func TestMemberGameProjection(t *testing.T) {
	raw := json.RawMessage(`{"guildID":"guild","connectCode":"ABCDEFGH","running":true,"futureSecret":"secret","userData":{"private":"secret"},"amongUsData":{"room":"ROOM","futureSecret":"secret","playerData":{"a":{"name":"a","color":1,"isAlive":true,"secret":"secret"}}}}`)
	view, err := memberGameState(raw, "guild", "ABCDEFGH")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(view)
	for _, forbidden := range []string{"secret", "connectCode", "userData", "ROOM"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("leaked %s: %s", forbidden, encoded)
		}
	}
	if _, err := memberGameState(raw, "other", "ABCDEFGH"); err == nil {
		t.Fatal("cross guild snapshot accepted")
	}
	if _, err := memberGameState(raw, "guild", "XXXXXXXX"); err == nil {
		t.Fatal("wrong connect code accepted")
	}
}

type snapshotStore struct {
	fakeStore
	raw json.RawMessage
}

func (s *snapshotStore) GameState(context.Context, string, string) (json.RawMessage, error) {
	s.calls++
	return s.raw, nil
}

func TestBearerRoomCodeBindsSnapshot(t *testing.T) {
	for _, guild := range []string{"123456789012345678", "223456789012345678"} {
		s := &snapshotStore{raw: json.RawMessage(fmt.Sprintf(`{"guildID":%q,"connectCode":"ABCDEFGH","amongUsData":{"room":"SAFE"}}`, guild))}
		r := NewRouter(Config{GuildVerifier: verifierFunc(func(_ context.Context, _, target string) (VerifiedGuildAccess, error) {
			return VerifiedGuildAccess{UserID: "user", GuildID: target, Member: true}, nil
		})}, s)
		w := bearerRequest(r, "/game/roomcode?guildID=123456789012345678&connectCode=ABCDEFGH", "token")
		if guild == "123456789012345678" {
			if w.Code != 200 || w.Body.String() != `{"roomCode":"SAFE"}` {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		} else if w.Code != 404 || strings.Contains(w.Body.String(), "SAFE") {
			t.Fatal("cross-guild room leaked")
		}
		if s.calls != 1 {
			t.Fatal("room endpoint used global lookup")
		}
	}
}

func TestDiscordVerifierCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newDiscordVerifier().VerifyGuild(ctx, "token", "123456789012345678")
	if err != errDiscordUnavailable {
		t.Fatalf("cancelled verification: %v", err)
	}
}
