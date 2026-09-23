package settings

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/game"
)

func TestPremiumSnapshot_UnchangedDocumentReportsNothing(t *testing.T) {
	sett := MakeGuildSettings()
	snap := sett.PremiumSnapshot()
	if got := snap.Changed(sett); got != nil {
		t.Errorf("no change should report nothing, got %v", got)
	}
	// Echoing every premium value back unchanged through the strict decoder is not a change either.
	encoded, _ := json.Marshal(sett)
	if err := UnmarshalStrict(encoded, sett); err != nil {
		t.Fatal(err)
	}
	if got := snap.Changed(sett); got != nil {
		t.Errorf("round trip should report nothing, got %v", got)
	}
}

// Every premium-only setting is detected individually, and nothing else is.
func TestPremiumSnapshot_DetectsEachPremiumField(t *testing.T) {
	cases := map[string]func(*GuildSettings){
		"deleteGameSummary":     func(s *GuildSettings) { s.SetDeleteGameSummaryMinutes(-1) },
		"matchSummaryChannelID": func(s *GuildSettings) { s.SetMatchSummaryChannelID("123456789012345678") },
		"autoRefresh":           func(s *GuildSettings) { s.SetAutoRefresh(true) },
		"leaderboardMention":    func(s *GuildSettings) { s.SetLeaderboardMention(false) },
		"leaderboardSize":       func(s *GuildSettings) { s.SetLeaderboardSize(5) },
		"leaderboardMin":        func(s *GuildSettings) { s.SetLeaderboardMin(5) },
		"muteSpectator":         func(s *GuildSettings) { s.SetMuteSpectator(true) },
		"displayRoomCode":       func(s *GuildSettings) { s.SetDisplayRoomCode(DisplayRoomCodeNever) },
	}
	for field, mutate := range cases {
		t.Run(field, func(t *testing.T) {
			sett := MakeGuildSettings()
			snap := sett.PremiumSnapshot()
			mutate(sett)
			if got := snap.Changed(sett); !reflect.DeepEqual(got, []string{field}) {
				t.Errorf("Changed = %v, want [%s]", got, field)
			}
		})
	}
}

// The settings every guild may change never show up as premium changes.
func TestPremiumSnapshot_IgnoresFreeFields(t *testing.T) {
	sett := MakeGuildSettings()
	snap := sett.PremiumSnapshot()
	sett.SetLanguage("de")
	sett.SetAdminUserIDs([]string{"123456789012345678"})
	sett.SetPermissionRoleIDs([]string{"223456789012345678"})
	sett.SetUnmuteDeadDuringTasks(true)
	sett.SetMapDetailed(true)
	sett.SetDelay(game.LOBBY, game.TASKS, 1)
	sett.SetVoiceRule(true, game.LOBBY, "alive", true)
	if got := snap.Changed(sett); got != nil {
		t.Errorf("free fields reported as premium changes: %v", got)
	}
}

func TestPremiumSnapshot_ReportsEveryChangedField(t *testing.T) {
	sett := MakeGuildSettings()
	snap := sett.PremiumSnapshot()
	sett.SetAutoRefresh(true)
	sett.SetLeaderboardSize(9)
	sett.SetDisplayRoomCode(DisplayRoomCodeSpoiler)
	got := snap.Changed(sett)
	sort.Strings(got)
	want := []string{"autoRefresh", "displayRoomCode", "leaderboardSize"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Changed = %v, want %v", got, want)
	}
}

// Setting a premium field back to the value it already holds is not a change: a free guild may "set" the default.
func TestPremiumSnapshot_SameValueIsNotAChange(t *testing.T) {
	sett := MakeGuildSettings()
	snap := sett.PremiumSnapshot()
	sett.SetLeaderboardSize(DefaultLeaderboardSize)
	sett.SetDisplayRoomCode(DisplayRoomCodeAlways)
	if got := snap.Changed(sett); got != nil {
		t.Errorf("same-value writes reported: %v", got)
	}
}
