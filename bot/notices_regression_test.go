package bot

import (
	"context"
	"errors"
	"github.com/automuteus/automuteus/v8/internal/server"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/lock"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/go-redis/redis/v8"
)

type unavailableNoticeSettings struct{}

func (unavailableNoticeSettings) LoadGuildSettings(context.Context, string) (*settings.GuildSettings, error) {
	return nil, errors.New("settings unavailable")
}

func TestHandleEvent_ShutdownCleansUpWithoutSettings(t *testing.T) {
	bot, deps := newTestBot(t)
	seedTwoMatches(t, bot, deps)
	bot.settings = unavailableNoticeSettings{}
	bot.handleEvent(&notice.Event{Shutdown: &notice.Shutdown{ConnectCodes: []string{scenarioConnectCode}}})
	if deps.store.getCode(scenarioConnectCode) != nil {
		t.Error("targeted game survived the critical notice when settings were unavailable")
	}
	if deps.store.getCode(secondConnectCode) == nil {
		t.Error("unrelated game was stopped")
	}
	if got := deps.recorder.aborted; len(got) != 1 {
		t.Errorf("aborted matches = %v, want one", got)
	}
	if users := unmutedUsers(deps); !users[10] || !users[11] || users[20] || users[21] {
		t.Errorf("unmuted users = %v, want exactly 10 and 11", users)
	}
	if len(deps.discord.sent) != 1 || !strings.Contains(deps.discord.sent[0].Content, "capture service is restarting") {
		t.Errorf("missing fallback end message: %+v", deps.discord.sent)
	}
}

type observedNoticeVoice struct {
	*fakeVoice
	called chan struct{}
	before func()
}

func (v observedNoticeVoice) ModifyUsers(guild, code string, req task.UserModifyRequest, l lock.Lock) error {
	if v.before != nil {
		v.before()
	}
	err := v.fakeVoice.ModifyUsers(guild, code, req, l)
	if v.called != nil {
		v.called <- struct{}{}
	}
	return err
}

func TestStopGame_StopsStateBeforeUnmutingAndReportsFailure(t *testing.T) {
	bot, deps := newTestBot(t)
	gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	deps.voice.err = errors.New("Discord rejected the unmute")
	bot.voice = observedNoticeVoice{fakeVoice: deps.voice, before: func() {
		if dgs := deps.store.getCode(scenarioConnectCode); dgs == nil || dgs.Running {
			t.Error("game must be marked stopped before sending the final unmute")
		}
	}}
	if err := bot.stopGame(gsr, server.EndReasonCriticalNotice, ""); !errors.Is(err, deps.voice.err) {
		t.Errorf("stop error = %v, want the unmute failure", err)
	}
	if got := deps.metrics.cleanupFailed(server.CleanupUnmute); got != 1 {
		t.Errorf("unmute cleanup failures = %d, want 1", got)
	}
	if got := deps.metrics.cleanupFailed(server.CleanupRecordMatch) + deps.metrics.cleanupFailed(server.CleanupNotify); got != 0 {
		t.Errorf("other cleanup steps succeeded but %d were counted as failed", got)
	}
	if got := deps.metrics.endedBy(server.EndReasonCriticalNotice); got != 1 {
		t.Errorf("games ended for a critical notice = %d, want 1 even though unmute failed", got)
	}
}

func TestStopGame_DelayedCaptureMuteFinishesBeforeFinalUnmute(t *testing.T) {
	bot, deps := newTestBot(t)
	gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	dgs := deps.store.getCode(scenarioConnectCode)
	dgs.GameData.UpdatePhase(game.DISCUSS)
	for id, user := range dgs.UserData {
		user.SetShouldBeMuteDeaf(false, false)
		dgs.UserData[id] = user
	}
	deps.store.put(dgs)
	deps.settings.SetDelay(game.DISCUSS, game.TASKS, 3)
	delaying, resume := make(chan struct{}), make(chan struct{})
	var resumeOnce sync.Once
	release := func() { resumeOnce.Do(func() { close(resume) }) }
	t.Cleanup(release)
	bot.sleep = func(d time.Duration) {
		if d == 3*time.Second {
			close(delaying)
			<-resume
		}
	}
	voiceCalled := make(chan struct{}, 10)
	bot.voice = observedNoticeVoice{fakeVoice: deps.voice, called: voiceCalled}
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	bot.RedisInterface = &RedisInterface{client: client}
	ack := task.AckSubscribe(context.Background(), client, scenarioConnectCode)
	defer ack.Close()
	if _, err := ack.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}
	stop := make(chan EndGameMessage, 1)
	bot.EndGameChannels[scenarioConnectCode] = stop
	subscriberDone := make(chan struct{})
	go func() {
		bot.SubscribeToGameByConnectCode(scenarioGuild, scenarioConnectCode, stop)
		close(subscriberDone)
	}()
	t.Cleanup(func() {
		release()
		select {
		case <-subscriberDone:
			return
		default:
		}
		select {
		case stop <- EndGameMessage{reason: "test cleanup"}: // unknown reasons are counted as "other"
		default:
		}
		select {
		case <-subscriberDone:
		case <-time.After(2 * time.Second):
			t.Error("capture subscriber survived test cleanup")
		}
	})
	select {
	case <-ack.Channel():
	case <-time.After(time.Second):
		t.Fatal("subscriber did not start")
	}
	if err := task.PushJob(context.Background(), client, scenarioConnectCode, task.StateJob, phaseJob(game.TASKS).Payload.(string)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-delaying:
	case <-time.After(time.Second):
		t.Fatal("capture phase transition did not reach its delay")
	}
	stopDone := make(chan struct{})
	go func() {
		// /end supplies the channel rather than the connect code; it must still stop the existing subscriber.
		if err := bot.stopGame(GameStateRequest{GuildID: gsr.GuildID, TextChannel: scenarioTextChannel}, "maintenance", ""); err != nil {
			t.Errorf("stop game: %v", err)
		}
		close(stopDone)
	}()
	eventually(t, "the stop request to be queued behind the delayed capture operation", func() bool { return len(stop) == 1 })
	select {
	case <-voiceCalled:
		t.Error("cleanup unmuted players while a capture mute was still pending")
	default:
	}
	release()
	for _, done := range []chan struct{}{subscriberDone, stopDone} {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("game shutdown did not finish")
		}
	}
	requests := deps.voice.all()
	if len(requests) < 2 {
		t.Fatalf("voice requests = %+v, want a capture mute followed by cleanup", requests)
	}
	for _, user := range requests[len(requests)-1].Users {
		if user.Mute || user.Deaf {
			t.Errorf("last voice change leaves user muted/deafened: %+v", user)
		}
	}
	if deps.store.getCode(scenarioConnectCode) != nil {
		t.Error("game state survived shutdown")
	}
}
