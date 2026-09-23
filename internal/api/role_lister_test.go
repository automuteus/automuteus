package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/gin-gonic/gin"
)

const (
	modRole     = "663456789012345678"
	helperRole  = "663456789012345679"
	botRole     = "663456789012345680"
	strangeRole = "663456789012345699"
)

func TestDiscordRoleLister(t *testing.T) {
	stub := (&discordStub{}).reply("/guilds/"+verifyGuild+"/roles", 200, `[
		{"id":"`+verifyGuild+`","name":"@everyone","color":0,"position":0},
		{"id":"`+helperRole+`","name":"Helper","color":0,"position":1},
		{"id":"`+modRole+`","name":"Mods","color":16711680,"position":3},
		{"id":"`+botRole+`","name":"AutoMuteUs","color":5793266,"position":2,"managed":true},
		{"id":"nope","name":"bad id","position":9}
	]`)
	srv := stub.server(t)
	defer srv.Close()
	v := &discordChannelVerifier{client: srv.Client(), baseURL: srv.URL, token: "bot-token"}
	roles, err := v.ListRoles(context.Background(), verifyGuild)
	if err != nil {
		t.Fatal(err)
	}
	want := []GuildRole{
		{ID: modRole, Name: "Mods", Color: 16711680, Position: 3},
		{ID: botRole, Name: "AutoMuteUs", Color: 5793266, Position: 2, Managed: true},
		{ID: helperRole, Name: "Helper", Position: 1},
	}
	if len(roles) != len(want) {
		t.Fatalf("roles = %+v", roles)
	}
	for i := range want {
		if roles[i] != want[i] {
			t.Errorf("roles[%d] = %+v, want %+v", i, roles[i], want[i])
		}
	}

	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"bot not in guild", 403, `{"message":"Missing Access"}`, errChannelNotFound},
		{"unknown guild", 404, `{}`, errChannelNotFound},
		{"outage", 500, ``, errChannelUnavailable},
		{"malformed", 200, `{"not":"a list"}`, errChannelUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := (&discordStub{}).reply("/guilds/"+verifyGuild+"/roles", tc.status, tc.body).server(t)
			defer srv.Close()
			v := &discordChannelVerifier{client: srv.Client(), baseURL: srv.URL, token: "bot-token"}
			if _, err := v.ListRoles(context.Background(), verifyGuild); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := v.ListRoles(context.Background(), "general"); !errors.Is(err, errChannelNotFound) {
		t.Errorf("bad guild ID should be refused locally: %v", err)
	}
}

// roleListerFunc adapts a function to RoleLister for tests.
type roleListerFunc func(ctx context.Context, guildID string) ([]GuildRole, error)

func (f roleListerFunc) ListRoles(ctx context.Context, guildID string) ([]GuildRole, error) {
	return f(ctx, guildID)
}

func fakeRoles(calls *int) roleListerFunc {
	return func(_ context.Context, guildID string) ([]GuildRole, error) {
		if calls != nil {
			*calls++
		}
		if guildID != writeGuild {
			return nil, errChannelNotFound
		}
		return []GuildRole{{ID: modRole, Name: "Mods", Color: 16711680, Position: 2}, {ID: helperRole, Name: "Helper", Position: 1}}, nil
	}
}

func writeRouterRoles(s *fakeStore, access VerifiedGuildAccess, roles RoleLister) http.Handler {
	v := verifierFunc(func(_ context.Context, token, guild string) (VerifiedGuildAccess, error) {
		if token != "valid" {
			return VerifiedGuildAccess{}, errInvalidToken
		}
		if guild != access.GuildID {
			return VerifiedGuildAccess{UserID: access.UserID, GuildID: guild}, nil
		}
		return access, nil
	})
	return NewRouter(Config{GuildVerifier: v, AdminPassword: "test-password", RoleLister: roles}, s)
}

func TestGuildRolesRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := writeRouterRoles(&fakeStore{}, memberAccess, fakeRoles(nil))
	w := bearerRequest(r, "/guild/roles?guildID="+writeGuild, "valid")
	var roles []GuildRole
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &roles) != nil || len(roles) != 2 || roles[0].Name != "Mods" {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("missing cache protection")
	}
	// Repeated requests for the same guild are served from the list cache.
	calls := 0
	cached := writeRouterRoles(&fakeStore{}, memberAccess, fakeRoles(&calls))
	for i := 0; i < 3; i++ {
		if w := bearerRequest(cached, "/guild/roles?guildID="+writeGuild, "valid"); w.Code != 200 {
			t.Fatalf("cached request %d: %d", i, w.Code)
		}
	}
	if calls != 1 {
		t.Errorf("roles fetched %d times for three requests, want 1", calls)
	}
	if w := bearerRequest(r, "/guild/roles?guildID=general", "valid"); w.Code != 400 {
		t.Errorf("bad guild: %d", w.Code)
	}
	if w := bearerRequest(r, "/guild/roles?guildID="+writeGuild, "bogus"); w.Code != 401 {
		t.Errorf("bad token: %d", w.Code)
	}
	departed := VerifiedGuildAccess{UserID: "999", GuildID: writeGuild}
	if w := bearerRequest(writeRouterRoles(&fakeStore{}, departed, fakeRoles(nil)), "/guild/roles?guildID="+writeGuild, "valid"); w.Code != 403 {
		t.Errorf("non-member: %d", w.Code)
	}
	if w := bearerRequest(writeRouterRoles(&fakeStore{}, memberAccess, nil), "/guild/roles?guildID="+writeGuild, "valid"); w.Code != http.StatusNotImplemented {
		t.Errorf("no bot token: %d", w.Code)
	}
	gone := roleListerFunc(func(context.Context, string) ([]GuildRole, error) { return nil, errChannelNotFound })
	if w := bearerRequest(writeRouterRoles(&fakeStore{}, memberAccess, gone), "/guild/roles?guildID="+writeGuild, "valid"); w.Code != 404 {
		t.Errorf("bot not in guild: %d", w.Code)
	}
	down := roleListerFunc(func(context.Context, string) ([]GuildRole, error) { return nil, errChannelUnavailable })
	if w := bearerRequest(writeRouterRoles(&fakeStore{}, memberAccess, down), "/guild/roles?guildID="+writeGuild, "valid"); w.Code != 503 {
		t.Errorf("outage: %d", w.Code)
	}
}

func TestUpdateSettings_OperatorRolesMustExistInGuild(t *testing.T) {
	gin.SetMode(gin.TestMode)
	calls := 0
	s := &fakeStore{}
	r := writeRouterRoles(s, ownerAccess, fakeRoles(&calls))

	w := patchSettings(r, "valid", `{"permissionRoleIDs":["`+modRole+`","`+helperRole+`"]}`)
	if w.Code != 200 || s.saved == nil || len(s.saved.PermissionRoleIDs) != 2 {
		t.Fatalf("known roles: %d %s", w.Code, w.Body)
	}
	s.saved = nil
	w = patchSettings(r, "valid", `{"permissionRoleIDs":["`+modRole+`","`+strangeRole+`"]}`)
	if w.Code != 400 || s.saved != nil {
		t.Fatalf("unknown role: %d %s", w.Code, w.Body)
	}
	if got := validationFields(t, w); len(got) != 1 || got[0] != "permissionRoleIDs[1]" {
		t.Errorf("fields = %v, want [permissionRoleIDs[1]]", got)
	}
	if !strings.Contains(w.Body.String(), "not a role in this server") {
		t.Errorf("message should say why: %s", w.Body)
	}

	// Clearing the list, or resubmitting the stored one, needs no lookup; an unrelated write does not either.
	stored := settings.MakeGuildSettings()
	stored.SetPermissionRoleIDs([]string{strangeRole})
	s = &fakeStore{stored: stored}
	calls = 0
	r = writeRouterRoles(s, ownerAccess, fakeRoles(&calls))
	// Order matters: the fake store hands back the same document, so the clearing write goes last.
	for _, body := range []string{`{"permissionRoleIDs":["` + strangeRole + `"],"language":"de"}`, `{"autoRefresh":true}`, `{"permissionRoleIDs":[]}`} {
		if w := patchSettings(r, "valid", body); w.Code != 200 {
			t.Fatalf("%s: %d %s", body, w.Code, w.Body)
		}
	}
	if calls != 0 {
		t.Errorf("role lister called %d times, want 0", calls)
	}

	// Without a bot token the IDs are accepted as typed, as before.
	s = &fakeStore{}
	r = writeRouterRoles(s, ownerAccess, nil)
	if w := patchSettings(r, "valid", `{"permissionRoleIDs":["`+strangeRole+`"]}`); w.Code != 200 || s.saved == nil {
		t.Fatalf("no lister: %d %s", w.Code, w.Body)
	}

	// A lookup outage refuses the write rather than trusting the client.
	s = &fakeStore{}
	r = writeRouterRoles(s, ownerAccess, roleListerFunc(func(context.Context, string) ([]GuildRole, error) { return nil, errChannelUnavailable }))
	if w := patchSettings(r, "valid", `{"permissionRoleIDs":["`+modRole+`"]}`); w.Code != 503 || s.saved != nil {
		t.Fatalf("outage: %d %s", w.Code, w.Body)
	}
}
