package bot

import (
	"context"
	"errors"
	"testing"
	"time"

	redis_common "github.com/automuteus/automuteus/v8/common"
	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/go-redis/redis/v8"
)

func TestRateLimitReservation_ExpiredReleasePreservesNewReservation(t *testing.T) {
	bot, _ := newTestBot(t)
	mr, client := withRedis(t, bot)
	old := &rateLimitReservation{client: client}
	old.reserve("42", "info", time.Second)

	// The first request is still awaiting Discord when its reservation expires
	// and a second interaction from the same user is admitted.
	mr.FastForward(2 * time.Second)
	current := &rateLimitReservation{client: client}
	current.reserve("42", "info", time.Second)
	if !redis_common.IsUserRateLimitedGeneral(client, "42") || !redis_common.IsUserRateLimitedSpecific(client, "42", "info") {
		t.Fatal("second interaction did not reserve both rate limits")
	}

	old.release() // the first request's delayed response fails
	if !redis_common.IsUserRateLimitedGeneral(client, "42") {
		t.Error("a delayed failed response removed the newer interaction's general reservation")
	}
	if !redis_common.IsUserRateLimitedSpecific(client, "42", "info") {
		t.Error("a delayed failed response removed the newer interaction's command reservation")
	}
}

func TestSubscriber_CleanupTimeoutPreservesGameOwnedByAnotherConsumer(t *testing.T) {
	bot, deps := newTestBot(t)
	syncLogs(bot)
	seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	_, client := withRedis(t, bot)
	holder := bot.newConsumerLease(scenarioConnectCode)
	if !holder.acquire() {
		t.Fatal("foreign consumer could not acquire the lease")
	}

	stop := make(chan EndGameMessage, 1)
	done := make(chan error, 1)
	exited := make(chan struct{})
	bot.EndGameChannels[scenarioConnectCode] = stop
	go func() {
		defer close(exited)
		bot.SubscribeToGameByConnectCode(scenarioGuild, scenarioConnectCode, stop)
	}()
	t.Cleanup(func() {
		holder.release(false)
		// Allow either a pending end request or a subsequent cleanup request to
		// finish once the foreign owner yields. Never leave an unbounded wait.
		select {
		case <-exited:
			return
		default:
		}
		select {
		case stop <- EndGameMessage{reason: "test cleanup"}:
		default:
		}
		select {
		case <-exited:
		case <-time.After(3 * time.Second):
			t.Error("subscriber did not exit after the foreign owner released its lease")
		}
	})
	stop <- EndGameMessage{reason: "maintenance", done: done}

	// Keep the foreign holder renewing beyond the finisher's acquisition
	// deadline. Safe behavior is to report failure or leave cleanup pending.
	select {
	case err := <-done:
		if err == nil {
			t.Error("cleanup reported success while another consumer still owns the game")
		}
	case <-time.After(ConsumerLeaseTTL + 2*time.Second):
	}
	if token, err := client.Get(ctx, holder.key).Result(); err != nil || token != holder.token {
		t.Fatalf("foreign lease = %q, %v; want the renewing owner's token", token, err)
	}
	if deps.store.getCode(scenarioConnectCode) == nil {
		t.Fatal("cleanup deleted the game after timing out while a foreign owner kept renewing its lease")
	}
}

// beforeLeaseAcquireHook schedules another consumer's cleanup immediately before
// this consumer's SET NX reaches Redis. It makes the existence-check/acquisition
// interleaving deterministic without relying on goroutine scheduling or sleeps.
type beforeLeaseAcquireHook struct {
	key    string
	token  string
	before func()
}

func (h *beforeLeaseAcquireHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	args := cmd.Args()
	if cmd.Name() == "set" && len(args) >= 3 && args[1] == h.key && (h.token == "" || args[2] == h.token) && h.before != nil {
		before := h.before
		h.before = nil
		before()
	}
	return ctx, nil
}

func (*beforeLeaseAcquireHook) AfterProcess(context.Context, redis.Cmder) error { return nil }

func (*beforeLeaseAcquireHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (*beforeLeaseAcquireHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func TestConsumeQueue_GameEndedBeforeLeaseAcquisitionIsNotRecreated(t *testing.T) {
	bot, deps := newTestBot(t)
	syncLogs(bot)
	seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	_, client := withRedis(t, bot)
	holder := bot.newConsumerLease(scenarioConnectCode)
	if !holder.acquire() {
		t.Fatal("foreign consumer could not acquire the lease")
	}
	defer holder.release(false)

	gsr := GameStateRequest{GuildID: scenarioGuild, ConnectCode: scenarioConnectCode}
	consumer := bot.newConsumerLease(scenarioConnectCode)
	defer consumer.release(false)
	ended := false
	client.AddHook(&beforeLeaseAcquireHook{
		key: consumer.key, token: consumer.token,
		before: func() {
			// The owning consumer ends the game and yields just before this
			// subscriber acquires. A read before acquisition is now stale.
			deps.store.DeleteDiscordGameState(deps.store.getCode(scenarioConnectCode))
			holder.release(false)
			ended = true
		},
	})
	if err := task.PushJob(ctx, client, scenarioConnectCode, task.ConnectionJob, "true"); err != nil {
		t.Fatal(err)
	}
	finished := bot.consumeQueue(bot.gameLog(gsr), scenarioGuild, gsr, consumer, make(chan EndGameMessage, 1), func(EndGameMessage) {
		t.Error("unexpected local end request")
	})
	if !ended {
		t.Fatal("test did not exercise cleanup before lease acquisition")
	}
	if deps.store.getCode(scenarioConnectCode) != nil {
		t.Error("subscriber recreated a game ended between its existence check and lease acquisition")
	}
	if !finished {
		t.Error("subscriber should exit after discovering the game ended")
	}
	if remaining, err := client.LLen(ctx, rediskey.JobNamespace+scenarioConnectCode).Result(); err != nil || remaining != 1 {
		t.Errorf("remaining queued jobs = %d, %v; must not pop an event for an ended game", remaining, err)
	}
}

func TestStopGame_WithoutLocalSubscriberWaitsForConsumerLease(t *testing.T) {
	for _, endedElsewhere := range []bool{false, true} {
		name := "active game"
		if endedElsewhere {
			name = "game ended while waiting"
		}
		t.Run(name, func(t *testing.T) {
			bot, deps := newTestBot(t)
			syncLogs(bot)
			gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
			_, client := withRedis(t, bot)
			holder := bot.newConsumerLease(gsr.ConnectCode)
			if !holder.acquire() {
				t.Fatal("foreign consumer could not acquire lease")
			}

			attempted := make(chan struct{})
			client.AddHook(&beforeLeaseAcquireHook{key: holder.key, before: func() { close(attempted) }})
			done := make(chan error, 1)
			exited := make(chan struct{})
			go func() {
				defer close(exited)
				// Commands identify the game by channel, without a connect code.
				done <- bot.stopGame(GameStateRequest{GuildID: gsr.GuildID, TextChannel: scenarioTextChannel}, server.EndReasonCriticalNotice, "")
			}()
			t.Cleanup(func() {
				holder.release(false)
				select {
				case <-exited:
				case <-time.After(3 * time.Second):
					t.Error("cleanup did not finish after lease release")
				}
			})

			select {
			case err := <-done:
				t.Fatalf("cleanup returned %v while another consumer owns the lease", err)
			case <-attempted:
			case <-time.After(2 * time.Second):
				t.Fatal("cleanup did not attempt to acquire the consumer lease")
			}
			if dgs := deps.store.getCode(gsr.ConnectCode); dgs == nil || !dgs.Running {
				t.Fatal("cleanup modified the game before acquiring its lease")
			}
			if len(deps.voice.all()) != 0 {
				t.Fatal("cleanup unmuted players while another consumer owns the lease")
			}
			if endedElsewhere {
				deps.store.DeleteDiscordGameState(deps.store.getCode(gsr.ConnectCode))
			}
			holder.release(false)
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("cleanup did not finish after lease release")
			}
			if deps.store.getCode(gsr.ConnectCode) != nil {
				t.Fatal("game still exists after cleanup")
			}
			if client.Exists(ctx, holder.key).Val() != 0 {
				t.Fatal("cleanup left the consumer lease held")
			}
			if endedElsewhere {
				if len(deps.voice.all()) != 0 || deps.metrics.endedBy(server.EndReasonCriticalNotice) != 0 {
					t.Fatal("cleanup repeated effects for a game ended by another consumer")
				}
			} else if users := unmutedUsers(deps); !users[10] || !users[11] {
				t.Fatalf("unmuted users = %v, want both players", users)
			}
		})
	}
}

func TestSubscriber_GameEndedElsewhereClosesRedisSubscription(t *testing.T) {
	bot, deps := newTestBot(t)
	syncLogs(bot)
	gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	_, client := withRedis(t, bot)
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
			t.Error("subscriber did not exit")
		}
	})
	channel := rediskey.JobNamespace + gsr.ConnectCode + ":notify"
	eventually(t, "subscriber to attach", func() bool {
		return client.PubSubNumSub(ctx, channel).Val()[channel] == 1
	})
	deps.store.DeleteDiscordGameState(deps.store.getCode(gsr.ConnectCode))
	task.Notify(ctx, client, gsr.ConnectCode)
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("subscriber did not exit after the game ended elsewhere")
	}
	// Closing the client and Redis reading EOF happen asynchronously.
	eventually(t, "Redis subscription to close", func() bool {
		return client.PubSubNumSub(ctx, channel).Val()[channel] == 0
	})
}

func TestStopGame_WithoutRedisPreservesGame(t *testing.T) {
	bot, deps := newTestBot(t)
	gsr := seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	if err := bot.stopGame(gsr, server.EndReasonCriticalNotice, ""); !errors.Is(err, errNoRedis) {
		t.Fatalf("cleanup error = %v, want missing Redis client", err)
	}
	if dgs := deps.store.getCode(gsr.ConnectCode); dgs == nil || !dgs.Running || len(deps.voice.all()) != 0 {
		t.Fatal("cleanup changed the game without acquiring a consumer lease")
	}
}
