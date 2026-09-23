package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/game"
)

// testLanguages stands in for locale.GetLanguages() so these tests never touch the locales directory.
var testLanguages = map[string]string{"en": "English", "de": "German", "pt-BR": "Brazilian Portuguese"}

const (
	validID   = "123456789012345678"
	validID2  = "234567890123456789"
	validID3  = "345678901234567890"
	epochID   = "1420070400000"        // exactly the Discord epoch: the smallest accepted ID
	preEpoch  = "1420070399999"        // one below the epoch
	tooBigID  = "18446744073709551616" // uint64 max + 1
	uint64Max = "18446744073709551615"
)

// fieldsOf returns the sorted offending fields, or fails the test if err is not a ValidationErrors.
func fieldsOf(t *testing.T, err error) []string {
	t.Helper()
	if err == nil {
		return nil
	}
	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}
	fields := verrs.Fields()
	sort.Strings(fields)
	return fields
}

// assertOnlyFields checks that Validate rejected exactly the given fields (sorted), and nothing else.
func assertOnlyFields(t *testing.T, err error, want ...string) {
	t.Helper()
	got := fieldsOf(t, err)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rejected fields = %v, want %v\n  err: %v", got, want, err)
	}
}

func assertValid(t *testing.T, sett *GuildSettings) {
	t.Helper()
	if err := sett.Validate(testLanguages); err != nil {
		t.Errorf("expected valid settings, got: %v", err)
	}
}

func TestValidate_DefaultsAreValid(t *testing.T) {
	assertValid(t, MakeGuildSettings())
}

func TestValidate_NilSettings(t *testing.T) {
	var sett *GuildSettings
	if err := sett.Validate(testLanguages); err == nil {
		t.Fatal("nil settings must not validate")
	}
}

// The committed v8 fixture is a real, fully-populated document; it must pass so the validator can never reject
// what the bot itself writes.
func TestValidate_V8FixtureIsValid(t *testing.T) {
	var sett GuildSettings
	if err := json.Unmarshal(loadFixture(t), &sett); err != nil {
		t.Fatal(err)
	}
	assertValid(t, &sett)
}

// Every value the slash commands would accept must also pass Validate. Each case mutates the defaults with the
// same setter the corresponding slash command calls.
func TestValidate_AcceptsEverySlashCommandValue(t *testing.T) {
	cases := map[string]func(*GuildSettings){
		"one admin":               func(s *GuildSettings) { s.SetAdminUserIDs([]string{validID}) },
		"many admins":             func(s *GuildSettings) { s.SetAdminUserIDs([]string{validID, validID2, validID3}) },
		"admin at epoch boundary": func(s *GuildSettings) { s.SetAdminUserIDs([]string{epochID}) },
		"admin at uint64 max":     func(s *GuildSettings) { s.SetAdminUserIDs([]string{uint64Max}) },
		"cleared admins":          func(s *GuildSettings) { s.SetAdminUserIDs([]string{}) },
		"nil admins":              func(s *GuildSettings) { s.SetAdminUserIDs(nil) },
		"operator roles":          func(s *GuildSettings) { s.SetPermissionRoleIDs([]string{validID, validID2}) },
		"nil roles":               func(s *GuildSettings) { s.SetPermissionRoleIDs(nil) },
		"same id as admin and role": func(s *GuildSettings) {
			s.SetAdminUserIDs([]string{validID})
			s.SetPermissionRoleIDs([]string{validID})
		},
		"language de":             func(s *GuildSettings) { s.SetLanguage("de") },
		"language with region":    func(s *GuildSettings) { s.SetLanguage("pt-BR") },
		"map detailed":            func(s *GuildSettings) { s.SetMapDetailed(true) },
		"map simple":              func(s *GuildSettings) { s.SetMapDetailed(false) },
		"mute rule flipped":       func(s *GuildSettings) { s.SetVoiceRule(true, game.LOBBY, "dead", true) },
		"deaf rule flipped":       func(s *GuildSettings) { s.SetVoiceRule(false, game.DISCUSS, "alive", true) },
		"delay min":               func(s *GuildSettings) { s.SetDelay(game.LOBBY, game.TASKS, MinDelaySeconds) },
		"delay max":               func(s *GuildSettings) { s.SetDelay(game.DISCUSS, game.TASKS, MaxDelaySeconds) },
		"delay same phase":        func(s *GuildSettings) { s.SetDelay(game.TASKS, game.TASKS, 4) },
		"summary never delete":    func(s *GuildSettings) { s.SetDeleteGameSummaryMinutes(-1) },
		"summary delete now":      func(s *GuildSettings) { s.SetDeleteGameSummaryMinutes(0) },
		"summary max":             func(s *GuildSettings) { s.SetDeleteGameSummaryMinutes(MaxDeleteGameSummaryMinutes) },
		"unmute dead":             func(s *GuildSettings) { s.SetUnmuteDeadDuringTasks(true) },
		"auto refresh":            func(s *GuildSettings) { s.SetAutoRefresh(true) },
		"summary channel":         func(s *GuildSettings) { s.SetMatchSummaryChannelID(validID) },
		"summary channel cleared": func(s *GuildSettings) { s.SetMatchSummaryChannelID("") },
		"leaderboard no mention":  func(s *GuildSettings) { s.SetLeaderboardMention(false) },
		"leaderboard size min":    func(s *GuildSettings) { s.SetLeaderboardSize(MinLeaderboardSize) },
		"leaderboard size max":    func(s *GuildSettings) { s.SetLeaderboardSize(MaxLeaderboardSize) },
		"leaderboard min min":     func(s *GuildSettings) { s.SetLeaderboardMin(MinLeaderboardMin) },
		"leaderboard min max":     func(s *GuildSettings) { s.SetLeaderboardMin(MaxLeaderboardMin) },
		"mute spectators":         func(s *GuildSettings) { s.SetMuteSpectator(true) },
		"room code spoiler":       func(s *GuildSettings) { s.SetDisplayRoomCode(DisplayRoomCodeSpoiler) },
		"room code never":         func(s *GuildSettings) { s.SetDisplayRoomCode(DisplayRoomCodeNever) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			sett := MakeGuildSettings()
			mutate(sett)
			assertValid(t, sett)
		})
	}
}

// Each case sets exactly one bad value on otherwise-default settings and expects exactly that field to be rejected.
func TestValidate_RejectsSingleBadField(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*GuildSettings)
		field  string
	}{
		{"admin empty string", func(s *GuildSettings) { s.AdminUserIDs = []string{""} }, "adminIDs[0]"},
		{"admin not numeric", func(s *GuildSettings) { s.AdminUserIDs = []string{"not-an-id"} }, "adminIDs[0]"},
		{"admin mention syntax", func(s *GuildSettings) { s.AdminUserIDs = []string{"<@" + validID + ">"} }, "adminIDs[0]"},
		{"admin negative", func(s *GuildSettings) { s.AdminUserIDs = []string{"-" + validID} }, "adminIDs[0]"},
		{"admin explicit plus sign", func(s *GuildSettings) { s.AdminUserIDs = []string{"+" + validID} }, "adminIDs[0]"},
		{"admin whitespace padded", func(s *GuildSettings) { s.AdminUserIDs = []string{" " + validID} }, "adminIDs[0]"},
		{"admin float", func(s *GuildSettings) { s.AdminUserIDs = []string{"1234567890123456.78"} }, "adminIDs[0]"},
		{"admin hex", func(s *GuildSettings) { s.AdminUserIDs = []string{"0x1B69B4BACD05F15"} }, "adminIDs[0]"},
		{"admin before epoch", func(s *GuildSettings) { s.AdminUserIDs = []string{preEpoch} }, "adminIDs[0]"},
		{"admin zero", func(s *GuildSettings) { s.AdminUserIDs = []string{"0"} }, "adminIDs[0]"},
		{"admin overflows uint64", func(s *GuildSettings) { s.AdminUserIDs = []string{tooBigID} }, "adminIDs[0]"},
		{"admin second entry bad", func(s *GuildSettings) { s.AdminUserIDs = []string{validID, "bad"} }, "adminIDs[1]"},
		{"admin duplicate", func(s *GuildSettings) { s.AdminUserIDs = []string{validID, validID2, validID} }, "adminIDs[2]"},
		{"role not numeric", func(s *GuildSettings) { s.PermissionRoleIDs = []string{"@everyone"} }, "permissionRoleIDs[0]"},
		{"role mention syntax", func(s *GuildSettings) { s.PermissionRoleIDs = []string{"<@&" + validID + ">"} }, "permissionRoleIDs[0]"},
		{"role duplicate", func(s *GuildSettings) { s.PermissionRoleIDs = []string{validID, validID} }, "permissionRoleIDs[1]"},
		{"language empty", func(s *GuildSettings) { s.Language = "" }, "language"},
		{"language unknown", func(s *GuildSettings) { s.Language = "xx" }, "language"},
		{"language wrong case", func(s *GuildSettings) { s.Language = "EN" }, "language"},
		{"language region without base", func(s *GuildSettings) { s.Language = "pt" }, "language"},
		{"language padded", func(s *GuildSettings) { s.Language = "en " }, "language"},
		{"language display name", func(s *GuildSettings) { s.Language = "English" }, "language"},
		{"map empty", func(s *GuildSettings) { s.MapVersion = "" }, "mapVersion"},
		{"map unknown", func(s *GuildSettings) { s.MapVersion = "ultra" }, "mapVersion"},
		{"map wrong case", func(s *GuildSettings) { s.MapVersion = "Detailed" }, "mapVersion"},
		{"map boolean-ish", func(s *GuildSettings) { s.MapVersion = "true" }, "mapVersion"},
		{"summary below min", func(s *GuildSettings) { s.DeleteGameSummaryMinutes = MinDeleteGameSummaryMinutes - 1 }, "deleteGameSummary"},
		{"summary above max", func(s *GuildSettings) { s.DeleteGameSummaryMinutes = MaxDeleteGameSummaryMinutes + 1 }, "deleteGameSummary"},
		{"summary huge", func(s *GuildSettings) { s.DeleteGameSummaryMinutes = 1 << 40 }, "deleteGameSummary"},
		{"summary channel not numeric", func(s *GuildSettings) { s.MatchSummaryChannelID = "general" }, "matchSummaryChannelID"},
		{"summary channel mention", func(s *GuildSettings) { s.MatchSummaryChannelID = "<#" + validID + ">" }, "matchSummaryChannelID"},
		{"summary channel before epoch", func(s *GuildSettings) { s.MatchSummaryChannelID = preEpoch }, "matchSummaryChannelID"},
		{"leaderboard size zero", func(s *GuildSettings) { s.LeaderboardSize = 0 }, "leaderboardSize"},
		{"leaderboard size negative", func(s *GuildSettings) { s.LeaderboardSize = -3 }, "leaderboardSize"},
		{"leaderboard size above max", func(s *GuildSettings) { s.LeaderboardSize = MaxLeaderboardSize + 1 }, "leaderboardSize"},
		{"leaderboard min zero", func(s *GuildSettings) { s.LeaderboardMin = 0 }, "leaderboardMin"},
		{"leaderboard min above max", func(s *GuildSettings) { s.LeaderboardMin = MaxLeaderboardMin + 1 }, "leaderboardMin"},
		{"room code empty", func(s *GuildSettings) { s.DisplayRoomCode = "" }, "displayRoomCode"},
		{"room code unknown", func(s *GuildSettings) { s.DisplayRoomCode = "sometimes" }, "displayRoomCode"},
		{"room code wrong case", func(s *GuildSettings) { s.DisplayRoomCode = "Always" }, "displayRoomCode"},
		{"room code boolean-ish", func(s *GuildSettings) { s.DisplayRoomCode = "false" }, "displayRoomCode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sett := MakeGuildSettings()
			tc.mutate(sett)
			assertOnlyFields(t, sett.Validate(testLanguages), tc.field)
		})
	}
}

func TestValidate_IDListLimits(t *testing.T) {
	ids := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("%d", 1000000000000000000+i)
		}
		return out
	}

	sett := MakeGuildSettings()
	sett.AdminUserIDs = ids(MaxAdminUserIDs)
	sett.PermissionRoleIDs = ids(MaxPermissionRoleIDs)
	assertValid(t, sett)

	sett.AdminUserIDs = ids(MaxAdminUserIDs + 1)
	assertOnlyFields(t, sett.Validate(testLanguages), "adminIDs")

	sett = MakeGuildSettings()
	sett.PermissionRoleIDs = ids(MaxPermissionRoleIDs + 1)
	assertOnlyFields(t, sett.Validate(testLanguages), "permissionRoleIDs")
}

// A duplicate that is also malformed is reported once, as malformed, and does not poison the seen-set.
func TestValidate_DuplicateReportingIsPerEntry(t *testing.T) {
	sett := MakeGuildSettings()
	sett.AdminUserIDs = []string{"bad", "bad", validID, validID, validID}
	assertOnlyFields(t, sett.Validate(testLanguages), "adminIDs[0]", "adminIDs[1]", "adminIDs[3]", "adminIDs[4]")
}

func TestValidate_VoiceRules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*GuildSettings)
		fields []string
	}{
		{"mute rules nil", func(s *GuildSettings) { s.VoiceRules.MuteRules = nil }, []string{"voiceRules.MuteRules"}},
		{"deaf rules nil", func(s *GuildSettings) { s.VoiceRules.DeafRules = nil }, []string{"voiceRules.DeafRules"}},
		{"both nil", func(s *GuildSettings) { s.VoiceRules = game.VoiceRules{} }, []string{"voiceRules.MuteRules", "voiceRules.DeafRules"}},
		{"mute rules empty map", func(s *GuildSettings) { s.VoiceRules.MuteRules = map[game.PhaseNameString]map[string]bool{} },
			[]string{"voiceRules.MuteRules.LOBBY", "voiceRules.MuteRules.TASKS", "voiceRules.MuteRules.DISCUSSION"}},
		{"phase missing", func(s *GuildSettings) { delete(s.VoiceRules.MuteRules, "TASKS") }, []string{"voiceRules.MuteRules.TASKS"}},
		{"phase nil map", func(s *GuildSettings) { s.VoiceRules.DeafRules["LOBBY"] = nil },
			[]string{"voiceRules.DeafRules.LOBBY.alive", "voiceRules.DeafRules.LOBBY.dead"}},
		{"alive missing", func(s *GuildSettings) { delete(s.VoiceRules.MuteRules["DISCUSSION"], "alive") }, []string{"voiceRules.MuteRules.DISCUSSION.alive"}},
		{"dead missing", func(s *GuildSettings) { delete(s.VoiceRules.DeafRules["TASKS"], "dead") }, []string{"voiceRules.DeafRules.TASKS.dead"}},
		{"unknown player state", func(s *GuildSettings) { s.VoiceRules.MuteRules["TASKS"]["spectator"] = true }, []string{"voiceRules.MuteRules.TASKS.spectator"}},
		{"player state wrong case", func(s *GuildSettings) { s.VoiceRules.MuteRules["TASKS"]["Alive"] = true }, []string{"voiceRules.MuteRules.TASKS.Alive"}},
		// The getters look up PhaseNames[phase] ("LOBBY"); any other spelling is silently ignored at runtime, so it must be rejected here.
		{"phase lowercase", func(s *GuildSettings) {
			s.VoiceRules.MuteRules["lobby"] = s.VoiceRules.MuteRules["LOBBY"]
		}, []string{"voiceRules.MuteRules.lobby"}},
		{"phase alias DISCUSS", func(s *GuildSettings) {
			s.VoiceRules.DeafRules["DISCUSS"] = s.VoiceRules.DeafRules["DISCUSSION"]
		}, []string{"voiceRules.DeafRules.DISCUSS"}},
		{"phase MENU has no rules", func(s *GuildSettings) {
			s.VoiceRules.MuteRules["MENU"] = map[string]bool{"alive": true, "dead": true}
		}, []string{"voiceRules.MuteRules.MENU"}},
		{"phase GAMEOVER", func(s *GuildSettings) {
			s.VoiceRules.MuteRules["GAMEOVER"] = map[string]bool{"alive": true, "dead": true}
		}, []string{"voiceRules.MuteRules.GAMEOVER"}},
		{"phase renamed not added", func(s *GuildSettings) {
			s.VoiceRules.MuteRules["lobby"] = s.VoiceRules.MuteRules["LOBBY"]
			delete(s.VoiceRules.MuteRules, "LOBBY")
		}, []string{"voiceRules.MuteRules.lobby", "voiceRules.MuteRules.LOBBY"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sett := MakeGuildSettings()
			tc.mutate(sett)
			assertOnlyFields(t, sett.Validate(testLanguages), tc.fields...)
		})
	}
}

func TestValidate_Delays(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*GuildSettings)
		fields []string
	}{
		{"delays nil", func(s *GuildSettings) { s.Delays = game.GameDelays{} }, []string{"delays.delays"}},
		{"delays empty", func(s *GuildSettings) { s.Delays.Delays = map[game.PhaseNameString]map[game.PhaseNameString]int{} },
			[]string{"delays.delays.LOBBY", "delays.delays.TASKS", "delays.delays.DISCUSSION"}},
		{"origin missing", func(s *GuildSettings) { delete(s.Delays.Delays, "DISCUSSION") }, []string{"delays.delays.DISCUSSION"}},
		{"origin nil map", func(s *GuildSettings) { s.Delays.Delays["TASKS"] = nil },
			[]string{"delays.delays.TASKS.LOBBY", "delays.delays.TASKS.TASKS", "delays.delays.TASKS.DISCUSSION"}},
		{"destination missing", func(s *GuildSettings) { delete(s.Delays.Delays["LOBBY"], "TASKS") }, []string{"delays.delays.LOBBY.TASKS"}},
		{"negative", func(s *GuildSettings) { s.SetDelay(game.LOBBY, game.TASKS, -1) }, []string{"delays.delays.LOBBY.TASKS"}},
		{"above max", func(s *GuildSettings) { s.SetDelay(game.DISCUSS, game.LOBBY, MaxDelaySeconds+1) }, []string{"delays.delays.DISCUSSION.LOBBY"}},
		{"huge", func(s *GuildSettings) { s.SetDelay(game.TASKS, game.DISCUSS, 1<<31) }, []string{"delays.delays.TASKS.DISCUSSION"}},
		{"two bad in one row", func(s *GuildSettings) {
			s.SetDelay(game.TASKS, game.LOBBY, -5)
			s.SetDelay(game.TASKS, game.DISCUSS, 99)
		}, []string{"delays.delays.TASKS.LOBBY", "delays.delays.TASKS.DISCUSSION"}},
		{"unknown origin", func(s *GuildSettings) {
			s.Delays.Delays["MENU"] = map[game.PhaseNameString]int{"LOBBY": 1, "TASKS": 1, "DISCUSSION": 1}
		}, []string{"delays.delays.MENU"}},
		{"unknown destination", func(s *GuildSettings) { s.Delays.Delays["LOBBY"]["MENU"] = 1 }, []string{"delays.delays.LOBBY.MENU"}},
		{"lowercase destination", func(s *GuildSettings) { s.Delays.Delays["LOBBY"]["tasks"] = 1 }, []string{"delays.delays.LOBBY.tasks"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sett := MakeGuildSettings()
			tc.mutate(sett)
			assertOnlyFields(t, sett.Validate(testLanguages), tc.fields...)
		})
	}
}

// Every problem is reported in one pass rather than stopping at the first.
func TestValidate_ReportsAllProblemsAtOnce(t *testing.T) {
	sett := MakeGuildSettings()
	sett.Language = "klingon"
	sett.MapVersion = "3d"
	sett.LeaderboardSize = 0
	sett.LeaderboardMin = 1000
	sett.DeleteGameSummaryMinutes = -2
	sett.DisplayRoomCode = "maybe"
	sett.AdminUserIDs = []string{"x"}
	sett.MatchSummaryChannelID = "y"
	sett.SetDelay(game.LOBBY, game.TASKS, 11)
	delete(sett.VoiceRules.DeafRules, "TASKS")

	assertOnlyFields(t, sett.Validate(testLanguages),
		"language", "mapVersion", "leaderboardSize", "leaderboardMin", "deleteGameSummary",
		"displayRoomCode", "adminIDs[0]", "matchSummaryChannelID", "delays.delays.LOBBY.TASKS", "voiceRules.DeafRules.TASKS")
}

func TestValidate_ErrorMessagesNameTheValue(t *testing.T) {
	sett := MakeGuildSettings()
	sett.Language = "xx"
	sett.LeaderboardSize = 11
	err := sett.Validate(testLanguages)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{`"xx"`, "de, en, pt-BR", "11", "[1, 10]", "language:", "leaderboardSize:"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q should mention %q", msg, want)
		}
	}
}

// With no installed languages at all (the API process never loads the locale bundle), every language is rejected,
// including the default. That is deliberate: the caller must supply the language set rather than have the
// validator quietly accept anything.
func TestValidate_NoLanguagesRejectsEverything(t *testing.T) {
	sett := MakeGuildSettings()
	assertOnlyFields(t, sett.Validate(map[string]string{}), "language")
	assertOnlyFields(t, sett.Validate(nil), "language")
}

// FieldError values serialize to something an API client can act on.
func TestValidationErrors_JSON(t *testing.T) {
	sett := MakeGuildSettings()
	sett.LeaderboardSize = 0
	err := sett.Validate(testLanguages)
	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	encoded, jsonErr := json.Marshal(verrs)
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}
	var decoded []map[string]string
	if jsonErr := json.Unmarshal(encoded, &decoded); jsonErr != nil {
		t.Fatal(jsonErr)
	}
	if len(decoded) != 1 || decoded[0]["field"] != "leaderboardSize" || decoded[0]["message"] == "" {
		t.Errorf("unexpected JSON: %s", encoded)
	}
}

// ---- FillDefaults -------------------------------------------------------------------------------------------

// A guild whose stored blob predates several fields decodes with zeros and nil maps. The getters tolerate that at
// read time; FillDefaults makes the same substitutions so the document validates as it behaves.
func TestFillDefaults_LegacyBlobBecomesValid(t *testing.T) {
	var sett GuildSettings
	if err := json.Unmarshal([]byte(`{"language":"en","adminIDs":[]}`), &sett); err != nil {
		t.Fatal(err)
	}
	if err := sett.Validate(testLanguages); err == nil {
		t.Fatal("legacy blob should not validate before FillDefaults")
	}
	sett.FillDefaults()
	assertValid(t, &sett)

	want := MakeGuildSettings()
	if !reflect.DeepEqual(sett.VoiceRules, want.VoiceRules) || !reflect.DeepEqual(sett.Delays, want.Delays) {
		t.Error("FillDefaults should install the default rules and delays")
	}
	if sett.LeaderboardSize != DefaultLeaderboardSize || sett.LeaderboardMin != DefaultLeaderboardMin {
		t.Errorf("leaderboard = %d/%d, want defaults", sett.LeaderboardSize, sett.LeaderboardMin)
	}
	if sett.DisplayRoomCode != DisplayRoomCodeAlways || sett.MapVersion != MapVersionSimple {
		t.Errorf("displayRoomCode=%q mapVersion=%q, want defaults", sett.DisplayRoomCode, sett.MapVersion)
	}
	if sett.PermissionRoleIDs == nil {
		t.Error("nil role list should become empty")
	}
}

// A zero struct becomes the defaults, with one deliberate exception: a bool cannot distinguish "unset" from
// false, and the existing getter already reports legacy blobs without leaderboardMention as false. FillDefaults
// mirrors the getters, so it leaves that field alone rather than flipping a guild's stored false to true.
func TestFillDefaults_EmptyStructBecomesDefaults(t *testing.T) {
	var sett GuildSettings
	sett.FillDefaults()
	assertValid(t, &sett)
	if sett.LeaderboardMention {
		t.Error("FillDefaults must not invent a true for a bool it cannot know was unset")
	}
	sett.LeaderboardMention = true
	got, _ := json.Marshal(&sett)
	want, _ := json.Marshal(MakeGuildSettings())
	if string(got) != string(want) {
		t.Errorf("FillDefaults on a zero struct should otherwise equal MakeGuildSettings:\n got %s\nwant %s", got, want)
	}
}

// FillDefaults must never mask invalid input: it only fills the "unset" zero values and leaves everything else.
func TestFillDefaults_DoesNotRepairInvalidValues(t *testing.T) {
	sett := MakeGuildSettings()
	sett.Language = "xx"
	sett.MapVersion = "weird"
	sett.LeaderboardSize = 99
	sett.LeaderboardMin = -1 // < 1 is "unset" for the getters, so this one IS filled
	sett.DisplayRoomCode = "nope"
	delete(sett.Delays.Delays, "LOBBY")                    // partially missing is not "unset"
	sett.VoiceRules.MuteRules["TASKS"] = map[string]bool{} // present but incomplete
	sett.FillDefaults()
	assertOnlyFields(t, sett.Validate(testLanguages),
		"language", "mapVersion", "leaderboardSize", "displayRoomCode", "delays.delays.LOBBY",
		"voiceRules.MuteRules.TASKS.alive", "voiceRules.MuteRules.TASKS.dead")
}

func TestFillDefaults_PreservesExistingValues(t *testing.T) {
	var sett GuildSettings
	if err := json.Unmarshal(loadFixture(t), &sett); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(&sett)
	sett.FillDefaults()
	after, _ := json.Marshal(&sett)
	if string(before) != string(after) {
		t.Errorf("FillDefaults changed a complete document:\nbefore %s\nafter  %s", before, after)
	}
}

// ---- UnmarshalStrict ----------------------------------------------------------------------------------------

func TestUnmarshalStrict_AcceptsFixture(t *testing.T) {
	var sett GuildSettings
	if err := UnmarshalStrict(loadFixture(t), &sett); err != nil {
		t.Fatalf("fixture should decode strictly: %v", err)
	}
	assertValid(t, &sett)
	if sett.Language != "de" || sett.LeaderboardSize != 5 {
		t.Errorf("fixture values not decoded: language=%q size=%d", sett.Language, sett.LeaderboardSize)
	}
}

func TestUnmarshalStrict_RoundTripsDefaults(t *testing.T) {
	encoded, err := json.Marshal(MakeGuildSettings())
	if err != nil {
		t.Fatal(err)
	}
	var sett GuildSettings
	if err := UnmarshalStrict(encoded, &sett); err != nil {
		t.Fatalf("encoded defaults should decode strictly: %v", err)
	}
	assertValid(t, &sett)
}

// Documents encoding/json would accept, or partially accept, that must be rejected outright.
func TestUnmarshalStrict_RejectsMalformedDocuments(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"empty", ``},
		{"whitespace", "  \n\t "},
		{"null", `null`},
		{"null padded", ` null `},
		{"array", `[]`},
		{"string", `"language"`},
		{"number", `3`},
		{"truncated", `{"language":"en"`},
		{"trailing garbage", `{"language":"en"} x`},
		{"two documents", `{"language":"en"}{"language":"de"}`},
		{"trailing null", `{"language":"en"} null`},
		{"unknown top-level field", `{"languag":"en"}`},
		{"unknown field mixed with valid", `{"language":"en","turboMode":true}`},
		{"unexported lock field", `{"lock":{}}`},
		{"json key wrong case", `{"Language":"en"}`},
		{"json key all lowercase typo", `{"leaderboardsize":5}`},
		{"json key all caps", `{"AUTOREFRESH":true}`},
		{"nested key wrong case", `{"delays":{"Delays":{}}}`},
		{"go field name instead of json tag", `{"AdminUserIDs":[]}`},
		{"unknown nested field in delays", `{"delays":{"delay":{}}}`},
		{"unknown nested field in voiceRules", `{"voiceRules":{"muteRules":{}}}`},
		{"string for int", `{"leaderboardSize":"3"}`},
		{"float for int", `{"leaderboardSize":3.5}`},
		{"exponent for int", `{"leaderboardSize":1e1}`},
		{"int overflow", `{"leaderboardSize":99999999999999999999}`},
		{"bool for int", `{"leaderboardSize":true}`},
		{"string for bool", `{"autoRefresh":"true"}`},
		{"int for bool", `{"autoRefresh":1}`},
		{"int for string", `{"language":1}`},
		{"number for id", `{"adminIDs":[123456789012345678]}`},
		{"string for array", `{"adminIDs":"123456789012345678"}`},
		{"object for array", `{"adminIDs":{"0":"123456789012345678"}}`},
		{"int for rule bool", `{"voiceRules":{"MuteRules":{"LOBBY":{"alive":1,"dead":0}}}}`},
		{"string for delay", `{"delays":{"delays":{"LOBBY":{"TASKS":"7"}}}}`},
		{"array for delay row", `{"delays":{"delays":{"LOBBY":[7]}}}`},
		{"null scalar", `{"language":null}`},
		{"null bool", `{"autoRefresh":null}`},
		{"null array", `{"adminIDs":null}`},
		{"null inside array", `{"adminIDs":[null]}`},
		{"null after valid element", `{"adminIDs":["` + validID + `",null]}`},
		{"null delays object", `{"delays":null}`},
		{"null delays map", `{"delays":{"delays":null}}`},
		{"null delay row", `{"delays":{"delays":{"TASKS":null}}}`},
		{"null delay value", `{"delays":{"delays":{"TASKS":{"LOBBY":null}}}}`},
		{"null voiceRules", `{"voiceRules":null}`},
		{"null rule table", `{"voiceRules":{"MuteRules":null}}`},
		{"null rule row", `{"voiceRules":{"DeafRules":{"TASKS":null}}}`},
		{"null rule value", `{"voiceRules":{"MuteRules":{"LOBBY":{"alive":null,"dead":false}}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sett := MakeGuildSettings()
			if err := UnmarshalStrict([]byte(tc.body), sett); err == nil {
				t.Errorf("body %q should be rejected", tc.body)
			}
		})
	}
}

func TestUnmarshalStrict_NilDestination(t *testing.T) {
	if err := UnmarshalStrict([]byte(`{}`), nil); err == nil {
		t.Error("nil destination should be rejected")
	}
}

// Decoding over a copy of stored settings: present keys overwrite, absent keys are kept. This is the merge
// semantics a PATCH-style write relies on.
func TestUnmarshalStrict_MergesOverExisting(t *testing.T) {
	sett := MakeGuildSettings()
	sett.SetLanguage("de")
	sett.SetLeaderboardSize(7)
	sett.SetAdminUserIDs([]string{validID})

	if err := UnmarshalStrict([]byte(`{"leaderboardSize":2,"muteSpectator":true}`), sett); err != nil {
		t.Fatal(err)
	}
	assertValid(t, sett)
	if sett.LeaderboardSize != 2 || !sett.MuteSpectator {
		t.Errorf("present keys not applied: size=%d muteSpectator=%v", sett.LeaderboardSize, sett.MuteSpectator)
	}
	if sett.Language != "de" || !reflect.DeepEqual(sett.AdminUserIDs, []string{validID}) {
		t.Errorf("absent keys were altered: language=%q admins=%v", sett.Language, sett.AdminUserIDs)
	}
	if sett.GetDelay(game.DISCUSS, game.TASKS) != 7 {
		t.Error("absent nested delays were altered")
	}
}

func TestUnmarshalStrict_EmptyObjectIsNoOp(t *testing.T) {
	sett := MakeGuildSettings()
	before, _ := json.Marshal(sett)
	if err := UnmarshalStrict([]byte(`{}`), sett); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(sett)
	if string(before) != string(after) {
		t.Errorf("{} changed the document:\nbefore %s\nafter  %s", before, after)
	}
}

// Arrays replace wholesale (they do not append), so a client can clear or rewrite the admin list.
func TestUnmarshalStrict_ArraysReplace(t *testing.T) {
	sett := MakeGuildSettings()
	sett.SetAdminUserIDs([]string{validID, validID2})
	if err := UnmarshalStrict([]byte(`{"adminIDs":["`+validID3+`"]}`), sett); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sett.AdminUserIDs, []string{validID3}) {
		t.Errorf("adminIDs = %v, want replaced", sett.AdminUserIDs)
	}
	if err := UnmarshalStrict([]byte(`{"adminIDs":[]}`), sett); err != nil {
		t.Fatal(err)
	}
	if len(sett.AdminUserIDs) != 0 {
		t.Errorf("adminIDs = %v, want cleared", sett.AdminUserIDs)
	}
	assertValid(t, sett)
}

// The outer rule/delay tables merge key-by-key, but encoding/json builds a fresh inner map for each row it
// decodes, so a row that is present replaces the stored row wholesale. A partial row therefore loses the
// siblings it omitted, and Validate must name them rather than let the gap reach storage.
func TestUnmarshalStrict_RowsReplaceNotMerge(t *testing.T) {
	sett := MakeGuildSettings()
	if err := UnmarshalStrict([]byte(`{"delays":{"delays":{"LOBBY":{"TASKS":2}}}}`), sett); err != nil {
		t.Fatal(err)
	}
	assertOnlyFields(t, sett.Validate(testLanguages), "delays.delays.LOBBY.LOBBY", "delays.delays.LOBBY.DISCUSSION")
	if sett.GetDelay(game.DISCUSS, game.LOBBY) != 6 {
		t.Error("rows that were not sent should keep their values")
	}

	sett = MakeGuildSettings()
	if err := UnmarshalStrict([]byte(`{"voiceRules":{"MuteRules":{"TASKS":{"dead":true}}}}`), sett); err != nil {
		t.Fatal(err)
	}
	assertOnlyFields(t, sett.Validate(testLanguages), "voiceRules.MuteRules.TASKS.alive")
}

// Sending a complete row is the supported way to change one rule or delay: the other rows are untouched.
func TestUnmarshalStrict_FullRowReplacesOnlyThatRow(t *testing.T) {
	sett := MakeGuildSettings()
	if err := UnmarshalStrict([]byte(`{"delays":{"delays":{"LOBBY":{"LOBBY":0,"TASKS":2,"DISCUSSION":0}}}}`), sett); err != nil {
		t.Fatal(err)
	}
	assertValid(t, sett)
	if sett.GetDelay(game.LOBBY, game.TASKS) != 2 {
		t.Error("LOBBY->TASKS not updated")
	}
	if sett.GetDelay(game.DISCUSS, game.LOBBY) != 6 || sett.GetDelay(game.TASKS, game.LOBBY) != 1 {
		t.Error("other rows should keep their values")
	}

	if err := UnmarshalStrict([]byte(`{"voiceRules":{"DeafRules":{"DISCUSSION":{"alive":true,"dead":true}}}}`), sett); err != nil {
		t.Fatal(err)
	}
	assertValid(t, sett)
	if !sett.GetVoiceRule(false, game.DISCUSS, "alive") || !sett.GetVoiceRule(false, game.DISCUSS, "dead") {
		t.Error("DISCUSSION deaf row not updated")
	}
	if !sett.GetVoiceRule(false, game.TASKS, "alive") || sett.GetVoiceRule(true, game.DISCUSS, "alive") {
		t.Error("other rule rows should keep their values")
	}
}

// Duplicate keys: encoding/json keeps the last one. Document the behaviour so a change in the decoder is noticed.
func TestUnmarshalStrict_DuplicateKeyLastWins(t *testing.T) {
	sett := MakeGuildSettings()
	if err := UnmarshalStrict([]byte(`{"leaderboardSize":2,"leaderboardSize":9}`), sett); err != nil {
		t.Fatal(err)
	}
	if sett.LeaderboardSize != 9 {
		t.Errorf("leaderboardSize = %d, want 9 (last duplicate wins)", sett.LeaderboardSize)
	}
}

// ---- End-to-end scenarios: decode over stored settings, then validate --------------------------------------

// applyBody mimics what the write endpoint will do: copy the stored settings, decode the request body over them,
// and validate the result.
func applyBody(t *testing.T, stored []byte, body string) (*GuildSettings, error) {
	t.Helper()
	var sett GuildSettings
	if err := json.Unmarshal(stored, &sett); err != nil {
		t.Fatal(err)
	}
	sett.FillDefaults()
	if err := UnmarshalStrict([]byte(body), &sett); err != nil {
		return nil, err
	}
	return &sett, sett.Validate(testLanguages)
}

func TestScenario_WellFormedBodiesAreAccepted(t *testing.T) {
	stored := loadFixture(t)
	bodies := map[string]string{
		"single scalar":       `{"autoRefresh":false}`,
		"language change":     `{"language":"en"}`,
		"all scalars":         `{"language":"en","mapVersion":"simple","deleteGameSummary":-1,"unmuteDeadDuringTasks":false,"autoRefresh":false,"matchSummaryChannelID":"","leaderboardMention":true,"leaderboardSize":10,"leaderboardMin":100,"muteSpectator":false,"displayRoomCode":"never"}`,
		"replace admins":      `{"adminIDs":["` + validID2 + `","` + validID3 + `"]}`,
		"clear roles":         `{"permissionRoleIDs":[]}`,
		"one delay row":       `{"delays":{"delays":{"TASKS":{"LOBBY":10,"TASKS":0,"DISCUSSION":0}}}}`,
		"one rule row":        `{"voiceRules":{"DeafRules":{"DISCUSSION":{"alive":false,"dead":false}}}}`,
		"full rules replace":  `{"voiceRules":{"MuteRules":{"LOBBY":{"alive":true,"dead":true},"TASKS":{"alive":true,"dead":true},"DISCUSSION":{"alive":true,"dead":true}},"DeafRules":{"LOBBY":{"alive":false,"dead":false},"TASKS":{"alive":false,"dead":false},"DISCUSSION":{"alive":false,"dead":false}}}}`,
		"whole fixture":       string(stored),
		"unicode whitespace":  "\n{\n  \"language\" : \"de\"\n}\n",
		"empty object":        `{}`,
		"channel set then id": `{"matchSummaryChannelID":"` + validID + `"}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			if _, err := applyBody(t, stored, body); err != nil {
				t.Errorf("body should be accepted: %v\n  body: %s", err, body)
			}
		})
	}
}

func TestScenario_BadBodiesAreRejectedWithFieldPaths(t *testing.T) {
	stored := loadFixture(t)
	cases := []struct {
		name   string
		body   string
		fields []string
	}{
		{"unknown language", `{"language":"tlh"}`, []string{"language"}},
		{"leaderboard too big", `{"leaderboardSize":11}`, []string{"leaderboardSize"}},
		{"leaderboard zero", `{"leaderboardSize":0}`, []string{"leaderboardSize"}},
		{"negative leaderboard min", `{"leaderboardMin":-1}`, []string{"leaderboardMin"}},
		{"summary too long", `{"deleteGameSummary":61}`, []string{"deleteGameSummary"}},
		{"summary too negative", `{"deleteGameSummary":-2}`, []string{"deleteGameSummary"}},
		{"bad map", `{"mapVersion":"DETAILED"}`, []string{"mapVersion"}},
		{"bad room code", `{"displayRoomCode":"hidden"}`, []string{"displayRoomCode"}},
		{"bad admin", `{"adminIDs":["123"]}`, []string{"adminIDs[0]"}},
		{"admin mention", `{"adminIDs":["<@123456789012345678>"]}`, []string{"adminIDs[0]"}},
		{"duplicate admin", `{"adminIDs":["` + validID + `","` + validID + `"]}`, []string{"adminIDs[1]"}},
		{"bad role among good", `{"permissionRoleIDs":["` + validID + `","role","` + validID2 + `"]}`, []string{"permissionRoleIDs[1]"}},
		{"bad channel", `{"matchSummaryChannelID":"#general"}`, []string{"matchSummaryChannelID"}},
		{"delay too long", `{"delays":{"delays":{"LOBBY":{"LOBBY":0,"TASKS":11,"DISCUSSION":0}}}}`, []string{"delays.delays.LOBBY.TASKS"}},
		{"delay negative", `{"delays":{"delays":{"LOBBY":{"LOBBY":0,"TASKS":-1,"DISCUSSION":0}}}}`, []string{"delays.delays.LOBBY.TASKS"}},
		{"delay to unknown phase", `{"delays":{"delays":{"LOBBY":{"LOBBY":0,"TASKS":1,"DISCUSSION":0,"MENU":1}}}}`, []string{"delays.delays.LOBBY.MENU"}},
		{"partial delay row", `{"delays":{"delays":{"LOBBY":{"TASKS":1}}}}`, []string{"delays.delays.LOBBY.LOBBY", "delays.delays.LOBBY.DISCUSSION"}},
		{"empty delay row", `{"delays":{"delays":{"LOBBY":{}}}}`, []string{"delays.delays.LOBBY.LOBBY", "delays.delays.LOBBY.TASKS", "delays.delays.LOBBY.DISCUSSION"}},
		{"delay from lowercase phase", `{"delays":{"delays":{"lobby":{"TASKS":1}}}}`, []string{"delays.delays.lobby"}},
		{"rule for unknown phase", `{"voiceRules":{"MuteRules":{"GAMEOVER":{"alive":true,"dead":true}}}}`, []string{"voiceRules.MuteRules.GAMEOVER"}},
		{"rule for unknown state", `{"voiceRules":{"MuteRules":{"TASKS":{"alive":true,"dead":false,"ghost":true}}}}`, []string{"voiceRules.MuteRules.TASKS.ghost"}},
		{"partial rule row", `{"voiceRules":{"MuteRules":{"TASKS":{"dead":true}}}}`, []string{"voiceRules.MuteRules.TASKS.alive"}},
		{"empty rule row", `{"voiceRules":{"DeafRules":{"LOBBY":{}}}}`, []string{"voiceRules.DeafRules.LOBBY.alive", "voiceRules.DeafRules.LOBBY.dead"}},
		{"several at once", `{"language":"tlh","leaderboardSize":0,"adminIDs":["a","b"]}`,
			[]string{"language", "leaderboardSize", "adminIDs[0]", "adminIDs[1]"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := applyBody(t, stored, tc.body)
			if err == nil {
				t.Fatalf("body should be rejected: %s", tc.body)
			}
			assertOnlyFields(t, err, tc.fields...)
		})
	}
}

// Decode-level rejections (shape, type, unknown key) never reach Validate and never partially apply: the caller
// gets a plain error and must discard the working copy.
func TestScenario_DecodeErrorsAreNotValidationErrors(t *testing.T) {
	stored := loadFixture(t)
	for name, body := range map[string]string{
		"typo":          `{"leaderboardsize":5}`,
		"type":          `{"leaderboardSize":"5"}`,
		"null":          `null`,
		"null in array": `{"adminIDs":[null]}`,
		"null nested":   `{"delays":{"delays":{"TASKS":null}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := applyBody(t, stored, body)
			if err == nil {
				t.Fatal("expected error")
			}
			var verrs ValidationErrors
			if errors.As(err, &verrs) {
				t.Errorf("decode failure should not be a ValidationErrors: %v", err)
			}
		})
	}
}

// A guild whose stored settings predate several fields can still be updated field-by-field: the gaps are filled
// with defaults before the body is applied, so untouched legacy zeros never fail validation.
func TestScenario_LegacyStoredSettingsAcceptPartialUpdate(t *testing.T) {
	legacy := []byte(`{"language":"en","adminIDs":[],"permissionRoleIDs":[]}`)
	sett, err := applyBody(t, legacy, `{"muteSpectator":true}`)
	if err != nil {
		t.Fatalf("partial update over legacy settings should be accepted: %v", err)
	}
	if !sett.MuteSpectator {
		t.Error("update not applied")
	}
	if sett.GetLeaderboardSize() != DefaultLeaderboardSize || sett.GetDelay(game.LOBBY, game.TASKS) != 7 {
		t.Error("legacy gaps should hold the defaults")
	}
}

// The validated document must be exactly what the bot would write itself: the JSON shape matches the fixture,
// so the stored blob stays readable by the existing decode path.
func TestScenario_ValidatedDocumentKeepsStorageShape(t *testing.T) {
	sett, err := applyBody(t, loadFixture(t), `{"leaderboardSize":4}`)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(sett)
	if err != nil {
		t.Fatal(err)
	}
	var fixture, got interface{}
	if err := json.Unmarshal(loadFixture(t), &fixture); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if want, have := jsonShape(fixture), jsonShape(got); !reflect.DeepEqual(want, have) {
		t.Errorf("shape drift after a validated write:\n want %v\n have %v", want, have)
	}
}
