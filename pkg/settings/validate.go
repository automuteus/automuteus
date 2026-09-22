package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/game"
)

// Limits shared by every path that writes guild settings (slash commands, the
// HTTP API). bot/setting mirrors these into the Discord option constraints so
// the two never drift apart.
const (
	MinDelaySeconds = 0
	MaxDelaySeconds = 10

	MinLeaderboardSize = 1
	MaxLeaderboardSize = 10

	MinLeaderboardMin = 1
	MaxLeaderboardMin = 100

	// -1 means never delete the match summary.
	MinDeleteGameSummaryMinutes = -1
	MaxDeleteGameSummaryMinutes = 60

	MaxAdminUserIDs      = 100
	MaxPermissionRoleIDs = 100

	MapVersionSimple   = "simple"
	MapVersionDetailed = "detailed"

	DisplayRoomCodeAlways  = "always"
	DisplayRoomCodeSpoiler = "spoiler"
	DisplayRoomCodeNever   = "never"
)

// rulePhases are the only phases that have voice rules and delays. MENU exists
// as a Phase but has never had rules; the getters would silently ignore any
// other key, which is exactly the kind of mistake validation must surface.
var rulePhases = []game.PhaseNameString{
	game.PhaseNames[game.LOBBY],
	game.PhaseNames[game.TASKS],
	game.PhaseNames[game.DISCUSS],
}

var aliveStates = []string{"alive", "dead"}

// FieldError describes one invalid value. Field is a JSON path into the
// settings document (for example "delays.delays.LOBBY.TASKS" or "adminIDs[2]")
// so an API client can point at the offending input.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e FieldError) Error() string {
	return e.Field + ": " + e.Message
}

// ValidationErrors is every problem found in one pass over the settings, so a
// client can fix them all at once instead of one per round trip.
type ValidationErrors []FieldError

func (v ValidationErrors) Error() string {
	msgs := make([]string, len(v))
	for i, e := range v {
		msgs[i] = e.Error()
	}
	return "invalid guild settings: " + strings.Join(msgs, "; ")
}

// Fields returns the offending JSON paths in document order, for assertions
// and logs.
func (v ValidationErrors) Fields() []string {
	fields := make([]string, len(v))
	for i, e := range v {
		fields[i] = e.Field
	}
	return fields
}

func (v *ValidationErrors) add(field, format string, args ...interface{}) {
	*v = append(*v, FieldError{Field: field, Message: fmt.Sprintf(format, args...)})
}

// Validate checks every field of a complete settings document against the
// same constraints the slash commands enforce. languages is the set of
// installed locale tags (locale.GetLanguages()); it is a parameter so callers
// that never load the locale bundle, such as the API process, decide what is
// accepted rather than silently accepting nothing.
//
// The receiver is expected to be complete: missing voice rules or delays are
// errors, not defaults. Call FillDefaults first on documents that may predate
// a field (legacy stored blobs) so their gaps become the same defaults the
// getters would have returned.
//
// It returns nil when the settings are valid; otherwise a ValidationErrors.
func (gs *GuildSettings) Validate(languages map[string]string) error {
	if gs == nil {
		return ValidationErrors{{Field: "", Message: "settings document is missing"}}
	}
	var errs ValidationErrors

	validateIDList(&errs, "adminIDs", gs.AdminUserIDs, MaxAdminUserIDs)
	validateIDList(&errs, "permissionRoleIDs", gs.PermissionRoleIDs, MaxPermissionRoleIDs)

	if gs.Language == "" {
		errs.add("language", "must not be empty")
	} else if _, ok := languages[gs.Language]; !ok {
		errs.add("language", "%q is not an installed language (known: %s)", gs.Language, joinKeys(languages))
	}

	validateRules(&errs, "voiceRules.MuteRules", gs.VoiceRules.MuteRules)
	validateRules(&errs, "voiceRules.DeafRules", gs.VoiceRules.DeafRules)

	switch gs.MapVersion {
	case MapVersionSimple, MapVersionDetailed:
	default:
		errs.add("mapVersion", "%q must be %q or %q", gs.MapVersion, MapVersionSimple, MapVersionDetailed)
	}

	validateDelays(&errs, "delays.delays", gs.Delays.Delays)

	validateRange(&errs, "deleteGameSummary", gs.DeleteGameSummaryMinutes, MinDeleteGameSummaryMinutes, MaxDeleteGameSummaryMinutes)

	if gs.MatchSummaryChannelID != "" {
		if err := discord.ValidateSnowflake(gs.MatchSummaryChannelID); err != nil {
			errs.add("matchSummaryChannelID", "%q is not a Discord channel ID: %v", gs.MatchSummaryChannelID, err)
		}
	}

	validateRange(&errs, "leaderboardSize", gs.LeaderboardSize, MinLeaderboardSize, MaxLeaderboardSize)
	validateRange(&errs, "leaderboardMin", gs.LeaderboardMin, MinLeaderboardMin, MaxLeaderboardMin)

	switch gs.DisplayRoomCode {
	case DisplayRoomCodeAlways, DisplayRoomCodeSpoiler, DisplayRoomCodeNever:
	default:
		errs.add("displayRoomCode", "%q must be one of %q, %q, %q", gs.DisplayRoomCode,
			DisplayRoomCodeAlways, DisplayRoomCodeSpoiler, DisplayRoomCodeNever)
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// FillDefaults replaces "unset" zero values with the built-in defaults, the
// same substitutions the getters make at read time. It exists so a document
// read from storage that predates a field validates the way it behaves. It
// never touches a field that already holds a value, valid or not, so invalid
// input still fails Validate.
func (gs *GuildSettings) FillDefaults() {
	defaults := MakeGuildSettings()
	if gs.AdminUserIDs == nil {
		gs.AdminUserIDs = []string{}
	}
	if gs.PermissionRoleIDs == nil {
		gs.PermissionRoleIDs = []string{}
	}
	if gs.Language == "" {
		gs.Language = defaults.Language
	}
	if gs.VoiceRules.MuteRules == nil {
		gs.VoiceRules.MuteRules = defaults.VoiceRules.MuteRules
	}
	if gs.VoiceRules.DeafRules == nil {
		gs.VoiceRules.DeafRules = defaults.VoiceRules.DeafRules
	}
	if gs.MapVersion == "" {
		gs.MapVersion = defaults.MapVersion
	}
	if gs.Delays.Delays == nil {
		gs.Delays.Delays = defaults.Delays.Delays
	}
	if gs.LeaderboardSize < 1 {
		gs.LeaderboardSize = defaults.LeaderboardSize
	}
	if gs.LeaderboardMin < 1 {
		gs.LeaderboardMin = defaults.LeaderboardMin
	}
	if gs.DisplayRoomCode == "" {
		gs.DisplayRoomCode = defaults.DisplayRoomCode
	}
}

// UnmarshalStrict decodes a settings JSON document over dst, rejecting what
// encoding/json would otherwise let through silently:
//
//   - unknown keys, matched exactly: encoding/json matches keys
//     case-insensitively, so "leaderboardsize" would update leaderboardSize
//     and "Language" would update language; a typo must fail, not half-work;
//   - null anywhere in the document: Unmarshal treats null as "leave the
//     existing value", and when it reuses a slice's backing array a null
//     element even resurrects the previous entry (["x", null] keeps the old
//     second admin). No setting has a legitimate null value;
//   - a bare non-object document (null, array, scalar) and trailing content.
//
// Keys present in data overwrite the corresponding fields of dst; keys absent
// from data leave dst untouched, so decoding over a copy of the stored
// settings yields a merged document to validate as a whole. That merge is
// shallow for the rule and delay tables: a phase row that is present replaces
// the stored row entirely, so a partial row fails Validate with the missing
// siblings named rather than silently dropping them.
//
// Type errors ("3" for an int, 3.5 for an int, an object for an array) are
// reported by encoding/json and returned unchanged.
func UnmarshalStrict(data []byte, dst *GuildSettings) error {
	if dst == nil {
		return errors.New("nil destination")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return errors.New("settings document is empty")
	}
	if trimmed[0] != '{' {
		return errors.New("settings document must be a JSON object")
	}
	if err := rejectNulls(trimmed); err != nil {
		return err
	}
	if err := checkExactKeys(trimmed, reflect.TypeOf(dst).Elem(), ""); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("unexpected content after settings document")
	}
	return nil
}

// rejectNulls fails on the first JSON null token anywhere in data. Syntax
// errors are left for the real decode so they get encoding/json's messages.
func rejectNulls(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return nil
		}
		if tok == nil {
			return errors.New("null is not a valid settings value")
		}
	}
}

// checkExactKeys walks the JSON object in raw against the struct type typ and
// fails on any key that is not exactly one of the struct's JSON field names.
// Struct-typed fields are checked recursively; map-typed fields (the rule and
// delay tables) accept any key here and are checked by Validate. Malformed
// JSON is left for the real decode.
func checkExactKeys(raw []byte, typ reflect.Type, path string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil
	}
	fields := make(map[string]reflect.Type, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" {
			continue // unexported (the lock)
		}
		name := f.Name
		if tag := f.Tag.Get("json"); tag != "" {
			if tag == "-" {
				continue
			}
			if idx := strings.IndexByte(tag, ','); idx >= 0 {
				tag = tag[:idx]
			}
			if tag != "" {
				name = tag
			}
		}
		fields[name] = f.Type
	}
	for key, child := range object {
		childType, ok := fields[key]
		if !ok {
			return fmt.Errorf("unknown settings field %q", path+key)
		}
		if childType.Kind() == reflect.Struct {
			if err := checkExactKeys(child, childType, path+key+"."); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateIDList(errs *ValidationErrors, field string, ids []string, max int) {
	if len(ids) > max {
		errs.add(field, "has %d entries, at most %d are allowed", len(ids), max)
	}
	seen := make(map[string]int, len(ids))
	for i, id := range ids {
		path := fmt.Sprintf("%s[%d]", field, i)
		if err := discord.ValidateSnowflake(id); err != nil {
			errs.add(path, "%q is not a Discord ID: %v", id, err)
			continue
		}
		if first, dup := seen[id]; dup {
			errs.add(path, "%q duplicates %s[%d]", id, field, first)
			continue
		}
		seen[id] = i
	}
}

func validateRules(errs *ValidationErrors, field string, rules map[game.PhaseNameString]map[string]bool) {
	if rules == nil {
		errs.add(field, "is missing")
		return
	}
	for _, phase := range rulePhases {
		states, ok := rules[phase]
		if !ok {
			errs.add(field+"."+string(phase), "is missing")
			continue
		}
		for _, alive := range aliveStates {
			if _, ok := states[alive]; !ok {
				errs.add(field+"."+string(phase)+"."+alive, "is missing")
			}
		}
		for alive := range states {
			if !containsString(aliveStates, alive) {
				errs.add(field+"."+string(phase)+"."+alive, "is not a player state (expected %q or %q)", aliveStates[0], aliveStates[1])
			}
		}
	}
	for phase := range rules {
		if !isRulePhase(phase) {
			errs.add(field+"."+string(phase), "is not a game phase (expected %s)", joinPhases())
		}
	}
}

func validateDelays(errs *ValidationErrors, field string, delays map[game.PhaseNameString]map[game.PhaseNameString]int) {
	if delays == nil {
		errs.add(field, "is missing")
		return
	}
	for _, from := range rulePhases {
		dests, ok := delays[from]
		if !ok {
			errs.add(field+"."+string(from), "is missing")
			continue
		}
		for _, to := range rulePhases {
			path := field + "." + string(from) + "." + string(to)
			delay, ok := dests[to]
			if !ok {
				errs.add(path, "is missing")
				continue
			}
			validateRange(errs, path, delay, MinDelaySeconds, MaxDelaySeconds)
		}
		for to := range dests {
			if !isRulePhase(to) {
				errs.add(field+"."+string(from)+"."+string(to), "is not a game phase (expected %s)", joinPhases())
			}
		}
	}
	for from := range delays {
		if !isRulePhase(from) {
			errs.add(field+"."+string(from), "is not a game phase (expected %s)", joinPhases())
		}
	}
}

func validateRange(errs *ValidationErrors, field string, value, min, max int) {
	if value < min || value > max {
		errs.add(field, "%d is out of range [%d, %d]", value, min, max)
	}
}

func isRulePhase(phase game.PhaseNameString) bool {
	for _, p := range rulePhases {
		if p == phase {
			return true
		}
	}
	return false
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func joinPhases() string {
	names := make([]string, len(rulePhases))
	for i, p := range rulePhases {
		names[i] = string(p)
	}
	return strings.Join(names, ", ")
}

func joinKeys(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
