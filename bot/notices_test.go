package bot

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
)

const (
	secondConnectCode  = "IJKLMNOP"
	secondTextChannel  = "600"
	secondVoiceChannel = "910"
)

// seedMatch puts a running game in the tasks phase in the store, with a match record open and two linked players
// muted and deafened in its voice channel, and registers it as a game this shard is subscribed to.
func seedMatch(t *testing.T, bot *Bot, deps *testDeps, code, textChannel, voiceChannel string, users ...string) GameStateRequest {
	t.Helper()
	dgs := NewDiscordGameState(scenarioGuild)
	dgs.ConnectCode = code
	dgs.Running = true
	dgs.Linked = true
	dgs.VoiceChannel = voiceChannel
	dgs.GameData.UpdatePhase(game.TASKS)
	dgs.MatchID = int64(len(code)) // any positive id; distinct per code length is not needed
	dgs.MatchStartUnix = time.Now().Unix()
	dgs.GameStateMsg = GameStateMessage{MessageID: "status-" + code, MessageChannelID: textChannel, LeaderID: users[0], CreationTimeUnix: time.Now().Unix()}

	guild, err := deps.guilds.Guild(scenarioGuild)
	if err != nil {
		guild = &discordgo.Guild{ID: scenarioGuild}
	}
	for i, u := range users {
		addLinkedUserWithColor(dgs, u, "player"+u, i, true, true, true)
		guild.VoiceStates = append(guild.VoiceStates, inChannel(u, voiceChannel))
	}
	if err := deps.guilds.GuildAdd(guild); err != nil {
		t.Fatal(err)
	}
	deps.store.put(dgs)
	gsr := GameStateRequest{GuildID: scenarioGuild, ConnectCode: code}
	bot.trackGame(gsr)
	return gsr
}

func seedTwoMatches(t *testing.T, bot *Bot, deps *testDeps) (first, second GameStateRequest) {
	t.Helper()
	first = seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	second = seedMatch(t, bot, deps, secondConnectCode, secondTextChannel, secondVoiceChannel, "20", "21")
	return first, second
}

// unmutedUsers returns every user that received a mute=false, deaf=false change.
func unmutedUsers(deps *testDeps) map[uint64]bool {
	users := map[uint64]bool{}
	for _, req := range deps.voice.all() {
		for _, u := range req.Users {
			if !u.Mute && !u.Deaf {
				users[u.UserID] = true
			}
		}
	}
	return users
}

func TestHandleNotice_CriticalEndsEveryGameOnTheShard(t *testing.T) {
	bot, deps := newTestBot(t)
	first, _ := seedTwoMatches(t, bot, deps)

	// stand in for the first game's capture subscriber: on the end signal it deletes the game, as the real one does
	kill := make(chan EndGameMessage)
	bot.EndGameChannels[scenarioConnectCode] = kill
	done := make(chan struct{})
	go func() {
		<-kill
		bot.forceEndGame(first)
		close(done)
	}()

	bot.handleNotice(&notice.Notice{Severity: notice.Critical, Message: "platform going down", Source: "admin-api"})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("capture subscriber was never signalled to end the game")
	}

	if got := deps.recorder.aborted; len(got) != 2 {
		t.Errorf("aborted matches = %v, want both", got)
	}
	for _, u := range []uint64{10, 11, 20, 21} {
		if !unmutedUsers(deps)[u] {
			t.Errorf("user %d was not unmuted", u)
		}
	}
	for _, code := range []string{scenarioConnectCode, secondConnectCode} {
		if deps.store.getCode(code) != nil {
			t.Errorf("game %s should have been deleted", code)
		}
	}
	posted := map[string]bool{}
	for _, m := range deps.discord.sent {
		if strings.Contains(m.Content, "platform going down") {
			posted[m.ChannelID] = true
		}
	}
	if !posted[scenarioTextChannel] || !posted[secondTextChannel] {
		t.Errorf("end-of-game message missing from a game channel: %v", posted)
	}
	if _, ok := bot.EndGameChannels[scenarioConnectCode]; ok {
		t.Error("end channel should have been removed")
	}
}

func TestHandleNotice_TargetedCriticalOnlyEndsListedGames(t *testing.T) {
	bot, deps := newTestBot(t)
	seedTwoMatches(t, bot, deps)

	bot.handleNotice(&notice.Notice{Severity: notice.Critical, Message: "unrelated", ConnectCodes: []string{"QRSTUVWX"}})
	if n := len(deps.voice.all()); n != 0 {
		t.Fatalf("a notice targeted at another game issued %d voice changes", n)
	}

	bot.handleNotice(&notice.Notice{Severity: notice.Critical, Message: "restarting", MessageID: notice.GalactusShutdownMessageID, ConnectCodes: []string{"QRSTUVWX", secondConnectCode}})

	if deps.store.getCode(secondConnectCode) != nil {
		t.Error("targeted game should have been ended")
	}
	if got := deps.store.getCode(scenarioConnectCode); got == nil || !got.Running {
		t.Error("game not named by the notice should be untouched")
	}
	unmuted := unmutedUsers(deps)
	if !unmuted[20] || !unmuted[21] || unmuted[10] || unmuted[11] {
		t.Errorf("unmuted users = %v, want exactly 20 and 21", unmuted)
	}
	var posted string
	for _, m := range deps.discord.sent {
		if m.ChannelID == secondTextChannel {
			posted = m.Content
		}
	}
	if !strings.Contains(posted, "capture service is restarting") {
		t.Errorf("end message should carry the localized shutdown text, got %q", posted)
	}
}

func TestHandleNotice_WarningRefreshesStatusMessagesAndLeavesGamesRunning(t *testing.T) {
	bot, deps := newTestBot(t)
	seedTwoMatches(t, bot, deps)
	deps.notices.set(&notice.Notice{Severity: notice.Warning, Message: "expect some lag"})

	bot.handleNotice(&notice.Notice{Severity: notice.Warning, Message: "expect some lag"})

	// both status messages get edited (after the deferred-edit delay, which is recorded rather than slept)
	eventually(t, "two status message edits", func() bool { return deps.discord.editCount() == 2 })
	deps.discord.mu.Lock()
	edits := append([]*discordgo.MessageEdit(nil), deps.discord.edits...)
	deps.discord.mu.Unlock()
	for _, e := range edits {
		if e.Embeds == nil || len(e.Embeds) == 0 || len(e.Embeds[0].Fields) == 0 || !strings.Contains(e.Embeds[0].Fields[0].Name, "WARNING") {
			t.Errorf("edited embed lacks the warning banner: %+v", e.Embeds)
		}
	}
	if n := len(deps.voice.all()); n != 0 {
		t.Errorf("warning issued %d voice changes, want 0", n)
	}
	if len(deps.recorder.aborted) != 0 {
		t.Errorf("warning aborted matches: %v", deps.recorder.aborted)
	}
	for _, code := range []string{scenarioConnectCode, secondConnectCode} {
		if got := deps.store.getCode(code); got == nil || !got.Running {
			t.Errorf("warning changed game %s: %+v", code, got)
		}
	}
}

func TestHandleNotice_ExpiringNoticeSchedulesARefresh(t *testing.T) {
	bot, deps := newTestBot(t)
	seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	expiresIn := 10 * time.Minute
	n := &notice.Notice{Severity: notice.Warning, Message: "brief", ExpiresAt: time.Now().Add(expiresIn).Unix()}
	deps.notices.set(n)

	bot.handleNotice(n)

	// one edit now (banner appears) and one after the notice lapses (banner disappears); the wait is recorded, not slept
	eventually(t, "the deferred refresh to have been scheduled", func() bool {
		for _, d := range deps.slept() {
			if d > expiresIn-time.Minute && d <= expiresIn {
				return true
			}
		}
		return false
	})
	deps.notices.set(nil)
	eventually(t, "two status message edits", func() bool { return deps.discord.editCount() == 2 })
}

func TestListenForNotices_DeliversPublishedNoticesToHandler(t *testing.T) {
	bot, deps := newTestBot(t)
	seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	sub := notice.Subscribe(ctx, client)
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	go bot.listenForNotices(sub)

	if err := notice.Raise(ctx, client, notice.Notice{Severity: notice.Critical, Message: "restarting", ConnectCodes: []string{scenarioConnectCode}}, 0); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the game to be ended by the published notice", func() bool { return deps.store.getCode(scenarioConnectCode) == nil })
	if !unmutedUsers(deps)[10] {
		t.Error("published notice did not reach the handler: user 10 still muted")
	}
}

func TestGameStateResponse_ShowsActiveNoticeBanner(t *testing.T) {
	bot, deps := newTestBot(t)
	sett := settings.MakeGuildSettings()
	dgs := runningGame(deps, game.TASKS)
	dgs.Linked = true
	deps.store.put(dgs)

	cases := []struct {
		name      string
		n         *notice.Notice
		wantTitle string
		wantText  string
		wantColor int
	}{
		{"critical", &notice.Notice{Severity: notice.Critical, Message: "going down"}, "CRITICAL", "going down", discord.RED},
		{"warning", &notice.Notice{Severity: notice.Warning, Message: "degraded"}, "WARNING", "degraded", discord.YELLOW},
		{"info", &notice.Notice{Severity: notice.Info, Message: "v9 is out"}, "NOTICE", "v9 is out", 0},
		{"localized by id", &notice.Notice{Severity: notice.Critical, Message: "fallback", MessageID: notice.GalactusShutdownMessageID}, "CRITICAL", "capture service is restarting", discord.RED},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			deps.notices.set(c.n)
			embed := bot.gameStateResponse(dgs, sett)
			if len(embed.Fields) == 0 {
				t.Fatal("no fields on embed")
			}
			banner := embed.Fields[0]
			if !strings.Contains(banner.Name, c.wantTitle) || !strings.Contains(banner.Value, c.wantText) || banner.Inline {
				t.Errorf("banner = %+v, want full-width %s containing %q", banner, c.wantTitle, c.wantText)
			}
			if c.wantColor != 0 && embed.Color != c.wantColor {
				t.Errorf("color = %d, want %d", embed.Color, c.wantColor)
			}
		})
	}

	deps.notices.set(&notice.Notice{Cleared: true})
	if embed := bot.gameStateResponse(dgs, sett); len(embed.Fields) > 0 && strings.Contains(embed.Fields[0].Name, "NOTICE") {
		t.Error("cleared notice still rendered a banner")
	}
	deps.notices.set(nil)
	if embed := bot.gameStateResponse(dgs, sett); len(embed.Fields) > 0 && strings.Contains(embed.Fields[0].Name, "NOTICE") {
		t.Error("no notice still rendered a banner")
	}
}

func TestEndInactiveGame_UnmutesAndAborts(t *testing.T) {
	bot, deps := newTestBot(t)
	gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")

	bot.endInactiveGame(gsr)

	unmuted := unmutedUsers(deps)
	if !unmuted[10] || !unmuted[11] {
		t.Errorf("unmuted users = %v, want 10 and 11", unmuted)
	}
	if got := deps.recorder.aborted; len(got) != 1 {
		t.Errorf("aborted matches = %v, want one", got)
	}
	if deps.store.get() != nil {
		t.Error("game state should have been deleted")
	}
}

func TestForEachGame_RunsConcurrentlyWithABound(t *testing.T) {
	games := make([]GameStateRequest, 20)
	var inFlight, peak int32
	var mu sync.Mutex
	start := time.Now()
	forEachGame(games, func(GameStateRequest) {
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		inFlight--
		mu.Unlock()
	})
	if peak > noticeWorkers || peak < 2 {
		t.Errorf("peak concurrency = %d, want between 2 and %d", peak, noticeWorkers)
	}
	// 20 games at 20ms each: serial would be 400ms, 8 workers should finish in about 60ms
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Errorf("took %v, expected the pool to overlap the work", elapsed)
	}
}
