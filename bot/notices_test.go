package bot

import (
	"context"
	"github.com/automuteus/automuteus/v8/internal/server"
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

// seedMatch puts a running game in the tasks phase in the store, with a match record open and the given players
// linked, muted and deafened in its voice channel, and registers it as a game this shard is subscribed to.
func seedMatch(t *testing.T, bot *Bot, deps *testDeps, code, textChannel, voiceChannel string, users ...string) GameStateRequest {
	t.Helper()
	dgs := NewDiscordGameState(scenarioGuild)
	dgs.ConnectCode = code
	dgs.Running = true
	dgs.Linked = true
	dgs.VoiceChannel = voiceChannel
	dgs.GameData.UpdatePhase(game.TASKS)
	dgs.MatchID = int64(len(code))
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

// postedIn returns the content of the last message sent to a channel.
func postedIn(deps *testDeps, channelID string) string {
	deps.discord.mu.Lock()
	defer deps.discord.mu.Unlock()
	var content string
	for _, m := range deps.discord.sent {
		if m.ChannelID == channelID {
			content = m.Content
		}
	}
	return content
}

// standInSubscriber plays the capture subscriber for a game: on the end request it completes cleanup and
// acknowledges, as the real one does.
func standInSubscriber(bot *Bot, gsr GameStateRequest) <-chan struct{} {
	kill := make(chan EndGameMessage, 1)
	bot.EndGameChannels[gsr.ConnectCode] = kill
	done := make(chan struct{})
	go func() {
		end := <-kill
		err := bot.completeGame(gsr, end.reason, end.message)
		bot.ChannelsMapLock.Lock()
		delete(bot.EndGameChannels, gsr.ConnectCode)
		bot.ChannelsMapLock.Unlock()
		end.done <- err
		close(done)
	}()
	return done
}

func TestHandleEvent_CriticalNoticeEndsEveryGameOnTheShard(t *testing.T) {
	bot, deps := newTestBot(t)
	withRedis(t, bot)
	first, _ := seedTwoMatches(t, bot, deps)
	done := standInSubscriber(bot, first)

	deps.notices.set(&notice.Notice{Severity: notice.Critical, Message: "platform going down"})
	bot.handleEvent(&notice.Event{NoticeChanged: true})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("capture subscriber was never asked to end the game")
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
	for _, ch := range []string{scenarioTextChannel, secondTextChannel} {
		if !strings.Contains(postedIn(deps, ch), "platform going down") {
			t.Errorf("end-of-game message with the notice missing from channel %s", ch)
		}
	}
	if _, ok := bot.EndGameChannels[scenarioConnectCode]; ok {
		t.Error("end channel should have been removed")
	}
	if got := deps.metrics.endedBy(server.EndReasonCriticalNotice); got != 2 {
		t.Errorf("games ended for a critical notice = %d, want 2", got)
	}
}

func TestHandleEvent_ShutdownOnlyEndsListedGames(t *testing.T) {
	bot, deps := newTestBot(t)
	withRedis(t, bot)
	seedTwoMatches(t, bot, deps)

	bot.handleEvent(&notice.Event{Shutdown: &notice.Shutdown{ConnectCodes: []string{"QRSTUVWX"}}})
	if n := len(deps.voice.all()); n != 0 {
		t.Fatalf("a shutdown naming another game issued %d voice changes", n)
	}
	bot.handleEvent(&notice.Event{Shutdown: &notice.Shutdown{ConnectCodes: []string{}}})
	if n := len(deps.voice.all()); n != 0 {
		t.Fatalf("a shutdown naming no games issued %d voice changes", n)
	}

	bot.handleEvent(&notice.Event{Shutdown: &notice.Shutdown{ConnectCodes: []string{"QRSTUVWX", secondConnectCode}}})

	if deps.store.getCode(secondConnectCode) != nil {
		t.Error("game whose capture is going away should have been ended")
	}
	if got := deps.store.getCode(scenarioConnectCode); got == nil || !got.Running {
		t.Error("game not named by the shutdown should be untouched")
	}
	unmuted := unmutedUsers(deps)
	if !unmuted[20] || !unmuted[21] || unmuted[10] || unmuted[11] {
		t.Errorf("unmuted users = %v, want exactly 20 and 21", unmuted)
	}
	if posted := postedIn(deps, secondTextChannel); !strings.Contains(posted, "capture service is restarting") {
		t.Errorf("end message should explain the capture restart, got %q", posted)
	}
	if posted := postedIn(deps, scenarioTextChannel); posted != "" {
		t.Errorf("unaffected game got a message: %q", posted)
	}
	if got := deps.metrics.endedBy(server.EndReasonCaptureShutdown); got != 1 {
		t.Errorf("games ended for a capture shutdown = %d, want 1", got)
	}
}

func TestHandleEvent_WarningRefreshesStatusMessagesAndLeavesGamesRunning(t *testing.T) {
	bot, deps := newTestBot(t)
	seedTwoMatches(t, bot, deps)
	deps.notices.set(&notice.Notice{Severity: notice.Warning, Message: "expect some lag"})

	bot.handleEvent(&notice.Event{NoticeChanged: true})

	// both status messages get edited (after the deferred-edit delay, which is recorded rather than slept)
	eventually(t, "two status message edits", func() bool { return deps.discord.editCount() == 2 })
	deps.discord.mu.Lock()
	edits := append([]*discordgo.MessageEdit(nil), deps.discord.edits...)
	deps.discord.mu.Unlock()
	for _, e := range edits {
		if len(e.Embeds) == 0 || len(e.Embeds[0].Fields) == 0 || !strings.Contains(e.Embeds[0].Fields[0].Name, "WARNING") {
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

func TestHandleEvent_ClearedRefreshesStatusMessagesWithoutBanner(t *testing.T) {
	bot, deps := newTestBot(t)
	seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	deps.notices.set(nil) // the operator has already deleted the active notice

	bot.handleEvent(&notice.Event{NoticeChanged: true})

	eventually(t, "a status message edit", func() bool { return deps.discord.editCount() == 1 })
	deps.discord.mu.Lock()
	edit := deps.discord.edits[0]
	deps.discord.mu.Unlock()
	for _, f := range edit.Embeds[0].Fields {
		if strings.Contains(f.Name, "WARNING") || strings.Contains(f.Name, "CRITICAL") {
			t.Errorf("refresh after clear still shows a banner: %+v", f)
		}
	}
}

func TestListenForNotices_DeliversPublishedEventsToHandler(t *testing.T) {
	bot, deps := newTestBot(t)
	seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	bot.RedisInterface = &RedisInterface{client: client}
	ctx := context.Background()
	sub := notice.Subscribe(ctx, client)
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	go bot.listenForNotices(sub)

	if err := notice.AnnounceShutdown(ctx, client, []string{scenarioConnectCode}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the game to be ended by the published shutdown", func() bool { return deps.store.getCode(scenarioConnectCode) == nil })
	if !unmutedUsers(deps)[10] {
		t.Error("published shutdown did not reach the handler: user 10 still muted")
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
		wantColor int
	}{
		{"critical", &notice.Notice{Severity: notice.Critical, Message: "going down"}, "CRITICAL", discord.RED},
		{"warning", &notice.Notice{Severity: notice.Warning, Message: "degraded"}, "WARNING", discord.YELLOW},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			deps.notices.set(c.n)
			embed := bot.gameStateResponse(dgs, sett)
			if len(embed.Fields) == 0 {
				t.Fatal("no fields on embed")
			}
			banner := embed.Fields[0]
			if !strings.Contains(banner.Name, c.wantTitle) || !strings.Contains(banner.Value, c.n.Message) || banner.Inline {
				t.Errorf("banner = %+v, want full-width %s containing %q", banner, c.wantTitle, c.n.Message)
			}
			if embed.Color != c.wantColor {
				t.Errorf("color = %d, want %d", embed.Color, c.wantColor)
			}
		})
	}

	deps.notices.set(nil)
	if embed := bot.gameStateResponse(dgs, sett); len(embed.Fields) > 0 && (strings.Contains(embed.Fields[0].Name, "WARNING") || strings.Contains(embed.Fields[0].Name, "CRITICAL")) {
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
	if got := deps.metrics.endedBy(server.EndReasonInactivity); got != 1 {
		t.Errorf("games ended for inactivity = %d, want 1", got)
	}
	if got := deps.metrics.cleanupFailed(server.CleanupUnmute); got != 0 {
		t.Errorf("unmute cleanup failures = %d, want 0 when the unmute succeeded", got)
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
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Errorf("took %v, expected the pool to overlap the work", elapsed)
	}
}
