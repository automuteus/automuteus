package bot

import (
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/amongus"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/bwmarrin/discordgo"
)

const (
	trackedChannel = "900"
	otherChannel   = "901"
)

// voiceTestState builds a running game in the given phase with the tracked voice channel set.
func voiceTestState(phase game.Phase) *GameState {
	dgs := NewDiscordGameState("1")
	dgs.ConnectCode = "ABCDEFGH"
	dgs.Running = true
	dgs.VoiceChannel = trackedChannel
	dgs.GameData.UpdatePhase(phase)
	return dgs
}

// addLinkedUser adds a Discord user linked to an in-game player of the given aliveness, with the given current
// (believed) mute/deaf state.
func addLinkedUser(dgs *GameState, userID, playerName string, alive, currentMute, currentDeaf bool) {
	dgs.GameData.PlayerData[playerName] = amongus.PlayerData{Color: game.Red, Name: playerName, IsAlive: alive}
	dgs.UserData[userID] = UserData{
		User:         User{UserID: userID, UserName: playerName},
		ShouldBeMute: currentMute,
		ShouldBeDeaf: currentDeaf,
		InGameName:   playerName,
	}
}

// addUnlinkedUser adds a Discord user in voice who is not linked to any in-game player (e.g. a music bot).
func addUnlinkedUser(dgs *GameState, userID string, currentMute, currentDeaf bool) {
	dgs.UserData[userID] = UserData{
		User:         User{UserID: userID, UserName: "user" + userID},
		ShouldBeMute: currentMute,
		ShouldBeDeaf: currentDeaf,
		InGameName:   amongus.UnlinkedPlayerName,
	}
}

func inChannel(userID, channelID string) *discordgo.VoiceState {
	return &discordgo.VoiceState{UserID: userID, ChannelID: channelID}
}

func findChange(users []task.UserModify, userID uint64) (task.UserModify, bool) {
	for _, u := range users {
		if u.UserID == userID {
			return u, true
		}
	}
	return task.UserModify{}, false
}

func expectChange(t *testing.T, users []task.UserModify, userID uint64, mute, deaf bool) {
	t.Helper()
	got, ok := findChange(users, userID)
	if !ok {
		t.Errorf("expected a change for user %d, got none (changes: %+v)", userID, users)
		return
	}
	if got.Mute != mute || got.Deaf != deaf {
		t.Errorf("user %d: got mute=%v deaf=%v, want mute=%v deaf=%v", userID, got.Mute, got.Deaf, mute, deaf)
	}
}

func expectNoChange(t *testing.T, users []task.UserModify, userID uint64) {
	t.Helper()
	if got, ok := findChange(users, userID); ok {
		t.Errorf("expected no change for user %d, got %+v", userID, got)
	}
}

// The core happy path: the game starts, so every linked alive player in the tracked channel gets muted and
// deafened, and the recorded state is updated so a second pass is a no-op.
func TestComputeVoiceChanges_GameStartMutesAlivePlayers(t *testing.T) {
	sett := settings.MakeGuildSettings()
	dgs := voiceTestState(game.TASKS)
	addLinkedUser(dgs, "10", "Red", true, false, false)
	addLinkedUser(dgs, "11", "Blue", true, false, false)
	voice := []*discordgo.VoiceState{inChannel("10", trackedChannel), inChannel("11", trackedChannel)}

	users, priority := computeVoiceChanges(dgs, sett, voice, NoPriority)

	if len(users) != 2 || priority != 0 {
		t.Fatalf("got %d changes (%d priority), want 2 (0 priority): %+v", len(users), priority, users)
	}
	expectChange(t, users, 10, true, true)
	expectChange(t, users, 11, true, true)

	for _, id := range []string{"10", "11"} {
		if u, _ := dgs.GetUser(id); !u.ShouldBeMute || !u.ShouldBeDeaf {
			t.Errorf("user %s recorded state not updated: %+v", id, u)
		}
	}

	// idempotent: the same inputs must not produce more changes
	if again, _ := computeVoiceChanges(dgs, sett, voice, NoPriority); len(again) != 0 {
		t.Errorf("second pass produced changes: %+v", again)
	}
}

// Every phase transition for a single alive and a single dead player, using the default rules.
func TestComputeVoiceChanges_DefaultRulesPerPhase(t *testing.T) {
	tests := []struct {
		phase                game.Phase
		aliveMute, aliveDeaf bool
		deadMute, deadDeaf   bool
	}{
		{game.LOBBY, false, false, false, false},
		{game.TASKS, true, true, false, false},
		{game.DISCUSS, false, false, true, false},
	}

	for _, tt := range tests {
		sett := settings.MakeGuildSettings()
		dgs := voiceTestState(tt.phase)
		// start both players in the "opposite" state so a change is always expected if the rule differs from it
		addLinkedUser(dgs, "10", "Alive", true, !tt.aliveMute, !tt.aliveDeaf)
		addLinkedUser(dgs, "11", "Dead", false, !tt.deadMute, !tt.deadDeaf)
		voice := []*discordgo.VoiceState{inChannel("10", trackedChannel), inChannel("11", trackedChannel)}

		users, _ := computeVoiceChanges(dgs, sett, voice, NoPriority)

		expectChange(t, users, 10, tt.aliveMute, tt.aliveDeaf)
		expectChange(t, users, 11, tt.deadMute, tt.deadDeaf)
	}
}

// Players who are already in the correct state must not generate Discord API calls.
func TestComputeVoiceChanges_SkipsUsersAlreadyInCorrectState(t *testing.T) {
	sett := settings.MakeGuildSettings()
	dgs := voiceTestState(game.TASKS)
	addLinkedUser(dgs, "10", "Red", true, true, true)     // alive, already mute+deaf
	addLinkedUser(dgs, "11", "Blue", false, false, false) // dead, already free
	voice := []*discordgo.VoiceState{inChannel("10", trackedChannel), inChannel("11", trackedChannel)}

	if users, _ := computeVoiceChanges(dgs, sett, voice, NoPriority); len(users) != 0 {
		t.Errorf("expected no changes, got %+v", users)
	}
}

// Unlinked users (music bots, people just hanging out) must never be touched, even if their recorded state is
// stale, unless the guild has opted into muting spectators.
func TestComputeVoiceChanges_IgnoresUnlinkedUsers(t *testing.T) {
	sett := settings.MakeGuildSettings()
	dgs := voiceTestState(game.TASKS)
	addUnlinkedUser(dgs, "20", false, false)
	addUnlinkedUser(dgs, "21", true, true) // stale "should be muted" from some earlier game
	addLinkedUser(dgs, "10", "Red", true, false, false)
	voice := []*discordgo.VoiceState{inChannel("20", trackedChannel), inChannel("21", trackedChannel), inChannel("10", trackedChannel)}

	users, _ := computeVoiceChanges(dgs, sett, voice, NoPriority)

	expectNoChange(t, users, 20)
	expectNoChange(t, users, 21)
	expectChange(t, users, 10, true, true)
}

// A linked player who leaves the tracked channel (or voice entirely) mid-game must be released.
func TestComputeVoiceChanges_UnmutesPlayersOutsideTrackedChannel(t *testing.T) {
	sett := settings.MakeGuildSettings()
	dgs := voiceTestState(game.TASKS)
	addLinkedUser(dgs, "10", "Red", true, true, true)   // moved to another channel
	addLinkedUser(dgs, "11", "Blue", true, true, true)  // disconnected from voice (empty channel)
	addLinkedUser(dgs, "12", "Green", true, true, true) // still in the tracked channel
	voice := []*discordgo.VoiceState{inChannel("10", otherChannel), inChannel("11", ""), inChannel("12", trackedChannel)}

	users, _ := computeVoiceChanges(dgs, sett, voice, NoPriority)

	expectChange(t, users, 10, false, false)
	expectChange(t, users, 11, false, false)
	expectNoChange(t, users, 12)
}

// Users in voice that are not in the user cache are skipped; populating the cache is the caller's job.
func TestComputeVoiceChanges_SkipsUnknownUsers(t *testing.T) {
	sett := settings.MakeGuildSettings()
	dgs := voiceTestState(game.TASKS)
	voice := []*discordgo.VoiceState{inChannel("999", trackedChannel)}

	if users, _ := computeVoiceChanges(dgs, sett, voice, NoPriority); len(users) != 0 {
		t.Errorf("expected no changes for unknown user, got %+v", users)
	}
}

// With mute-spectators enabled, unlinked users in the tracked channel are treated as dead players.
func TestComputeVoiceChanges_MuteSpectators(t *testing.T) {
	sett := settings.MakeGuildSettings()
	sett.SetMuteSpectator(true)

	// during discussion, dead players (and therefore spectators) are muted
	dgs := voiceTestState(game.DISCUSS)
	addUnlinkedUser(dgs, "20", false, false)
	addUnlinkedUser(dgs, "21", false, false) // spectator in a different channel: not tracked, so untouched
	addLinkedUser(dgs, "10", "Red", true, false, false)
	voice := []*discordgo.VoiceState{inChannel("20", trackedChannel), inChannel("21", otherChannel), inChannel("10", trackedChannel)}

	users, _ := computeVoiceChanges(dgs, sett, voice, NoPriority)

	expectChange(t, users, 20, true, false)
	expectNoChange(t, users, 21)
	expectNoChange(t, users, 10)

	// back in the lobby, spectators are released again
	dgs.GameData.UpdatePhase(game.LOBBY)
	users, _ = computeVoiceChanges(dgs, sett, voice, NoPriority)
	expectChange(t, users, 20, false, false)
}

// Custom voice rules configured by the guild flow through to the issued changes.
func TestComputeVoiceChanges_HonorsCustomRules(t *testing.T) {
	sett := settings.MakeGuildSettings()
	sett.SetVoiceRule(true, game.TASKS, "dead", true) // mute the dead during tasks

	dgs := voiceTestState(game.TASKS)
	addLinkedUser(dgs, "11", "Blue", false, false, false)
	voice := []*discordgo.VoiceState{inChannel("11", trackedChannel)}

	users, _ := computeVoiceChanges(dgs, sett, voice, NoPriority)

	expectChange(t, users, 11, true, false)
}

// Entering discussion: dead players should be muted before alive players are unmuted, so that a dead player
// can't blurt something out in the window between the two batches.
func TestComputeVoiceChanges_DeadPriorityOnDiscussion(t *testing.T) {
	sett := settings.MakeGuildSettings()
	dgs := voiceTestState(game.DISCUSS)
	addLinkedUser(dgs, "10", "Red", true, true, true)
	addLinkedUser(dgs, "11", "Blue", true, true, true)
	addLinkedUser(dgs, "12", "Green", false, false, false)
	voice := []*discordgo.VoiceState{inChannel("10", trackedChannel), inChannel("11", trackedChannel), inChannel("12", trackedChannel)}

	users, priority := computeVoiceChanges(dgs, sett, voice, DeadPriority)

	if len(users) != 3 {
		t.Fatalf("got %d changes, want 3: %+v", len(users), users)
	}
	if priority != 1 {
		t.Fatalf("got %d priority requests, want 1", priority)
	}
	if users[0].UserID != 12 || !users[0].Mute {
		t.Errorf("first change should be muting the dead player, got %+v", users[0])
	}
	expectChange(t, users[1:], 10, false, false)
	expectChange(t, users[1:], 11, false, false)
}

// Leaving discussion for tasks: alive players should be muted before dead players are unmuted.
func TestComputeVoiceChanges_AlivePriorityOnTasks(t *testing.T) {
	sett := settings.MakeGuildSettings()
	dgs := voiceTestState(game.TASKS)
	addLinkedUser(dgs, "10", "Red", true, false, false)
	addLinkedUser(dgs, "11", "Blue", true, false, false)
	addLinkedUser(dgs, "12", "Green", false, true, false)
	voice := []*discordgo.VoiceState{inChannel("12", trackedChannel), inChannel("10", trackedChannel), inChannel("11", trackedChannel)}

	users, priority := computeVoiceChanges(dgs, sett, voice, AlivePriority)

	if len(users) != 3 {
		t.Fatalf("got %d changes, want 3: %+v", len(users), users)
	}
	if priority != 2 {
		t.Fatalf("got %d priority requests, want 2", priority)
	}
	expectChange(t, users[:2], 10, true, true)
	expectChange(t, users[:2], 11, true, true)
	if users[2].UserID != 12 || users[2].Mute {
		t.Errorf("last change should be unmuting the dead player, got %+v", users[2])
	}
}

// With no priority, order is preserved and nothing is counted as priority.
func TestComputeVoiceChanges_NoPriorityPreservesOrder(t *testing.T) {
	sett := settings.MakeGuildSettings()
	dgs := voiceTestState(game.TASKS)
	addLinkedUser(dgs, "10", "Red", true, false, false)
	addLinkedUser(dgs, "11", "Blue", false, true, true)
	addLinkedUser(dgs, "12", "Green", true, false, false)
	voice := []*discordgo.VoiceState{inChannel("10", trackedChannel), inChannel("11", trackedChannel), inChannel("12", trackedChannel)}

	users, priority := computeVoiceChanges(dgs, sett, voice, NoPriority)

	if priority != 0 {
		t.Errorf("got %d priority requests, want 0", priority)
	}
	if len(users) != 3 || users[0].UserID != 10 || users[1].UserID != 11 || users[2].UserID != 12 {
		t.Errorf("order not preserved: %+v", users)
	}
}
