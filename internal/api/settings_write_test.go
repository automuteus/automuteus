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

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
)

const writeGuild = "123456789012345678"

var errFakeStore = errors.New("fake store failure")

var (
	ownerAccess = VerifiedGuildAccess{UserID: "999", GuildID: writeGuild, Member: true, Owner: true}
	adminAccess = VerifiedGuildAccess{UserID: "999", GuildID: writeGuild, Member: true, Permissions: discordgo.PermissionAdministrator}
	// memberAccess is a moderator with every management bit except the two that unlock settings.
	memberAccess = VerifiedGuildAccess{UserID: "999", GuildID: writeGuild, Member: true, Permissions: discordgo.PermissionManageChannels | discordgo.PermissionManageRoles | discordgo.PermissionKickMembers | discordgo.PermissionBanMembers}
)

// writeRouter builds a router whose verifier returns access for the write guild and counts how often it is asked.
func writeRouter(s *fakeStore, access VerifiedGuildAccess) (http.Handler, *int) {
	return writeRouterWith(s, access, nil)
}

// channelVerifierFunc adapts a function to ChannelVerifier for tests.
type channelVerifierFunc func(ctx context.Context, channelID string) (ChannelInfo, error)

func (f channelVerifierFunc) VerifyChannel(ctx context.Context, channelID string) (ChannelInfo, error) {
	return f(ctx, channelID)
}

func writeRouterWith(s *fakeStore, access VerifiedGuildAccess, channels ChannelVerifier) (http.Handler, *int) {
	calls := 0
	v := verifierFunc(func(_ context.Context, token, guild string) (VerifiedGuildAccess, error) {
		calls++
		if token != "valid" {
			return VerifiedGuildAccess{}, errInvalidToken
		}
		if guild != access.GuildID {
			return VerifiedGuildAccess{UserID: access.UserID, GuildID: guild}, nil
		}
		return access, nil
	})
	return NewRouter(Config{GuildVerifier: v, AdminPassword: "test-password", ChannelVerifier: channels}, s), &calls
}

func patchSettingsIfMatch(r http.Handler, token, ifMatch, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/guild/settings?guildID="+writeGuild, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("If-Match", ifMatch)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func patchSettings(r http.Handler, token, body string) *httptest.ResponseRecorder {
	return patchSettingsFor(r, writeGuild, token, body)
}

func patchSettingsFor(r http.Handler, guild, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/guild/settings?guildID="+guild, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeSettings(t *testing.T, w *httptest.ResponseRecorder) *settings.GuildSettings {
	t.Helper()
	var sett settings.GuildSettings
	if err := json.Unmarshal(w.Body.Bytes(), &sett); err != nil {
		t.Fatalf("response is not a settings document: %v\n%s", err, w.Body)
	}
	return &sett
}

func validationFields(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	var resp SettingsValidationError
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not a validation error: %v\n%s", err, w.Body)
	}
	fields := make([]string, len(resp.Fields))
	for i, f := range resp.Fields {
		fields[i] = f.Field
	}
	return fields
}

func TestUpdateSettings_OwnerWritesMergedDocument(t *testing.T) {
	s := &fakeStore{}
	r, verifierCalls := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"language":"de","leaderboardSize":7}`)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if *verifierCalls != 1 {
		t.Errorf("verifier called %d times, want 1", *verifierCalls)
	}
	if s.saved == nil || s.savedGuild != writeGuild {
		t.Fatalf("nothing saved for the guild: %+v", s)
	}
	if s.saved.Language != "de" || s.saved.LeaderboardSize != 7 {
		t.Errorf("sent fields not applied: language=%q size=%d", s.saved.Language, s.saved.LeaderboardSize)
	}
	if s.saved.LeaderboardMin != settings.DefaultLeaderboardMin || s.saved.MapVersion != settings.MapVersionSimple {
		t.Error("fields that were not sent should keep the stored values")
	}
	resp := decodeSettings(t, w)
	if resp.Language != "de" || resp.LeaderboardSize != 7 {
		t.Errorf("response should be the stored document, got language=%q size=%d", resp.Language, resp.LeaderboardSize)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("missing cache protection")
	}
}

func TestUpdateSettings_AdministratorOrManagerMayWrite(t *testing.T) {
	manager := memberAccess
	manager.Permissions = discordgo.PermissionManageServer
	for name, access := range map[string]VerifiedGuildAccess{"administrator": adminAccess, "manage server": manager} {
		s := &fakeStore{}
		r, _ := writeRouter(s, access)
		if w := patchSettings(r, "valid", `{"autoRefresh":true}`); w.Code != 200 {
			t.Fatalf("%s: status %d: %s", name, w.Code, w.Body)
		}
		if s.saved == nil || !s.saved.AutoRefresh {
			t.Fatalf("%s: change was not saved", name)
		}
	}
}

// Everyone who can read settings but holds neither ownership nor a settings permission is refused before storage
// is touched.
func TestUpdateSettings_DeniedCallersNeverReachStorage(t *testing.T) {
	cases := []struct {
		name   string
		access VerifiedGuildAccess
		token  string
		status int
	}{
		{"moderator without a settings permission", memberAccess, "valid", 403},
		{"plain member", VerifiedGuildAccess{UserID: "999", GuildID: writeGuild, Member: true}, "valid", 403},
		{"nonmember owner flag", VerifiedGuildAccess{UserID: "999", GuildID: writeGuild, Owner: true}, "valid", 403},
		{"owner of another guild", VerifiedGuildAccess{UserID: "999", GuildID: "223456789012345678", Member: true, Owner: true}, "valid", 403},
		{"invalid token", ownerAccess, "expired", 401},
		{"no token", ownerAccess, "", 401},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeStore{}
			r, _ := writeRouter(s, tc.access)
			w := patchSettings(r, tc.token, `{"autoRefresh":true}`)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body)
			}
			if s.calls != 0 || s.saved != nil {
				t.Fatal("denied request reached storage")
			}
		})
	}
}

// A member may still read; the write policy is separate from the read policy on the same path.
func TestUpdateSettings_ReadStillWorksForMembers(t *testing.T) {
	s := &fakeStore{}
	r, _ := writeRouter(s, memberAccess)
	if w := bearerRequest(r, "/guild/settings?guildID="+writeGuild, "valid"); w.Code != 200 {
		t.Fatalf("GET status %d: %s", w.Code, w.Body)
	}
	if w := patchSettings(r, "valid", `{}`); w.Code != 403 {
		t.Fatalf("PATCH status %d, want 403", w.Code)
	}
}

func TestUpdateSettings_MissingOrInvalidGuildIDBeforeVerification(t *testing.T) {
	s := &fakeStore{}
	r, verifierCalls := writeRouter(s, ownerAccess)
	for _, guild := range []string{"", "abc", "123"} {
		w := patchSettingsFor(r, guild, "valid", `{"autoRefresh":true}`)
		if w.Code != 400 {
			t.Errorf("guild %q: status %d, want 400", guild, w.Code)
		}
	}
	if *verifierCalls != 0 || s.calls != 0 {
		t.Fatal("invalid guild ID reached Discord or storage")
	}
}

// Only PATCH is registered: the merge semantics do not match PUT's replace-everything contract, and POST is not a
// settings write.
func TestUpdateSettings_OtherMethodsAreNotWrites(t *testing.T) {
	s := &fakeStore{}
	r, _ := writeRouter(s, ownerAccess)
	for _, method := range []string{http.MethodPut, http.MethodPost, http.MethodDelete} {
		req := httptest.NewRequest(method, "/guild/settings?guildID="+writeGuild, strings.NewReader(`{"autoRefresh":true}`))
		req.Header.Set("Authorization", "Bearer valid")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == 200 || s.saved != nil {
			t.Fatalf("%s was accepted as a write: %d", method, w.Code)
		}
	}
}

func TestUpdateSettings_PlatformAdminMayWrite(t *testing.T) {
	s := &fakeStore{}
	r, verifierCalls := writeRouter(s, ownerAccess)
	if w := adminRequest(t, r, http.MethodPatch, "/guild/settings?guildID="+writeGuild, `{"muteSpectator":true}`, "test-password"); w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if s.saved == nil || !s.saved.MuteSpectator {
		t.Fatal("platform admin's change was not saved")
	}
	if *verifierCalls != 0 {
		t.Error("Basic Auth should not consult Discord")
	}
	s = &fakeStore{}
	r, _ = writeRouter(s, ownerAccess)
	if w := adminRequest(t, r, http.MethodPatch, "/guild/settings?guildID="+writeGuild, `{"muteSpectator":true}`, "automuteus"); w.Code != 401 || s.saved != nil {
		t.Fatalf("default password wrote settings: %d", w.Code)
	}
}

// Bodies that fail to decode are rejected with 400 and a plain HttpError; nothing is saved.
func TestUpdateSettings_MalformedBodiesAreRejected(t *testing.T) {
	cases := map[string]string{
		"empty":                "",
		"whitespace":           "   ",
		"null":                 "null",
		"array":                "[]",
		"truncated":            `{"autoRefresh":true`,
		"trailing garbage":     `{"autoRefresh":true} x`,
		"unknown field":        `{"autoRefres":true}`,
		"case-insensitive key": `{"AutoRefresh":true}`,
		"wrong type":           `{"leaderboardSize":"7"}`,
		"null value":           `{"language":null}`,
		"null in array":        `{"adminIDs":[null]}`,
		"nested unknown":       `{"voiceRules":{"muteRules":{}}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s := &fakeStore{}
			r, _ := writeRouter(s, ownerAccess)
			w := patchSettings(r, "valid", body)
			if w.Code != 400 {
				t.Fatalf("status %d, want 400: %s", w.Code, w.Body)
			}
			if !strings.Contains(w.Body.String(), "invalid settings document") {
				t.Errorf("unexpected error body: %s", w.Body)
			}
			if s.saved != nil {
				t.Fatal("malformed body was saved")
			}
		})
	}
}

// Bodies that decode but hold invalid values get every offending field back; nothing is saved.
func TestUpdateSettings_InvalidValuesListEveryField(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		fields []string
	}{
		{"one field", `{"leaderboardSize":11}`, []string{"leaderboardSize"}},
		{"unknown language", `{"language":"tlh"}`, []string{"language"}},
		{"bad admin id", `{"adminIDs":["123456789012345678","nope"]}`, []string{"adminIDs[1]"}},
		{"several fields", `{"leaderboardSize":0,"leaderboardMin":101,"displayRoomCode":"maybe"}`,
			[]string{"leaderboardSize", "leaderboardMin", "displayRoomCode"}},
		{"partial delay row", `{"delays":{"delays":{"LOBBY":{"TASKS":3}}}}`,
			[]string{"delays.delays.LOBBY.LOBBY", "delays.delays.LOBBY.DISCUSSION"}},
		{"unknown phase", `{"voiceRules":{"MuteRules":{"GAMEOVER":{"alive":true,"dead":true}}}}`,
			[]string{"voiceRules.MuteRules.GAMEOVER"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeStore{}
			r, _ := writeRouter(s, ownerAccess)
			w := patchSettings(r, "valid", tc.body)
			if w.Code != 400 {
				t.Fatalf("status %d, want 400: %s", w.Code, w.Body)
			}
			got := validationFields(t, w)
			if len(got) != len(tc.fields) {
				t.Fatalf("fields %v, want %v", got, tc.fields)
			}
			for _, want := range tc.fields {
				found := false
				for _, f := range got {
					found = found || f == want
				}
				if !found {
					t.Errorf("field %q missing from %v", want, got)
				}
			}
			if s.saved != nil {
				t.Fatal("invalid document was saved")
			}
		})
	}
}

func TestUpdateSettings_FullRowReplacesOnlyThatRow(t *testing.T) {
	s := &fakeStore{}
	r, _ := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"delays":{"delays":{"LOBBY":{"LOBBY":0,"TASKS":2,"DISCUSSION":0}}}}`)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if s.saved.Delays.Delays["LOBBY"]["TASKS"] != 2 || s.saved.Delays.Delays["DISCUSSION"]["LOBBY"] != 6 {
		t.Errorf("row replacement wrong: %v", s.saved.Delays.Delays)
	}
}

// A guild whose stored row predates several fields still accepts a one-field change: gaps are filled with the
// defaults before the body is applied.
func TestUpdateSettings_LegacyStoredSettingsAcceptPartialUpdate(t *testing.T) {
	var legacy settings.GuildSettings
	if err := json.Unmarshal([]byte(`{"language":"en","adminIDs":[]}`), &legacy); err != nil {
		t.Fatal(err)
	}
	s := &fakeStore{stored: &legacy}
	r, _ := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"muteSpectator":true}`)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if !s.saved.MuteSpectator || s.saved.LeaderboardSize != settings.DefaultLeaderboardSize || s.saved.Delays.Delays == nil {
		t.Errorf("legacy gaps not filled: %+v", s.saved)
	}
}

func TestUpdateSettings_EmptyObjectIsANoOpWrite(t *testing.T) {
	s := &fakeStore{}
	r, _ := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{}`)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	want, _ := json.Marshal(settings.MakeGuildSettings())
	got, _ := json.Marshal(s.saved)
	if string(want) != string(got) {
		t.Errorf("{} changed the document:\nwant %s\n got %s", want, got)
	}
}

func TestUpdateSettings_OversizedBody(t *testing.T) {
	s := &fakeStore{}
	r, _ := writeRouter(s, ownerAccess)
	body := `{"matchSummaryChannelID":"` + strings.Repeat("1", maxSettingsBody+1) + `"}`
	w := patchSettings(r, "valid", body)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413: %.200s", w.Code, w.Body)
	}
	if s.saved != nil {
		t.Fatal("oversized body was saved")
	}
}

// The write budget is checked after authentication (so anonymous callers cannot drain a guild's budget) and before
// the body is read or settings are loaded.
func TestUpdateSettings_RateLimited(t *testing.T) {
	s := &fakeStore{writeRetryAfter: 42 * time.Second}
	r, verifierCalls := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"autoRefresh":true}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429: %s", w.Code, w.Body)
	}
	if w.Header().Get("Retry-After") != "42" {
		t.Errorf("Retry-After = %q, want 42", w.Header().Get("Retry-After"))
	}
	if *verifierCalls != 1 {
		t.Error("budget must be checked after authentication")
	}
	if s.calls != 1 || s.saved != nil {
		t.Errorf("limited request touched storage beyond the budget check: calls=%d", s.calls)
	}

	// An anonymous caller never reaches the budget check.
	s = &fakeStore{writeRetryAfter: 42 * time.Second}
	r, _ = writeRouter(s, ownerAccess)
	if w := patchSettings(r, "", `{}`); w.Code != 401 || s.calls != 0 {
		t.Fatal("anonymous request consumed write budget")
	}
}

func TestUpdateSettings_StorageFailures(t *testing.T) {
	s := &fakeStore{err: errFakeStore}
	r, _ := writeRouter(s, ownerAccess)
	if w := patchSettings(r, "valid", `{"autoRefresh":true}`); w.Code != 503 {
		t.Fatalf("budget/load failure: status %d, want 503: %s", w.Code, w.Body)
	}

	s = &fakeStore{saveErr: errFakeStore}
	r, _ = writeRouter(s, ownerAccess)
	if w := patchSettings(r, "valid", `{"autoRefresh":true}`); w.Code != 503 || s.saved != nil {
		t.Fatalf("save failure: status %d, want 503 and nothing recorded: %s", w.Code, w.Body)
	}
}

// A store that refuses a document the handler considered valid still yields a 400, not a 500, so a future
// mismatch between the two validation passes shows up as a client error with fields.
func TestUpdateSettings_StoreValidationErrorIsA400(t *testing.T) {
	s := &fakeStore{saveErr: settings.ValidationErrors{{Field: "language", Message: "refused by store"}}}
	r, _ := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"autoRefresh":true}`)
	if w.Code != 400 {
		t.Fatalf("status %d, want 400: %s", w.Code, w.Body)
	}
	if fields := validationFields(t, w); len(fields) != 1 || fields[0] != "language" {
		t.Errorf("fields = %v", fields)
	}
}

// ---- Premium gating ------------------------------------------------------------------------------------------

var (
	freeGuild    = &premium.PremiumRecord{Tier: premium.FreeTier}
	expiredGuild = &premium.PremiumRecord{Tier: premium.GoldTier, Days: 0}
	paidGuild    = &premium.PremiumRecord{Tier: premium.BronzeTier, Days: 12}
)

// Every setting the slash command reserves for premium guilds is refused for a free guild, with the field named,
// and nothing is saved.
func TestUpdateSettings_FreeGuildCannotChangePremiumFields(t *testing.T) {
	cases := map[string]string{
		"deleteGameSummary":     `{"deleteGameSummary":-1}`,
		"matchSummaryChannelID": `{"matchSummaryChannelID":"223456789012345678"}`,
		"autoRefresh":           `{"autoRefresh":true}`,
		"leaderboardMention":    `{"leaderboardMention":false}`,
		"leaderboardSize":       `{"leaderboardSize":5}`,
		"leaderboardMin":        `{"leaderboardMin":5}`,
		"muteSpectator":         `{"muteSpectator":true}`,
		"displayRoomCode":       `{"displayRoomCode":"never"}`,
	}
	for field, body := range cases {
		t.Run(field, func(t *testing.T) {
			for _, record := range []*premium.PremiumRecord{freeGuild, expiredGuild} {
				s := &fakeStore{premium: record}
				r, _ := writeRouter(s, ownerAccess)
				w := patchSettings(r, "valid", body)
				if w.Code != 403 {
					t.Fatalf("tier %v: status %d, want 403: %s", record.Tier, w.Code, w.Body)
				}
				if got := validationFields(t, w); len(got) != 1 || got[0] != field {
					t.Errorf("fields = %v, want [%s]", got, field)
				}
				if s.saved != nil {
					t.Fatal("premium change was saved for a non-premium guild")
				}
			}
		})
	}
}

func TestUpdateSettings_FreeGuildMayChangeFreeFields(t *testing.T) {
	s := &fakeStore{premium: freeGuild}
	r, _ := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"language":"de","unmuteDeadDuringTasks":true,"adminIDs":["223456789012345678"],"delays":{"delays":{"LOBBY":{"LOBBY":0,"TASKS":2,"DISCUSSION":0}}}}`)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if s.saved == nil || s.saved.Language != "de" || !s.saved.UnmuteDeadDuringTasks {
		t.Fatal("free change was not saved")
	}
	if s.premiumCalls != 0 {
		t.Error("premium status should only be looked up when a premium field changes")
	}
}

// A free guild may resubmit the premium values it already has (for example the whole document from GET).
func TestUpdateSettings_FreeGuildMayEchoPremiumValues(t *testing.T) {
	s := &fakeStore{premium: freeGuild}
	r, _ := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"autoRefresh":false,"leaderboardSize":3,"displayRoomCode":"always","language":"de"}`)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if s.premiumCalls != 0 {
		t.Error("unchanged premium values should not trigger a premium lookup")
	}
}

// Mixed bodies are refused as a whole: the free fields are not applied while the premium ones are rejected.
func TestUpdateSettings_MixedBodyIsRefusedWhole(t *testing.T) {
	s := &fakeStore{premium: freeGuild}
	r, _ := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"language":"de","autoRefresh":true,"muteSpectator":true}`)
	if w.Code != 403 {
		t.Fatalf("status %d, want 403: %s", w.Code, w.Body)
	}
	if got := validationFields(t, w); len(got) != 2 {
		t.Errorf("fields = %v, want both premium fields", got)
	}
	if s.saved != nil {
		t.Fatal("partial body was saved")
	}
}

func TestUpdateSettings_PremiumGuildMayChangePremiumFields(t *testing.T) {
	for _, record := range []*premium.PremiumRecord{paidGuild, {Tier: premium.SelfHostTier, Days: premium.NoExpiryCode}, {Tier: premium.TrialTier, Days: 3}} {
		s := &fakeStore{premium: record}
		r, _ := writeRouter(s, ownerAccess)
		w := patchSettings(r, "valid", `{"autoRefresh":true,"leaderboardSize":5}`)
		if w.Code != 200 {
			t.Fatalf("tier %v: status %d: %s", record.Tier, w.Code, w.Body)
		}
		if s.saved == nil || !s.saved.AutoRefresh || s.saved.LeaderboardSize != 5 {
			t.Fatalf("tier %v: change not saved", record.Tier)
		}
		if s.premiumCalls != 1 {
			t.Errorf("tier %v: premium looked up %d times, want 1", record.Tier, s.premiumCalls)
		}
	}
}

// If premium status cannot be determined the write is refused, never allowed on the assumption of premium.
func TestUpdateSettings_PremiumLookupFailureRefusesWrite(t *testing.T) {
	s := &fakeStore{premiumErr: errFakeStore}
	r, _ := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"autoRefresh":true}`)
	if w.Code != 503 || s.saved != nil {
		t.Fatalf("status %d, want 503 and nothing saved: %s", w.Code, w.Body)
	}
}

// ---- Versioning: ETag, If-Match, conflicts -------------------------------------------------------------------

func TestSettings_GetReportsETag(t *testing.T) {
	s := &fakeStore{version: 4}
	r, _ := writeRouter(s, memberAccess)
	w := bearerRequest(r, "/guild/settings?guildID="+writeGuild, "valid")
	if w.Code != 200 || w.Header().Get("ETag") != `"4"` {
		t.Fatalf("status %d ETag %q, want 200 and \"4\"", w.Code, w.Header().Get("ETag"))
	}
}

func TestUpdateSettings_WriteBumpsETag(t *testing.T) {
	s := &fakeStore{version: 4}
	r, _ := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"autoRefresh":true}`)
	if w.Code != 200 || w.Header().Get("ETag") != `"5"` {
		t.Fatalf("status %d ETag %q, want 200 and \"5\"", w.Code, w.Header().Get("ETag"))
	}
	if s.version != 5 {
		t.Errorf("store version = %d, want 5", s.version)
	}
}

func TestUpdateSettings_IfMatch(t *testing.T) {
	cases := []struct {
		name    string
		ifMatch string
		status  int
	}{
		{"current version", `"4"`, 200},
		{"current version unquoted", `4`, 200},
		{"weak tag", `W/"4"`, 200},
		{"list containing current", `"2", "4"`, 200},
		{"any", `*`, 200},
		{"stale version", `"3"`, 412},
		{"future version", `"5"`, 412},
		{"garbage", `abc`, 412},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeStore{version: 4}
			r, _ := writeRouter(s, ownerAccess)
			w := patchSettingsIfMatch(r, "valid", tc.ifMatch, `{"autoRefresh":true}`)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body)
			}
			if tc.status == 412 {
				if s.saved != nil {
					t.Fatal("stale write was saved")
				}
				if w.Header().Get("ETag") != `"4"` {
					t.Errorf("412 should carry the current ETag, got %q", w.Header().Get("ETag"))
				}
				if s.premiumCalls != 0 {
					t.Error("stale write should be refused before premium is consulted")
				}
			}
		})
	}
}

// A write that loses the race to another writer between load and save is a 409, and the other write survives.
func TestUpdateSettings_ConcurrentWriteIsAConflict(t *testing.T) {
	s := &fakeStore{version: 4, saveErr: storage.ErrSettingsConflict}
	r, _ := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"autoRefresh":true}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409: %s", w.Code, w.Body)
	}
	if s.saved != nil {
		t.Fatal("conflicting write was recorded as saved")
	}
}

// ---- Summary channel verification ---------------------------------------------------------------------------

const (
	textChannelHere   = "223456789012345678"
	voiceChannelHere  = "223456789012345679"
	threadHere        = "223456789012345680"
	textNoPostHere    = "223456789012345681"
	textChannelThere  = "323456789012345678"
	unknownChannel    = "423456789012345678"
	otherGuildForChan = "923456789012345678"
)

func fakeChannels(calls *int) channelVerifierFunc {
	return func(_ context.Context, id string) (ChannelInfo, error) {
		if calls != nil {
			*calls++
		}
		const post = discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks
		switch id {
		case textChannelHere:
			return ChannelInfo{ID: id, GuildID: writeGuild, Type: discordgo.ChannelTypeGuildText, Name: "match-summaries", Permissions: post}, nil
		case textNoPostHere:
			return ChannelInfo{ID: id, GuildID: writeGuild, Type: discordgo.ChannelTypeGuildText, Name: "read-only", Permissions: discordgo.PermissionViewChannel}, nil
		case voiceChannelHere:
			return ChannelInfo{ID: id, GuildID: writeGuild, Type: discordgo.ChannelTypeGuildVoice, Name: "voice", Permissions: post}, nil
		case threadHere:
			return ChannelInfo{ID: id, GuildID: writeGuild, Type: discordgo.ChannelTypeGuildPublicThread, Name: "thread", Permissions: post | discordgo.PermissionSendMessagesInThreads}, nil
		case textChannelThere:
			return ChannelInfo{ID: id, GuildID: otherGuildForChan, Type: discordgo.ChannelTypeGuildText, Name: "elsewhere", Permissions: post}, nil
		default:
			return ChannelInfo{}, errChannelNotFound
		}
	}
}

func TestUpdateSettings_SummaryChannelMustBeTextChannelInGuild(t *testing.T) {
	cases := []struct {
		name    string
		channel string
		status  int
	}{
		{"text channel in guild", textChannelHere, 200},
		{"thread in guild", threadHere, 200},
		{"text channel the bot cannot post in", textNoPostHere, 400},
		{"voice channel in guild", voiceChannelHere, 400},
		{"text channel in another guild", textChannelThere, 400},
		{"unknown channel", unknownChannel, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeStore{}
			r, _ := writeRouterWith(s, ownerAccess, fakeChannels(nil))
			w := patchSettings(r, "valid", `{"matchSummaryChannelID":"`+tc.channel+`"}`)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body)
			}
			if tc.status == 200 {
				if s.saved == nil || s.saved.MatchSummaryChannelID != tc.channel {
					t.Fatal("valid channel was not saved")
				}
				return
			}
			if got := validationFields(t, w); len(got) != 1 || got[0] != "matchSummaryChannelID" {
				t.Errorf("fields = %v, want [matchSummaryChannelID]", got)
			}
			if tc.channel == textNoPostHere && !strings.Contains(w.Body.String(), "Send Messages, Embed Links permissions") {
				t.Errorf("missing permissions should be named: %s", w.Body)
			}
			if s.saved != nil {
				t.Fatal("cross-guild, non-text, or unpostable channel was saved")
			}
		})
	}
}

// Clearing the channel or resubmitting the stored one needs no Discord lookup.
func TestUpdateSettings_SummaryChannelUnchangedOrClearedSkipsLookup(t *testing.T) {
	stored := settings.MakeGuildSettings()
	stored.SetMatchSummaryChannelID(textChannelThere) // whatever is stored is not re-verified on unrelated writes
	calls := 0
	s := &fakeStore{stored: stored}
	r, _ := writeRouterWith(s, ownerAccess, fakeChannels(&calls))
	if w := patchSettings(r, "valid", `{"matchSummaryChannelID":"`+textChannelThere+`","language":"de"}`); w.Code != 200 {
		t.Fatalf("echo: status %d: %s", w.Code, w.Body)
	}
	if w := patchSettings(r, "valid", `{"matchSummaryChannelID":""}`); w.Code != 200 || s.saved.MatchSummaryChannelID != "" {
		t.Fatalf("clear: status %d: %s", w.Code, w.Body)
	}
	if calls != 0 {
		t.Errorf("channel verifier called %d times, want 0", calls)
	}
}

// Without a bot token the API cannot vouch for a channel, so it refuses to change it rather than trust the client.
func TestUpdateSettings_SummaryChannelNeedsVerifier(t *testing.T) {
	s := &fakeStore{}
	r, _ := writeRouter(s, ownerAccess)
	w := patchSettings(r, "valid", `{"matchSummaryChannelID":"`+textChannelHere+`"}`)
	if w.Code != http.StatusNotImplemented || s.saved != nil {
		t.Fatalf("status %d, want 501 and nothing saved: %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "DISCORD_BOT_TOKEN") {
		t.Errorf("error should say what to configure: %s", w.Body)
	}
	// Clearing still works without a verifier.
	stored := settings.MakeGuildSettings()
	stored.SetMatchSummaryChannelID(textChannelHere)
	s = &fakeStore{stored: stored}
	r, _ = writeRouter(s, ownerAccess)
	if w := patchSettings(r, "valid", `{"matchSummaryChannelID":""}`); w.Code != 200 {
		t.Fatalf("clear without verifier: status %d: %s", w.Code, w.Body)
	}
}

func TestUpdateSettings_SummaryChannelLookupFailureRefusesWrite(t *testing.T) {
	s := &fakeStore{}
	r, _ := writeRouterWith(s, ownerAccess, channelVerifierFunc(func(context.Context, string) (ChannelInfo, error) {
		return ChannelInfo{}, errChannelUnavailable
	}))
	w := patchSettings(r, "valid", `{"matchSummaryChannelID":"`+textChannelHere+`"}`)
	if w.Code != 503 || s.saved != nil {
		t.Fatalf("status %d, want 503 and nothing saved: %s", w.Code, w.Body)
	}
}

// Premium is checked before the channel lookup, so a free guild never triggers a Discord call for this field.
func TestUpdateSettings_FreeGuildChannelChangeRefusedBeforeLookup(t *testing.T) {
	calls := 0
	s := &fakeStore{premium: freeGuild}
	r, _ := writeRouterWith(s, ownerAccess, fakeChannels(&calls))
	w := patchSettings(r, "valid", `{"matchSummaryChannelID":"`+textChannelHere+`"}`)
	if w.Code != 403 || calls != 0 {
		t.Fatalf("status %d (want 403), verifier calls %d (want 0)", w.Code, calls)
	}
}

// A router configured with a bot token but no injected verifier builds the Discord-backed one.
func TestNewRouter_BotTokenEnablesChannelVerifier(t *testing.T) {
	s := &fakeStore{}
	v := verifierFunc(func(context.Context, string, string) (VerifiedGuildAccess, error) { return ownerAccess, nil })
	r := NewRouter(Config{GuildVerifier: v, BotToken: "bot-token"}, s)
	// The real verifier will fail to reach Discord in tests; that must surface as 503, not 501.
	w := patchSettings(r, "valid", `{"matchSummaryChannelID":"`+textChannelHere+`"}`)
	if w.Code == http.StatusNotImplemented {
		t.Fatalf("bot token did not enable channel verification: %d %s", w.Code, w.Body)
	}
}

// ---- Redis-backed write budget -------------------------------------------------------------------------------

func newBudgetStore(t *testing.T) (*DataStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return &DataStore{redis: client}, mr
}

func TestReserveSettingsWrite_AllowsLimitThenRefuses(t *testing.T) {
	store, mr := newBudgetStore(t)
	ctx := context.Background()
	for i := 0; i < SettingsWriteLimit; i++ {
		retry, err := store.ReserveSettingsWrite(ctx, writeGuild)
		if err != nil || retry != 0 {
			t.Fatalf("write %d: retry=%v err=%v", i+1, retry, err)
		}
	}
	retry, err := store.ReserveSettingsWrite(ctx, writeGuild)
	if err != nil {
		t.Fatal(err)
	}
	if retry <= 0 || retry > SettingsWriteWindow {
		t.Fatalf("retry after %v, want within (0, %v]", retry, SettingsWriteWindow)
	}
	if ttl := mr.TTL(rediskey.APISettingsWriteLimit(writeGuild)); ttl <= 0 || ttl > SettingsWriteWindow {
		t.Fatalf("budget key TTL = %v, want within the window", ttl)
	}

	// Other guilds have their own budget.
	if retry, err := store.ReserveSettingsWrite(ctx, "223456789012345678"); err != nil || retry != 0 {
		t.Fatalf("other guild affected: retry=%v err=%v", retry, err)
	}

	// The window passing restores the budget.
	mr.FastForward(SettingsWriteWindow + time.Second)
	if retry, err := store.ReserveSettingsWrite(ctx, writeGuild); err != nil || retry != 0 {
		t.Fatalf("after window: retry=%v err=%v", retry, err)
	}
}

func TestReserveSettingsWrite_FailsClosedWithoutRedis(t *testing.T) {
	store, mr := newBudgetStore(t)
	mr.Close()
	if _, err := store.ReserveSettingsWrite(context.Background(), writeGuild); err == nil {
		t.Fatal("unreachable Redis must refuse the write")
	}
}

func TestReserveSettingsWrite_HonoursCancelledContext(t *testing.T) {
	store, _ := newBudgetStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.ReserveSettingsWrite(ctx, writeGuild); err == nil {
		t.Fatal("cancelled context was ignored")
	}
}
