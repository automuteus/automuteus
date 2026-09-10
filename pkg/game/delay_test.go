package game

import "testing"

// TestMakeDefaultDelays pins the default number of seconds the bot waits before applying mute/deafen changes on
// each phase transition. These values are tuned against the in-game transition animations; changing them is
// user-visible.
func TestMakeDefaultDelays(t *testing.T) {
	delays := MakeDefaultDelays()

	tests := []struct {
		origin Phase
		dest   Phase
		want   int
	}{
		{LOBBY, LOBBY, 0},
		{LOBBY, TASKS, 7},
		{LOBBY, DISCUSS, 0},
		{TASKS, LOBBY, 1},
		{TASKS, TASKS, 0},
		{TASKS, DISCUSS, 0},
		{DISCUSS, LOBBY, 6},
		{DISCUSS, TASKS, 7},
		{DISCUSS, DISCUSS, 0},
	}

	for _, tt := range tests {
		if got := delays.GetDelay(tt.origin, tt.dest); got != tt.want {
			t.Errorf("%s->%s: got delay %d, want %d", PhaseNames[tt.origin], PhaseNames[tt.dest], got, tt.want)
		}
	}
}

// Transitions involving phases with no configured delay (MENU, GAMEOVER) must be zero rather than panic.
// Note that processTransition rewrites GAMEOVER to LOBBY before looking up the delay.
func TestGameDelays_PhasesWithoutDelays(t *testing.T) {
	delays := MakeDefaultDelays()

	for _, tt := range [][2]Phase{{MENU, TASKS}, {TASKS, MENU}, {GAMEOVER, LOBBY}, {DISCUSS, GAMEOVER}, {UNINITIALIZED, LOBBY}} {
		if got := delays.GetDelay(tt[0], tt[1]); got != 0 {
			t.Errorf("%d->%d: got delay %d, want 0", tt[0], tt[1], got)
		}
	}

	empty := GameDelays{}
	if got := empty.GetDelay(LOBBY, TASKS); got != 0 {
		t.Errorf("empty delays: got %d, want 0", got)
	}
}
