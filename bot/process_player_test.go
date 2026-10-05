package bot

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/amongus"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/bwmarrin/discordgo"
)

// These tests pin processPlayer's current behavior from the outside: a PlayerJob goes in through processJob, and the
// tests assert only on the stored state, the voice requests, and the status message activity that result. They are
// meant to keep passing unchanged while processPlayer is restructured to do its side effects after releasing the game
// state lock; the one exception is the stale-refresh test, which pins a known bug.

const sentinelUserID = "99"

type playerScenario struct {
	t     *testing.T
	bot   *Bot
	deps  *testDeps
	gsr   GameStateRequest
	msgID string
}

// newPlayerScenario seeds a running game in the given phase. Its status message ID is unique to the test, since
// pending status edits live in a package-level map.
//
// The game includes a sentinel: a linked, alive player in the tracked voice channel whose believed voice state is
// wrong for the phase. handleTrackedMembers always issues a change for the sentinel, so a request containing it shows
// that processPlayer asked for tracked members to be handled.
//
// Status edits are parked: the deferred edit worker blocks on its delay until the test ends, so whether an edit was
// requested can be read deterministically from DeferredEdits.
func newPlayerScenario(t *testing.T, phase game.Phase, seed func(dgs *GameState)) *playerScenario {
	t.Helper()
	bot, deps := newTestBot(t)
	dgs := runningGame(deps, phase, inChannel(sentinelUserID, trackedChannel))
	dgs.GameStateMsg.MessageID = "status-" + t.Name()

	alive := true
	mute, deaf := deps.settings.GetVoiceState(alive, true, phase)
	addLinkedUserWithColor(dgs, sentinelUserID, "sentinel", game.Coral, alive, !mute, deaf)
	if seed != nil {
		seed(dgs)
	}
	deps.store.put(dgs)

	release := make(chan struct{})
	var releaseOnce sync.Once
	bot.sleep = func(d time.Duration) {
		if d == time.Second*DeferredEditSeconds {
			<-release
			return
		}
		deps.recordSleep(d)
	}
	s := &playerScenario{
		t:     t,
		bot:   bot,
		deps:  deps,
		gsr:   GameStateRequest{GuildID: scenarioGuild, ConnectCode: scenarioConnectCode},
		msgID: dgs.GameStateMsg.MessageID,
	}
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		eventually(t, "parked status edit to drain", func() bool { return !s.editPending() })
	})
	return s
}

// send runs the player event through processJob and returns the user ID it correlated with the player.
func (s *playerScenario) send(player game.Player) string {
	s.t.Helper()
	payload, err := json.Marshal(player)
	if err != nil {
		s.t.Fatal(err)
	}
	return s.bot.processJob(task.Job{JobType: task.PlayerJob, Payload: string(payload)}, s.deps.settings, premium.FreeTier, s.gsr)
}

func (s *playerScenario) state() *GameState {
	return s.deps.store.get()
}

func (s *playerScenario) editPending() bool {
	DeferredEditsLock.Lock()
	defer DeferredEditsLock.Unlock()
	_, ok := DeferredEdits[s.msgID]
	return ok
}

// handledTracked reports whether handleTrackedMembers ran, by whether a voice request touched the sentinel.
func (s *playerScenario) handledTracked() bool {
	for _, req := range s.deps.voice.all() {
		if _, ok := findChange(req.Users, 99); ok {
			return true
		}
	}
	return false
}

// singleUnmutes returns the users that were unmuted and undeafened on their own, outside handleTrackedMembers'
// batch. That is how processPlayer's applyToSingle call shows up.
func (s *playerScenario) singleUnmutes() []uint64 {
	var ids []uint64
	for _, req := range s.deps.voice.all() {
		if len(req.Users) == 1 && req.Users[0].UserID != 99 && !req.Users[0].Mute && !req.Users[0].Deaf {
			ids = append(ids, req.Users[0].UserID)
		}
	}
	return ids
}

func (s *playerScenario) expect(want playerOutcome, gotUserID string) {
	s.t.Helper()
	if gotUserID != want.userID {
		s.t.Errorf("correlated user = %q, want %q", gotUserID, want.userID)
	}
	if got := s.editPending(); got != want.edit {
		s.t.Errorf("status edit requested = %v, want %v", got, want.edit)
	}
	if got := s.handledTracked(); got != want.handleTracked {
		s.t.Errorf("tracked members handled = %v, want %v", got, want.handleTracked)
	}
	got := s.singleUnmutes()
	if len(got) != len(want.unmuted) {
		s.t.Errorf("single unmutes = %v, want %v", got, want.unmuted)
	} else {
		for i := range got {
			if got[i] != want.unmuted[i] {
				s.t.Errorf("single unmutes = %v, want %v", got, want.unmuted)
				break
			}
		}
	}
	if !s.state().Linked {
		s.t.Error("game not marked linked after a player event")
	}
}

type playerOutcome struct {
	userID        string
	edit          bool
	handleTracked bool
	unmuted       []uint64
}

// addDiscordUser adds a Discord user who is not linked to any player.
func addDiscordUser(dgs *GameState, userID, userName string) {
	dgs.UserData[userID] = UserData{
		User:       User{UserID: userID, UserName: userName},
		InGameName: amongus.UnlinkedPlayerName,
	}
}

func TestProcessPlayer_EmptyNameIsIgnored(t *testing.T) {
	s := newPlayerScenario(t, game.LOBBY, func(dgs *GameState) { dgs.Linked = false })

	if got := s.send(game.Player{Action: game.JOINED, Color: game.Red}); got != "" {
		t.Errorf("correlated user = %q, want none", got)
	}
	if s.state().Linked {
		t.Error("state was written for a nameless player")
	}
	if s.editPending() || len(s.deps.voice.all()) != 0 {
		t.Errorf("side effects for a nameless player: edit=%v voice=%+v", s.editPending(), s.deps.voice.all())
	}
}

func TestProcessPlayer_Leaving(t *testing.T) {
	cases := []struct {
		name         string
		phase        game.Phase
		disconnected bool
		seed         func(dgs *GameState)
		want         playerOutcome
		// wantLinks is the in-game name each listed Discord user is linked to afterwards
		wantLinks map[string]string
	}{
		{
			name:      "left, linked user unmuted and stays linked",
			phase:     game.LOBBY,
			seed:      func(dgs *GameState) { addLinkedUser(dgs, "10", "alice", true, true, true) },
			want:      playerOutcome{userID: "10", edit: true, handleTracked: true, unmuted: []uint64{10}},
			wantLinks: map[string]string{"10": "alice"},
		},
		{
			name:         "disconnected, linked user unmuted and unlinked",
			phase:        game.LOBBY,
			disconnected: true,
			seed:         func(dgs *GameState) { addLinkedUser(dgs, "10", "alice", true, true, true) },
			want:         playerOutcome{userID: "10", edit: true, handleTracked: true, unmuted: []uint64{10}},
			wantLinks:    map[string]string{"10": amongus.UnlinkedPlayerName},
		},
		{
			// linked through the color select, so their Discord name doesn't match the player's
			name:  "left, linked user with a different Discord name unmuted",
			phase: game.LOBBY,
			seed: func(dgs *GameState) {
				addLinkedUser(dgs, "10", "alice", true, true, true)
				u := dgs.UserData["10"]
				u.User.UserName = "bob123"
				dgs.UserData["10"] = u
			},
			want:      playerOutcome{userID: "10", edit: true, handleTracked: true, unmuted: []uint64{10}},
			wantLinks: map[string]string{"10": "alice"},
		},
		{
			// neither a matching Discord name nor a cached user ID links anyone to a player who is leaving
			name:  "left, unlinked users are not paired",
			phase: game.LOBBY,
			seed: func(dgs *GameState) {
				addDiscordUser(dgs, "10", "alice")
				addDiscordUser(dgs, "11", "someone_else")
				dgs.GameData.PlayerData["alice"] = amongus.PlayerData{Color: game.Red, Name: "alice", IsAlive: true}
			},
			want:      playerOutcome{edit: true, handleTracked: true},
			wantLinks: map[string]string{"10": amongus.UnlinkedPlayerName, "11": amongus.UnlinkedPlayerName},
		},
		{
			// editing the status message during tasks would leak who left
			name:      "left during tasks, no status edit",
			phase:     game.TASKS,
			seed:      func(dgs *GameState) { addLinkedUser(dgs, "10", "alice", true, true, true) },
			want:      playerOutcome{userID: "10", handleTracked: true, unmuted: []uint64{10}},
			wantLinks: map[string]string{"10": "alice"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newPlayerScenario(t, tc.phase, tc.seed)
			s.deps.store.mappings[scenarioGuild+":alice"] = map[string]interface{}{"11": ""}

			got := s.send(game.Player{Action: game.LEFT, Name: "alice", Color: game.Red, Disconnected: tc.disconnected})

			s.expect(tc.want, got)
			st := s.state()
			if _, ok := st.GameData.PlayerData["alice"]; ok {
				t.Error("departed player's data was kept")
			}
			for userID, want := range tc.wantLinks {
				if got := st.UserData[userID].InGameName; got != want {
					t.Errorf("user %s linked to %q, want %q", userID, got, want)
				}
			}
		})
	}
}

func TestProcessPlayer_UnmuteFailureReportsToStatusChannel(t *testing.T) {
	s := newPlayerScenario(t, game.LOBBY, func(dgs *GameState) { addLinkedUser(dgs, "10", "alice", true, true, true) })
	s.deps.voice.err = errors.New("missing permissions")

	s.send(game.Player{Action: game.LEFT, Name: "alice", Color: game.Red})

	var reported bool
	for _, m := range s.deps.discord.sent {
		if m.ChannelID == scenarioTextChannel && strings.Contains(m.Content, discord.MentionByUserID("10")) {
			reported = true
		}
	}
	if !reported {
		t.Errorf("no mute error reported for user 10; sent: %+v", s.deps.discord.sent)
	}
}

func TestProcessPlayer_Joined(t *testing.T) {
	cases := []struct {
		name string
		seed func(dgs *GameState)
		want playerOutcome
	}{
		{
			name: "paired by name",
			seed: func(dgs *GameState) { addDiscordUser(dgs, "10", "alice") },
			want: playerOutcome{userID: "10", edit: true, handleTracked: true},
		},
		{
			name: "paired by cached user ID",
			seed: func(dgs *GameState) { addDiscordUser(dgs, "11", "someone_else") },
			want: playerOutcome{userID: "11", edit: true, handleTracked: true},
		},
		{
			name: "unpaired",
			want: playerOutcome{edit: true, handleTracked: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newPlayerScenario(t, game.LOBBY, tc.seed)
			s.deps.store.mappings[scenarioGuild+":alice"] = map[string]interface{}{"11": ""}

			got := s.send(game.Player{Action: game.JOINED, Name: "alice", Color: game.Red})

			s.expect(tc.want, got)
			st := s.state()
			if p, ok := st.GameData.PlayerData["alice"]; !ok || p.Color != game.Red || !p.IsAlive {
				t.Errorf("player data = %+v (found %v), want alive red alice", p, ok)
			}
			if tc.want.userID != "" {
				if got := st.UserData[tc.want.userID].InGameName; got != "alice" {
					t.Errorf("user %s linked to %q, want alice", tc.want.userID, got)
				}
			}
		})
	}
}

func TestProcessPlayer_Updated(t *testing.T) {
	cases := []struct {
		name          string
		phase         game.Phase
		action        game.PlayerAction
		color         int
		dead          bool
		unmuteDuring  bool
		want          playerOutcome
		wantAlive     bool
		wantPlayerCol int
	}{
		{
			// editing the status message during tasks would leak who died
			name:          "died during tasks, dead stay muted",
			phase:         game.TASKS,
			action:        game.DIED,
			color:         game.Red,
			dead:          true,
			want:          playerOutcome{userID: "10"},
			wantAlive:     false,
			wantPlayerCol: game.Red,
		},
		{
			name:          "died during tasks, dead are unmuted",
			phase:         game.TASKS,
			action:        game.DIED,
			color:         game.Red,
			dead:          true,
			unmuteDuring:  true,
			want:          playerOutcome{userID: "10", edit: true, handleTracked: true},
			wantAlive:     false,
			wantPlayerCol: game.Red,
		},
		{
			name:          "exiled during tasks",
			phase:         game.TASKS,
			action:        game.EXILED,
			color:         game.Red,
			want:          playerOutcome{userID: "10", edit: true, handleTracked: true},
			wantAlive:     false,
			wantPlayerCol: game.Red,
		},
		{
			// the exiled player's mute is left to the next phase change
			name:          "exiled during discussion",
			phase:         game.DISCUSS,
			action:        game.EXILED,
			color:         game.Red,
			want:          playerOutcome{userID: "10", edit: true},
			wantAlive:     false,
			wantPlayerCol: game.Red,
		},
		{
			name:          "changed color in lobby",
			phase:         game.LOBBY,
			action:        game.CHANGECOLOR,
			color:         game.Blue,
			want:          playerOutcome{userID: "10", edit: true, handleTracked: true},
			wantAlive:     true,
			wantPlayerCol: game.Blue,
		},
		{
			// nothing changed, so nothing is done, though the game is still marked linked
			name:          "no change",
			phase:         game.LOBBY,
			action:        game.FORCEUPDATED,
			color:         game.Red,
			want:          playerOutcome{},
			wantAlive:     true,
			wantPlayerCol: game.Red,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newPlayerScenario(t, tc.phase, func(dgs *GameState) {
				addLinkedUser(dgs, "10", "alice", true, false, false)
				dgs.Linked = false
			})
			s.deps.settings.SetUnmuteDeadDuringTasks(tc.unmuteDuring)

			got := s.send(game.Player{Action: tc.action, Name: "alice", Color: tc.color, IsDead: tc.dead})

			s.expect(tc.want, got)
			p := s.state().GameData.PlayerData["alice"]
			if p.IsAlive != tc.wantAlive || p.Color != tc.wantPlayerCol {
				t.Errorf("player data = %+v, want alive=%v color=%d", p, tc.wantAlive, tc.wantPlayerCol)
			}
		})
	}
}

// TestProcessPlayer_StaleStatusMessageRefresh pins a bug. When the status message is over an hour old, processPlayer
// refreshes it while still holding the game state lock. The refresh deletes the old message and stores the new one's
// ID, then processPlayer's own deferred write puts the deleted message's ID back. Moving the refresh after the lock
// is released should flip the last two assertions.
func TestProcessPlayer_StaleStatusMessageRefresh(t *testing.T) {
	s := newPlayerScenario(t, game.LOBBY, func(dgs *GameState) {
		dgs.GameStateMsg.CreationTimeUnix = time.Now().Add(-2 * time.Hour).Unix()
	})

	s.send(game.Player{Action: game.JOINED, Name: "alice", Color: game.Red})

	if len(s.deps.discord.deleted) != 1 || s.deps.discord.deleted[0] != scenarioTextChannel+"/"+s.msgID {
		t.Fatalf("deleted = %v, want the old status message", s.deps.discord.deleted)
	}
	var replacement *discordgo.Message
	for _, m := range s.deps.discord.sent {
		if len(m.Embeds) > 0 {
			replacement = m
		}
	}
	if replacement == nil {
		t.Fatalf("no replacement status message sent; sent: %+v", s.deps.discord.sent)
	}

	st := s.state()
	if _, ok := st.GameData.PlayerData["alice"]; !ok {
		t.Error("joined player was lost")
	}
	// BUG: the stored message is the deleted one, not the replacement
	if st.GameStateMsg.MessageID != s.msgID {
		t.Errorf("stored status message = %q, want the deleted %q (pinned bug)", st.GameStateMsg.MessageID, s.msgID)
	}
	// BUG: so the next event sees an old message and refreshes again
	if !st.shouldRefresh() {
		t.Error("stored state no longer needs a refresh (pinned bug)")
	}
}
