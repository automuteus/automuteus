package bot

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/go-redis/redis/v8"
)

type lifecycleRedisHook struct {
	before func(redis.Cmder) error
}

func (h lifecycleRedisHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return ctx, h.before(cmd)
}
func (lifecycleRedisHook) AfterProcess(context.Context, redis.Cmder) error { return nil }
func (lifecycleRedisHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}
func (lifecycleRedisHook) AfterProcessPipeline(context.Context, []redis.Cmder) error { return nil }

func TestReadDiscordGameState_DistinguishesMissingFromErrors(t *testing.T) {
	for _, name := range []string{"connect code", "text channel", "voice channel", "missing pointer", "missing state", "pointer error", "state error", "invalid JSON"} {
		t.Run(name, func(t *testing.T) {
			bot, deps := newTestBot(t)
			gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
			mr, client := withRedis(t, bot)
			key := rediskey.ConnectCodeData(gsr.GuildID, gsr.ConnectCode)
			pointer := rediskey.ConnectCodePtr(gsr.GuildID, gsr.ConnectCode)
			data, err := json.Marshal(deps.store.getCode(gsr.ConnectCode))
			if err != nil {
				t.Fatal(err)
			}
			mr.Set(pointer, key)
			mr.Set(rediskey.TextChannelPtr(gsr.GuildID, scenarioTextChannel), key)
			mr.Set(rediskey.VoiceChannelPtr(gsr.GuildID, trackedChannel), key)
			mr.Set(key, string(data))
			wantMissing, wantError := false, false
			switch name {
			case "text channel":
				gsr = GameStateRequest{GuildID: gsr.GuildID, TextChannel: scenarioTextChannel}
			case "voice channel":
				gsr = GameStateRequest{GuildID: gsr.GuildID, VoiceChannel: trackedChannel}
			case "missing pointer":
				mr.Del(pointer)
				wantMissing = true
			case "missing state":
				mr.Del(key)
				wantMissing = true
			case "pointer error", "state error":
				wantError = true
				faultKey := key
				if name == "pointer error" {
					faultKey = pointer
					// Even a valid fallback must not hide a failed pointer read.
					gsr.TextChannel = scenarioTextChannel
				}
				client.AddHook(lifecycleRedisHook{before: func(cmd redis.Cmder) error {
					if cmd.Name() == "get" && cmd.Args()[1] == faultKey {
						return errors.New("temporary Redis failure")
					}
					return nil
				}})
			case "invalid JSON":
				wantError = true
				mr.Set(key, "invalid JSON")
			}
			state, err := bot.RedisInterface.ReadDiscordGameState(gsr)
			if (err != nil) != wantError {
				t.Fatalf("read error = %v, want error: %v", err, wantError)
			}
			if wantMissing || wantError {
				if state != nil {
					t.Fatal("read returned a game for a missing or failed read")
				}
			} else if state == nil || state.ConnectCode != scenarioConnectCode {
				t.Fatalf("read state = %+v, want seeded game", state)
			}
		})
	}
}

type lifecycleReadStore struct {
	GameStateStore
	read func(GameStateRequest) (*GameState, error)
}

func (s lifecycleReadStore) ReadDiscordGameState(gsr GameStateRequest) (*GameState, error) {
	return s.read(gsr)
}

func TestConsumeQueue_StateReadFailureLeavesJobsForRetry(t *testing.T) {
	bot, deps := newTestBot(t)
	syncLogs(bot)
	gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	_, client := withRedis(t, bot)
	failed := true
	bot.store = lifecycleReadStore{GameStateStore: deps.store, read: func(gsr GameStateRequest) (*GameState, error) {
		if failed {
			return nil, errors.New("temporary Redis failure")
		}
		return deps.store.ReadDiscordGameState(gsr)
	}}
	if err := task.PushJob(ctx, client, gsr.ConnectCode, task.StateJob, phaseJob(game.LOBBY).Payload.(string)); err != nil {
		t.Fatal(err)
	}
	lease := bot.newConsumerLease(gsr.ConnectCode)
	stop := make(chan EndGameMessage, 1)
	consume := func() bool {
		return bot.consumeQueue(bot.gameLog(gsr), gsr.GuildID, gsr, lease, stop, func(EndGameMessage) { t.Error("unexpected end request") })
	}
	if consume() {
		t.Fatal("failed read closed the subscription")
	}
	if client.LLen(ctx, rediskey.JobNamespace+gsr.ConnectCode).Val() != 1 || lease.held() {
		t.Fatal("failed read consumed queued work or retained the lease")
	}
	failed = false
	if consume() || client.LLen(ctx, rediskey.JobNamespace+gsr.ConnectCode).Val() != 0 {
		t.Fatal("subscriber did not consume the backlog after recovery")
	}
	if deps.store.getCode(gsr.ConnectCode).GameData.GetPhase() != game.LOBBY {
		t.Fatal("recovered subscriber did not apply the queued event")
	}
}

func TestLifecycle_StateReadFailureDoesNotAdoptOrCleanUp(t *testing.T) {
	bot, deps := newTestBot(t)
	gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	failure := errors.New("temporary Redis failure")
	bot.store = lifecycleReadStore{GameStateStore: deps.store, read: func(GameStateRequest) (*GameState, error) { return nil, failure }}
	if bot.attachToGame(gsr, server.AdoptAnnounce) {
		t.Fatal("adopted a game whose state could not be read")
	}
	if err := bot.stopGame(gsr, server.EndReasonCriticalNotice, ""); !errors.Is(err, failure) {
		t.Fatalf("stop error = %v, want read failure", err)
	}
	if err := bot.completeGame(gsr, server.EndReasonCriticalNotice, ""); !errors.Is(err, failure) {
		t.Fatalf("cleanup error = %v, want read failure", err)
	}
	if state := deps.store.getCode(gsr.ConnectCode); state == nil || !state.Running || len(deps.voice.all()) != 0 {
		t.Fatal("failed read changed the game or issued voice changes")
	}
}

func TestInactivityCheck_RequiresSharedEvidence(t *testing.T) {
	for _, name := range []string{"recent activity", "queued work", "missing activity", "queue error", "activity error", "state error", "inactive"} {
		t.Run(name, func(t *testing.T) {
			bot, deps := newTestBot(t)
			syncLogs(bot)
			gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
			mr, client := withRedis(t, bot)
			activity := rediskey.ActiveGamesForGuild(gsr.GuildID)
			mr.ZAdd(activity, float64(time.Now().Add(-2*GameTimeoutSeconds*time.Second).Unix()), gsr.ConnectCode)
			switch name {
			case "recent activity":
				bot.RedisInterface.RefreshActiveGame(gsr.GuildID, gsr.ConnectCode)
			case "queued work":
				if err := task.PushJob(ctx, client, gsr.ConnectCode, task.ConnectionJob, "true"); err != nil {
					t.Fatal(err)
				}
			case "missing activity":
				mr.Del(activity)
			case "queue error":
				mr.Set(rediskey.JobNamespace+gsr.ConnectCode, "wrong Redis type")
			case "activity error":
				mr.Del(activity)
				mr.Set(activity, "wrong Redis type")
			case "state error":
				bot.store = lifecycleReadStore{GameStateStore: deps.store, read: func(GameStateRequest) (*GameState, error) { return nil, errors.New("temporary Redis failure") }}
			}
			lease := bot.newConsumerLease(gsr.ConnectCode)
			ended := bot.endGameIfInactive(gsr, lease)
			if ended != (name == "inactive") {
				t.Fatalf("ended = %v for %s", ended, name)
			}
			if lease.held() || client.Exists(ctx, lease.key).Val() != 0 {
				t.Fatal("inactivity check left the lease held")
			}
			if ended {
				if deps.store.getCode(gsr.ConnectCode) != nil || deps.metrics.endedBy(server.EndReasonInactivity) != 1 {
					t.Fatal("inactive game was not deleted and recorded as ended")
				}
				if users := unmutedUsers(deps); !users[10] || !users[11] {
					t.Fatalf("unmuted users = %v, want both players", users)
				}
			} else if state := deps.store.getCode(gsr.ConnectCode); state == nil || !state.Running || len(deps.voice.all()) != 0 {
				t.Fatal("unconfirmed inactivity changed the game")
			}
		})
	}
}

func TestInactivityCheck_RechecksActivityAfterBusyConsumer(t *testing.T) {
	bot, deps := newTestBot(t)
	syncLogs(bot)
	gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	mr, _ := withRedis(t, bot)
	mr.ZAdd(rediskey.ActiveGamesForGuild(gsr.GuildID), float64(time.Now().Add(-2*GameTimeoutSeconds*time.Second).Unix()), gsr.ConnectCode)
	holder := bot.newConsumerLease(gsr.ConnectCode)
	if !holder.acquire() {
		t.Fatal("foreign consumer could not acquire lease")
	}
	defer holder.release(false)
	standby := bot.newConsumerLease(gsr.ConnectCode)
	if bot.endGameIfInactive(gsr, standby) {
		t.Fatal("standby ended the game while its consumer was busy")
	}
	bot.RedisInterface.RefreshActiveGame(gsr.GuildID, gsr.ConnectCode)
	holder.release(false)
	if bot.endGameIfInactive(gsr, standby) || deps.store.getCode(gsr.ConnectCode) == nil {
		t.Fatal("standby ignored activity recorded before acquiring the lease")
	}
}

func TestInactivityTimer_KeepsSubscriptionAndRetriesAfterReadError(t *testing.T) {
	bot, deps := newTestBot(t)
	syncLogs(bot)
	gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	mr, client := withRedis(t, bot)
	bot.captureTimeout = 1
	// Recent activity from another replica must override this subscriber's timer.
	mr.ZAdd(rediskey.ActiveGamesForGuild(gsr.GuildID), float64(time.Now().Add(time.Minute).Unix()), gsr.ConnectCode)
	var fail atomic.Bool
	fail.Store(true)
	checked := make(chan bool, 10)
	client.AddHook(lifecycleRedisHook{before: func(cmd redis.Cmder) error {
		if cmd.Name() == "zscore" {
			failing := fail.Load()
			checked <- failing
			if failing {
				return errors.New("temporary Redis failure")
			}
		}
		return nil
	}})
	stop := make(chan EndGameMessage, 1)
	exited := make(chan struct{})
	bot.EndGameChannels[gsr.ConnectCode] = stop
	go func() {
		defer close(exited)
		bot.SubscribeToGameByConnectCode(gsr.GuildID, gsr.ConnectCode, stop)
	}()
	t.Cleanup(func() {
		select {
		case <-exited:
			return
		default:
		}
		stop <- EndGameMessage{reason: server.EndReasonCriticalNotice}
		select {
		case <-exited:
		case <-time.After(3 * time.Second):
			t.Error("subscriber did not exit during cleanup")
		}
	})
	for _, wantFailed := range []bool{true, false} {
		select {
		case failed := <-checked:
			if failed != wantFailed {
				t.Fatalf("activity check failed = %v, want %v", failed, wantFailed)
			}
		case <-exited:
			t.Fatal("local timer ended the subscription without confirming inactivity")
		case <-time.After(5 * time.Second):
			t.Fatal("inactivity check did not run or retry")
		}
		fail.Store(false)
	}
	eventually(t, "inactivity check to release its lease", func() bool { return client.Exists(ctx, leaseKey()).Val() == 0 })
	if deps.store.getCode(gsr.ConnectCode) == nil {
		t.Fatal("local timer ended a game with recent shared activity")
	}
	channel := rediskey.JobNamespace + gsr.ConnectCode + ":notify"
	if counts, err := client.PubSubNumSub(ctx, channel).Result(); err != nil || counts[channel] != 1 {
		t.Fatalf("subscription count = %v, error = %v; want one open subscription", counts, err)
	}
}
