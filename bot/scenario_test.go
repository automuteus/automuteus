package bot

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/bwmarrin/discordgo"
)

// These tests drive the bot's game logic with spoofed inputs (capture jobs and Discord gateway events) against the
// in-memory fakes in fakes_test.go, and assert on the mutes, messages, and match records the bot produces.

const (
	scenarioGuild       = "1"
	scenarioConnectCode = "ABCDEFGH"
	scenarioTextChannel = "500"
)

// runningGame seeds the store with a running game in the given phase whose status message already exists, and
// registers the guild (with the given voice states) in the cached Discord state.
func runningGame(deps *testDeps, phase game.Phase, voiceStates ...*discordgo.VoiceState) *GameState {
	dgs := voiceTestState(phase)
	dgs.GameStateMsg = GameStateMessage{
		MessageID:        "status-msg",
		MessageChannelID: scenarioTextChannel,
		LeaderID:         "10",
		CreationTimeUnix: time.Now().Unix(),
	}
	if err := deps.guilds.GuildAdd(&discordgo.Guild{ID: scenarioGuild, VoiceStates: voiceStates}); err != nil {
		panic(err)
	}
	return dgs
}

func phaseJob(phase game.Phase) task.Job {
	return task.Job{JobType: task.StateJob, Payload: strconv.Itoa(int(phase))}
}

func TestProcessJob_LobbyToTasks_StartsMatchAndMutesLinkedPlayers(t *testing.T) {
	bot, deps := newTestBot(t)
	sett := settings.MakeGuildSettings()

	dgs := runningGame(deps, game.LOBBY, inChannel("10", trackedChannel), inChannel("11", trackedChannel), inChannel("12", otherChannel))
	addLinkedUser(dgs, "10", "alice", true, false, false)
	addLinkedUser(dgs, "11", "bob", true, false, false)
	addLinkedUser(dgs, "12", "carol", true, false, false) // linked, but sitting in a different voice channel
	deps.store.put(dgs)

	gsr := GameStateRequest{GuildID: scenarioGuild, ConnectCode: scenarioConnectCode}
	bot.processJob(phaseJob(game.TASKS), sett, premium.FreeTier, gsr)

	// state advanced and a match record was opened
	got := deps.store.get()
	if got.GameData.GetPhase() != game.TASKS {
		t.Fatalf("phase = %v, want TASKS", got.GameData.GetPhase())
	}
	if got.MatchID != 1 || got.MatchStartUnix <= 0 {
		t.Fatalf("match not started: id=%d start=%d", got.MatchID, got.MatchStartUnix)
	}
	if len(deps.recorder.games) != 1 || deps.recorder.games[0].ConnectCode != scenarioConnectCode {
		t.Fatalf("recorder games = %+v", deps.recorder.games)
	}
	// no lobby event was seen, so the map and region are unknown rather than defaulted
	if g := deps.recorder.games[0]; g.PlayMap != nil || g.Region != nil {
		t.Errorf("map/region = %v/%v, want nil/nil without a lobby event", g.PlayMap, g.Region)
	}

	// the configured lobby->tasks delay was honored (without actually sleeping)
	wantDelay := time.Second * time.Duration(sett.GetDelay(game.LOBBY, game.TASKS))
	if slept := deps.slept(); len(slept) != 1 || slept[0] != wantDelay {
		t.Fatalf("sleeps = %v, want [%v]", slept, wantDelay)
	}

	// exactly one batch of mutes went out, covering only the linked users in the tracked channel
	reqs := deps.voice.all()
	if len(reqs) != 1 {
		t.Fatalf("voice requests = %d, want 1: %+v", len(reqs), reqs)
	}
	wantMute, wantDeaf := sett.GetVoiceState(true, true, game.TASKS)
	if len(reqs[0].Users) != 2 {
		t.Fatalf("users muted = %+v, want alice and bob only", reqs[0].Users)
	}
	for _, id := range []uint64{10, 11} {
		u, ok := findChange(reqs[0].Users, id)
		if !ok || u.Mute != wantMute || u.Deaf != wantDeaf {
			t.Errorf("user %d: got (%v, mute=%v, deaf=%v), want (found, mute=%v, deaf=%v)", id, ok, u.Mute, u.Deaf, wantMute, wantDeaf)
		}
	}
	if _, ok := findChange(reqs[0].Users, 12); ok {
		t.Errorf("carol is in another channel and should not have been touched")
	}

	// the bot remembered what it asked Discord to do, so a repeat produces no further changes
	if u, _ := got.GetUser("10"); u.ShouldBeMute != wantMute || u.ShouldBeDeaf != wantDeaf {
		t.Errorf("stored intent for alice = (mute=%v, deaf=%v), want (%v, %v)", u.ShouldBeMute, u.ShouldBeDeaf, wantMute, wantDeaf)
	}
	bot.processJob(phaseJob(game.TASKS), sett, premium.FreeTier, gsr)
	if len(deps.voice.all()) != 1 {
		t.Fatalf("repeating the same phase should be a no-op, got %d requests", len(deps.voice.all()))
	}

	// every line the game path logged is attributable to this game
	for _, line := range strings.Split(strings.TrimSpace(deps.logs.String()), "\n") {
		if !strings.Contains(line, "guild="+scenarioGuild) || !strings.Contains(line, "code="+scenarioConnectCode) {
			t.Errorf("log line missing game identifiers: %s", line)
		}
	}
	if logs := deps.logs.String(); !strings.Contains(logs, `msg="phase changed"`) || !strings.Contains(logs, "from=LOBBY to=TASKS") {
		t.Errorf("expected a phase change log line, got:\n%s", deps.logs.String())
	}
}

func TestVoiceStateChange_JoiningTrackedChannelMidGameMutesLinkedUser(t *testing.T) {
	bot, deps := newTestBot(t)

	dgs := runningGame(deps, game.TASKS, inChannel("10", trackedChannel))
	addLinkedUser(dgs, "10", "alice", true, true, true)
	addLinkedUser(dgs, "11", "bob", true, false, false) // linked, currently not in any voice channel
	deps.store.put(dgs)

	// bob joins the tracked channel while tasks are underway
	bot.handleVoiceStateChange(nil, &discordgo.VoiceStateUpdate{VoiceState: &discordgo.VoiceState{
		GuildID:   scenarioGuild,
		ChannelID: trackedChannel,
		UserID:    "11",
		SessionID: "sess",
	}})

	wantMute, wantDeaf := deps.settings.GetVoiceState(true, true, game.TASKS)
	reqs := deps.voice.all()
	if len(reqs) != 1 || len(reqs[0].Users) != 1 {
		t.Fatalf("voice requests = %+v, want exactly one change for bob", reqs)
	}
	if u := reqs[0].Users[0]; u.UserID != 11 || u.Mute != wantMute || u.Deaf != wantDeaf {
		t.Fatalf("change = %+v, want user 11 mute=%v deaf=%v", u, wantMute, wantDeaf)
	}
	if u, _ := deps.store.get().GetUser("11"); u.ShouldBeMute != wantMute || u.ShouldBeDeaf != wantDeaf {
		t.Errorf("stored intent for bob not updated: %+v", u)
	}
}

func TestProcessJob_LobbyToTasks_RecordsMapAndRegion(t *testing.T) {
	bot, deps := newTestBot(t)
	sett := settings.MakeGuildSettings()

	deps.store.put(runningGame(deps, game.LOBBY))
	gsr := GameStateRequest{GuildID: scenarioGuild, ConnectCode: scenarioConnectCode}
	bot.processJob(task.Job{JobType: task.LobbyJob, Payload: `{"LobbyCode":"ABCDEF","Region":2,"Map":4}`}, sett, premium.FreeTier, gsr)
	bot.processJob(phaseJob(game.TASKS), sett, premium.FreeTier, gsr)

	if len(deps.recorder.games) != 1 {
		t.Fatalf("recorder games = %+v", deps.recorder.games)
	}
	g := deps.recorder.games[0]
	if g.PlayMap == nil || *g.PlayMap != int16(game.AIRSHIP) {
		t.Errorf("play map = %v, want %d", g.PlayMap, game.AIRSHIP)
	}
	if g.Region == nil || *g.Region != int16(game.EU) {
		t.Errorf("region = %v, want %d", g.Region, game.EU)
	}
}

func TestProcessJob_GameOver_RecordsPayloadAgainstClosingMatch(t *testing.T) {
	bot, deps := newTestBot(t)
	sett := settings.MakeGuildSettings()

	dgs := runningGame(deps, game.TASKS)
	addLinkedUser(dgs, "10", "alice", true, false, false)
	dgs.MatchID = 7
	dgs.MatchStartUnix = time.Now().Unix() - 60
	deps.store.put(dgs)

	payload := `{"GameOverReason":3,"PlayerInfos":[{"Name":"alice","IsImpostor":true},{"Name":"unlinked","IsImpostor":false}]}`
	gsr := GameStateRequest{GuildID: scenarioGuild, ConnectCode: scenarioConnectCode}
	bot.processJob(task.Job{JobType: task.GameOverJob, Payload: payload}, sett, premium.FreeTier, gsr)

	eventually(t, "the match result to be recorded", func() bool {
		deps.recorder.mu.Lock()
		defer deps.recorder.mu.Unlock()
		return len(deps.recorder.updates) == 1
	})
	deps.recorder.mu.Lock()
	defer deps.recorder.mu.Unlock()
	if deps.recorder.updates[0] != 7 {
		t.Fatalf("result recorded for match %d, want 7", deps.recorder.updates[0])
	}
	// the raw payload is kept, so unlinked players' roles survive even though only linked players get a result row
	if len(deps.recorder.events) != 1 {
		t.Fatalf("events = %+v, want the game over event only", deps.recorder.events)
	}
	if e := deps.recorder.events[0]; e.GameID != 7 || e.EventType != int16(task.GameOverJob) || e.Payload != payload || e.UserID != nil {
		t.Fatalf("event = %+v", e)
	}
	if got := deps.store.get(); got.MatchID != -1 {
		t.Fatalf("match still open after game over: %d", got.MatchID)
	}
}
