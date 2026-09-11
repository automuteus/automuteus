package settings

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/game"
)

const fixturePath = "testdata/guild_settings_v8.json"

// jsonShape reduces decoded JSON to its structure: objects become a map of key -> child shape, arrays become "[]",
// and scalars become ".". Comparing shapes catches renamed, added, or removed fields without caring about values.
func jsonShape(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, child := range val {
			out[k] = jsonShape(child)
		}
		return out
	case []interface{}:
		return "[]"
	default:
		return "."
	}
}

func loadFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	return data
}

// TestGuildSettings_DecodeV8Fixture decodes a settings blob exactly the way the storage package's legacy Redis reader does (into a
// zero-value struct, not MakeGuildSettings) and checks every field made it through. If this fails after a struct
// change, existing guilds' settings in Redis would be silently altered on the next read.
func TestGuildSettings_DecodeV8Fixture(t *testing.T) {
	var sett GuildSettings
	if err := json.Unmarshal(loadFixture(t), &sett); err != nil {
		t.Fatalf("failed to decode fixture: %v", err)
	}

	if got := sett.GetAdminUserIDs(); !reflect.DeepEqual(got, []string{"123456789012345678"}) {
		t.Errorf("AdminUserIDs = %v", got)
	}
	if got := sett.GetPermissionRoleIDs(); !reflect.DeepEqual(got, []string{"234567890123456789", "345678901234567890"}) {
		t.Errorf("PermissionRoleIDs = %v", got)
	}
	if got := sett.GetLanguage(); got != "de" {
		t.Errorf("Language = %q", got)
	}
	if !sett.GetMapDetailed() {
		t.Error("MapVersion should decode as detailed")
	}
	if got := sett.GetDeleteGameSummaryMinutes(); got != 5 {
		t.Errorf("DeleteGameSummaryMinutes = %d", got)
	}
	if !sett.GetUnmuteDeadDuringTasks() {
		t.Error("UnmuteDeadDuringTasks should be true")
	}
	if !sett.GetAutoRefresh() {
		t.Error("AutoRefresh should be true")
	}
	if got := sett.GetMatchSummaryChannelID(); got != "456789012345678901" {
		t.Errorf("MatchSummaryChannelID = %q", got)
	}
	if sett.GetLeaderboardMention() {
		t.Error("LeaderboardMention should be false")
	}
	if got := sett.GetLeaderboardSize(); got != 5 {
		t.Errorf("LeaderboardSize = %d", got)
	}
	if got := sett.GetLeaderboardMin(); got != 2 {
		t.Errorf("LeaderboardMin = %d", got)
	}
	if !sett.GetMuteSpectator() {
		t.Error("MuteSpectator should be true")
	}
	if got := sett.GetDisplayRoomCode(); got != "spoiler" {
		t.Errorf("DisplayRoomCode = %q", got)
	}

	// the fixture deliberately deviates from the defaults in two places to prove the nested maps are read
	if got := sett.GetDelay(game.LOBBY, game.TASKS); got != 3 {
		t.Errorf("LOBBY->TASKS delay = %d, want 3 (custom value from fixture)", got)
	}
	if mute, deaf := sett.GetVoiceState(false, true, game.DISCUSS); !mute || !deaf {
		t.Errorf("DISCUSS dead: got mute=%v deaf=%v, want both true (custom rule from fixture)", mute, deaf)
	}
	if mute, deaf := sett.GetVoiceState(true, true, game.TASKS); !mute || !deaf {
		t.Errorf("TASKS alive: got mute=%v deaf=%v, want both true", mute, deaf)
	}
}

// TestGuildSettings_JSONShapeMatchesFixture fails when a field is added to, removed from, or renamed on
// GuildSettings (or its nested rule/delay types) without updating the fixture. That is deliberate: it forces a
// conscious decision about what happens to settings already stored in Redis for existing guilds.
func TestGuildSettings_JSONShapeMatchesFixture(t *testing.T) {
	var fixture interface{}
	if err := json.Unmarshal(loadFixture(t), &fixture); err != nil {
		t.Fatalf("failed to decode fixture: %v", err)
	}

	encoded, err := json.Marshal(MakeGuildSettings())
	if err != nil {
		t.Fatalf("failed to encode default settings: %v", err)
	}
	var current interface{}
	if err := json.Unmarshal(encoded, &current); err != nil {
		t.Fatalf("failed to decode encoded settings: %v", err)
	}

	if want, got := jsonShape(fixture), jsonShape(current); !reflect.DeepEqual(want, got) {
		t.Errorf("GuildSettings JSON shape changed.\n  fixture: %v\n  current: %v\nUpdate %s if this change is intentional, and consider what existing guilds' stored settings will decode as.",
			want, got, fixturePath)
	}
}

// A decode -> encode -> decode cycle must be lossless, since settings are rewritten to Redis on every change.
func TestGuildSettings_RoundTrip(t *testing.T) {
	var sett GuildSettings
	if err := json.Unmarshal(loadFixture(t), &sett); err != nil {
		t.Fatalf("failed to decode fixture: %v", err)
	}
	encoded, err := json.MarshalIndent(&sett, "", "  ")
	if err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	var want, got map[string]interface{}
	if err := json.Unmarshal(loadFixture(t), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("round trip altered the settings.\n  want: %v\n  got:  %v", want, got)
	}
}

// Guilds created before a field existed have blobs missing that key. Decoding into a zero struct leaves those
// fields at their zero value, so the getters must paper over that rather than the callers.
func TestGuildSettings_DecodeLegacyBlobMissingFields(t *testing.T) {
	var sett GuildSettings
	if err := json.Unmarshal([]byte(`{"language":"en","adminIDs":[]}`), &sett); err != nil {
		t.Fatalf("failed to decode legacy blob: %v", err)
	}

	if got := sett.GetLeaderboardSize(); got != DefaultLeaderboardSize {
		t.Errorf("LeaderboardSize = %d, want default %d", got, DefaultLeaderboardSize)
	}
	if got := sett.GetLeaderboardMin(); got != DefaultLeaderboardMin {
		t.Errorf("LeaderboardMin = %d, want default %d", got, DefaultLeaderboardMin)
	}
	if got := sett.GetDisplayRoomCode(); got != "always" {
		t.Errorf("DisplayRoomCode = %q, want \"always\"", got)
	}
	if sett.GetMapDetailed() {
		t.Error("missing mapVersion should not be detailed")
	}
	// missing voiceRules/delays decode to nil maps; lookups must not panic
	if mute, deaf := sett.GetVoiceState(true, true, game.TASKS); mute || deaf {
		t.Errorf("missing voice rules: got mute=%v deaf=%v, want neither", mute, deaf)
	}
	if got := sett.GetDelay(game.LOBBY, game.TASKS); got != 0 {
		t.Errorf("missing delays: got %d, want 0", got)
	}
}

func TestGuildSettings_SetAndGetVoiceRule(t *testing.T) {
	sett := MakeGuildSettings()

	if mute, deaf := sett.GetVoiceState(false, true, game.TASKS); mute || deaf {
		t.Fatalf("default TASKS dead: got mute=%v deaf=%v, want neither", mute, deaf)
	}
	sett.SetVoiceRule(true, game.TASKS, "dead", true)
	if !sett.GetVoiceRule(true, game.TASKS, "dead") {
		t.Error("GetVoiceRule did not reflect SetVoiceRule for mute")
	}
	if mute, deaf := sett.GetVoiceState(false, true, game.TASKS); !mute || deaf {
		t.Errorf("TASKS dead after SetVoiceRule: got mute=%v deaf=%v, want mute only", mute, deaf)
	}

	sett.SetVoiceRule(false, game.TASKS, "dead", true)
	if !sett.GetVoiceRule(false, game.TASKS, "dead") {
		t.Error("GetVoiceRule did not reflect SetVoiceRule for deaf")
	}
	if mute, deaf := sett.GetVoiceState(false, true, game.TASKS); !mute || !deaf {
		t.Errorf("TASKS dead after both rules: got mute=%v deaf=%v, want both", mute, deaf)
	}
}

func TestGuildSettings_SetAndGetDelay(t *testing.T) {
	sett := MakeGuildSettings()

	if got := sett.GetDelay(game.DISCUSS, game.TASKS); got != 7 {
		t.Fatalf("default DISCUSS->TASKS delay = %d, want 7", got)
	}
	sett.SetDelay(game.DISCUSS, game.TASKS, 2)
	if got := sett.GetDelay(game.DISCUSS, game.TASKS); got != 2 {
		t.Errorf("DISCUSS->TASKS after SetDelay = %d, want 2", got)
	}
	// the reverse direction is independent
	if got := sett.GetDelay(game.TASKS, game.DISCUSS); got != 0 {
		t.Errorf("TASKS->DISCUSS should be unaffected, got %d", got)
	}
}
