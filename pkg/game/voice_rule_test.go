package game

import "testing"

// TestMakeMuteAndDeafenRules_Defaults pins the default mute/deafen behavior for every phase and aliveness
// combination. These defaults are what the vast majority of guilds run with, so any change here is user-visible.
func TestMakeMuteAndDeafenRules_Defaults(t *testing.T) {
	rules := MakeMuteAndDeafenRules()

	tests := []struct {
		phase    Phase
		alive    bool
		wantMute bool
		wantDeaf bool
	}{
		{LOBBY, true, false, false},
		{LOBBY, false, false, false},
		{TASKS, true, true, true},
		{TASKS, false, false, false},
		{DISCUSS, true, false, false},
		{DISCUSS, false, true, false},
	}

	for _, tt := range tests {
		gotMute, gotDeaf := rules.GetVoiceState(tt.alive, true, tt.phase)
		if gotMute != tt.wantMute || gotDeaf != tt.wantDeaf {
			t.Errorf("phase=%s alive=%v: got mute=%v deaf=%v, want mute=%v deaf=%v",
				PhaseNames[tt.phase], tt.alive, gotMute, gotDeaf, tt.wantMute, tt.wantDeaf)
		}
	}
}

// Untracked users (not in the tracked voice channel, or not linked) must never be muted or deafened,
// regardless of how aggressive the configured rules are.
func TestVoiceRules_UntrackedNeverMutedOrDeafened(t *testing.T) {
	rules := MakeMuteAndDeafenRules()
	configured := []Phase{LOBBY, TASKS, DISCUSS}
	for _, phase := range configured {
		for _, alive := range []string{"alive", "dead"} {
			rules.MuteRules[PhaseNames[phase]][alive] = true
			rules.DeafRules[PhaseNames[phase]][alive] = true
		}
	}

	for _, phase := range configured {
		for _, alive := range []bool{true, false} {
			if mute, deaf := rules.GetVoiceState(alive, false, phase); mute || deaf {
				t.Errorf("phase=%s alive=%v: untracked user got mute=%v deaf=%v, want neither",
					PhaseNames[phase], alive, mute, deaf)
			}
		}
	}
}

// Phases with no configured rules (MENU, GAMEOVER, UNINITIALIZED) must resolve to unmuted/undeafened
// rather than panicking on the missing map entries.
func TestVoiceRules_PhasesWithoutRules(t *testing.T) {
	rules := MakeMuteAndDeafenRules()

	for _, phase := range []Phase{MENU, GAMEOVER, UNINITIALIZED} {
		for _, alive := range []bool{true, false} {
			if mute, deaf := rules.GetVoiceState(alive, true, phase); mute || deaf {
				t.Errorf("phase=%d alive=%v: got mute=%v deaf=%v, want neither", phase, alive, mute, deaf)
			}
		}
	}

	// completely empty rules (e.g. a settings blob missing voiceRules) must also be safe
	empty := VoiceRules{}
	if mute, deaf := empty.GetVoiceState(true, true, TASKS); mute || deaf {
		t.Errorf("empty rules: got mute=%v deaf=%v, want neither", mute, deaf)
	}
}

// Custom rules configured via /settings voice-rules must be honored.
func TestVoiceRules_CustomRule(t *testing.T) {
	rules := MakeMuteAndDeafenRules()
	// e.g. a guild that wants dead players muted during tasks so they can't backseat
	rules.MuteRules[PhaseNames[TASKS]]["dead"] = true
	rules.DeafRules[PhaseNames[DISCUSS]]["dead"] = true

	if mute, deaf := rules.GetVoiceState(false, true, TASKS); !mute || deaf {
		t.Errorf("TASKS dead with custom rule: got mute=%v deaf=%v, want mute=true deaf=false", mute, deaf)
	}
	if mute, deaf := rules.GetVoiceState(false, true, DISCUSS); !mute || !deaf {
		t.Errorf("DISCUSS dead with custom rule: got mute=%v deaf=%v, want mute=true deaf=true", mute, deaf)
	}
	// alive players are unaffected by the dead rules
	if mute, deaf := rules.GetVoiceState(true, true, DISCUSS); mute || deaf {
		t.Errorf("DISCUSS alive: got mute=%v deaf=%v, want neither", mute, deaf)
	}
}
