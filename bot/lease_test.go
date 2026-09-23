package bot

import (
	"bytes"
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/go-redis/redis/v8"
)

// syncBuffer is a log sink safe to read while subscriber goroutines are still writing to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// syncLogs replaces a test bot's log sink with a synchronized one and returns it.
func syncLogs(bot *Bot) *syncBuffer {
	sb := &syncBuffer{}
	bot.log = slog.New(slog.NewTextHandler(sb, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return sb
}

func leaseKey() string { return rediskey.GameConsumerLease(scenarioConnectCode) }

func TestConsumerLease_ExclusiveRenewedAndReleased(t *testing.T) {
	bot, _ := newTestBot(t)
	mr, _ := withRedis(t, bot)

	a := bot.newConsumerLease(scenarioConnectCode)
	b := bot.newConsumerLease(scenarioConnectCode)
	if !a.acquire() || !a.held() {
		t.Fatal("first acquire should succeed")
	}
	if !a.acquire() {
		t.Fatal("re-acquiring a held lease should report held")
	}
	if b.acquire() || b.held() {
		t.Fatal("second process must not acquire a held lease")
	}
	if got, _ := mr.Get(leaseKey()); got != a.token {
		t.Fatalf("lease value = %q, want a's token", got)
	}
	if ttl := mr.TTL(leaseKey()); ttl != ConsumerLeaseTTL {
		t.Fatalf("lease TTL = %v, want %v", ttl, ConsumerLeaseTTL)
	}

	// the background renewer keeps it alive: advance most of the TTL, wait for a renewal, and it is still held
	mr.FastForward(ConsumerLeaseTTL - time.Second)
	time.Sleep(ConsumerLeaseTTL/3 + 100*time.Millisecond)
	if !mr.Exists(leaseKey()) {
		t.Fatal("lease expired despite renewal")
	}
	if ttl := mr.TTL(leaseKey()); ttl < ConsumerLeaseTTL-time.Second {
		t.Fatalf("lease not renewed, TTL = %v", ttl)
	}

	// release hands over: the key is gone, b takes it under its own token, and a's stale release cannot remove it
	a.release(false)
	if mr.Exists(leaseKey()) || a.held() {
		t.Fatal("release should delete the lease and drop the hold")
	}
	if !b.acquire() {
		t.Fatal("standby should acquire after release")
	}
	if got, _ := mr.Get(leaseKey()); got != b.token {
		t.Fatalf("lease value = %q, want b's token", got)
	}
	a.release(false)
	if got, _ := mr.Get(leaseKey()); got != b.token {
		t.Fatal("a stale release removed another holder's lease")
	}
	b.release(false)
}

// Every pop is fenced by the lease token, so a holder whose lease lapsed or was taken finds out at its next pop and
// leaves the queue to the new holder.
func TestConsumerLease_PopIsFencedByOwnership(t *testing.T) {
	bot, _ := newTestBot(t)
	mr, client := withRedis(t, bot)
	ctx := context.Background()
	logs := syncLogs(bot)
	queue := rediskey.JobNamespace + scenarioConnectCode

	a := bot.newConsumerLease(scenarioConnectCode)
	if _, owner, err := a.pop(); owner || err != nil {
		t.Fatalf("pop without the lease: owner=%v err=%v", owner, err)
	}
	if !a.acquire() {
		t.Fatal("acquire")
	}
	if entry, owner, err := a.pop(); entry != "" || !owner || err != nil {
		t.Fatalf("pop on empty queue = %q %v %v, want empty and owner", entry, owner, err)
	}
	for _, p := range []string{"first", "second"} {
		if err := task.PushJob(ctx, client, scenarioConnectCode, task.StateJob, p); err != nil {
			t.Fatal(err)
		}
	}
	entry, owner, err := a.pop()
	if err != nil || !owner {
		t.Fatalf("pop = %v %v", owner, err)
	}
	if job, _ := task.ParseJob(entry); job.Payload != "first" {
		t.Fatalf("popped %q, want the first job", entry)
	}

	// the lease lapses and another process takes it; a's next pop is refused and a drops its hold
	mr.FastForward(ConsumerLeaseTTL + time.Millisecond)
	b := bot.newConsumerLease(scenarioConnectCode)
	if !b.acquire() {
		t.Fatal("standby should acquire a lapsed lease")
	}
	if entry, owner, err := a.pop(); owner || entry != "" || err != nil {
		t.Fatalf("stale holder pop = %q %v %v, want refused", entry, owner, err)
	}
	if a.held() {
		t.Fatal("stale holder should have dropped its hold")
	}
	if !strings.Contains(logs.String(), "consumer lease lost") {
		t.Fatal("loss should be logged")
	}
	if entries, _ := mr.List(queue); len(entries) != 1 {
		t.Fatalf("queue = %v, want the second job left for the new holder", entries)
	}
	if entry, owner, _ := b.pop(); !owner || !strings.Contains(entry, "second") {
		t.Fatalf("new holder pop = %q %v", entry, owner)
	}
	b.release(false)
}

// twinSetup runs two subscribers for one game on a shared store and Redis, as twin pods do, and returns their logs.
func twinSetup(t *testing.T) (first, second *Bot, deps *testDeps, firstLogs, secondLogs *syncBuffer, client *redis.Client) {
	t.Helper()
	first, deps = newTestBot(t)
	firstLogs = syncLogs(first)
	seedMatch(t, first, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	dgs := deps.store.getCode(scenarioConnectCode)
	dgs.GameData.UpdatePhase(game.LOBBY)
	deps.store.put(dgs)
	_, client = withRedis(t, first)

	second, _ = newTestBot(t)
	secondLogs = syncLogs(second)
	second.store = deps.store
	second.guilds = deps.guilds
	second.RedisInterface = &RedisInterface{client: client}

	ctx := context.Background()
	ack := task.AckSubscribe(ctx, client, scenarioConnectCode)
	if _, err := ack.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ack.Close() })
	for _, b := range []*Bot{first, second} {
		stop := make(chan EndGameMessage, 1)
		b.EndGameChannels[scenarioConnectCode] = stop
		go b.SubscribeToGameByConnectCode(scenarioGuild, scenarioConnectCode, stop)
		select {
		case <-ack.Channel():
		case <-time.After(2 * time.Second):
			t.Fatal("subscriber never acked")
		}
	}
	t.Cleanup(func() {
		stopSubscriber(t, first, scenarioConnectCode)
		stopSubscriber(t, second, scenarioConnectCode)
	})
	return
}

func nudgeGame(client *redis.Client) {
	client.Publish(context.Background(), rediskey.JobNamespace+scenarioConnectCode+":notify", "1")
}

func countReceived(l *syncBuffer) int {
	return strings.Count(l.String(), `msg="capture event received"`)
}

// Two subscribers to the same game must apply its events one at a time and in order: only the lease holder pops,
// the lease is released between bursts, and a drain hands the queue to the standby with no gap.
func TestTwins_OnlyLeaseHolderConsumesAndDrainHandsOver(t *testing.T) {
	first, second, deps, firstLogs, secondLogs, client := twinSetup(t)
	ctx := context.Background()
	phase := func() game.Phase { return deps.store.getCode(scenarioConnectCode).GameData.GetPhase() }

	// one twin takes the lease for the burst and applies the event; the other stands by
	if err := task.PushJob(ctx, client, scenarioConnectCode, task.StateJob, strconv.Itoa(int(game.TASKS))); err != nil {
		t.Fatal(err)
	}
	eventually(t, "first event to be applied", func() bool { nudgeGame(client); return phase() == game.TASKS })
	time.Sleep(50 * time.Millisecond)
	holder, holderLogs, standbyLogs := first, firstLogs, secondLogs
	if countReceived(firstLogs) == 0 {
		holder, holderLogs, standbyLogs = second, secondLogs, firstLogs
	}
	if countReceived(holderLogs) != 1 || countReceived(standbyLogs) != 0 {
		t.Fatalf("events received: holder=%d standby=%d, want 1 and 0", countReceived(holderLogs), countReceived(standbyLogs))
	}
	// The key flickers while the twins drain the nudges queued during the wait above, so poll rather than
	// read once: the property is that the burst ends with the lease released, not that it is free at one instant.
	eventually(t, "lease to be released between bursts", func() bool {
		return first.RedisInterface.client.Exists(ctx, leaseKey()).Val() == 0
	})

	// draining the holder: the standby applies the next event, the holder receives nothing more
	holder.Drain()
	if err := task.PushJob(ctx, client, scenarioConnectCode, task.StateJob, strconv.Itoa(int(game.DISCUSS))); err != nil {
		t.Fatal(err)
	}
	eventually(t, "standby to apply the next event", func() bool { nudgeGame(client); return phase() == game.DISCUSS })
	time.Sleep(50 * time.Millisecond)
	if countReceived(standbyLogs) != 1 {
		t.Fatalf("standby received %d events after handover, want 1", countReceived(standbyLogs))
	}
	if countReceived(holderLogs) != 1 {
		t.Fatalf("draining holder received %d events, want it to stop at 1", countReceived(holderLogs))
	}
}

// While another process holds the lease, neither subscriber here consumes; once it lapses, the backlog is picked up.
func TestTwins_ForeignLeaseHolderBlocksConsumptionUntilItLapses(t *testing.T) {
	_, _, deps, firstLogs, secondLogs, client := twinSetup(t)
	ctx := context.Background()
	phase := func() game.Phase { return deps.store.getCode(scenarioConnectCode).GameData.GetPhase() }

	client.Set(ctx, leaseKey(), "another-process", 0)
	if err := task.PushJob(ctx, client, scenarioConnectCode, task.StateJob, strconv.Itoa(int(game.TASKS))); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		nudgeGame(client)
		time.Sleep(20 * time.Millisecond)
	}
	if phase() != game.LOBBY || countReceived(firstLogs)+countReceived(secondLogs) != 0 {
		t.Fatal("subscribers consumed while another process held the lease")
	}

	client.Del(ctx, leaseKey())
	eventually(t, "backlog to be consumed once the lease is free", func() bool { nudgeGame(client); return phase() == game.TASKS })
}

// Ending a game unmutes everyone and deletes its state, so it must wait for the current burst on whichever process
// holds the lease; once ended, the other subscriber notices the game is gone and closes.
func TestTwins_EndingWaitsForLeaseAndClosesOtherSubscriber(t *testing.T) {
	first, second, deps, _, _, client := twinSetup(t)
	ctx := context.Background()

	client.Set(ctx, leaseKey(), "busy-process", 0)
	first.ChannelsMapLock.RLock()
	stop := first.EndGameChannels[scenarioConnectCode]
	first.ChannelsMapLock.RUnlock()
	done := make(chan error, 1)
	stop <- EndGameMessage{reason: "maintenance", done: done}

	time.Sleep(300 * time.Millisecond)
	if deps.store.getCode(scenarioConnectCode) == nil {
		t.Fatal("game was ended while another process held the lease")
	}
	select {
	case <-done:
		t.Fatal("end completed while another process held the lease")
	default:
	}

	client.Del(ctx, leaseKey())
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("end: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("end did not complete once the lease was free")
	}
	if deps.store.getCode(scenarioConnectCode) != nil {
		t.Fatal("game state should be deleted")
	}
	if _, waits, _ := first.metrics.(*fakeMetrics).leaseCounts(); waits != 1 {
		t.Fatalf("lease waits = %d, want 1: the end request found the lease taken", waits)
	}
	eventually(t, "the other subscriber to close", func() bool {
		second.ChannelsMapLock.RLock()
		defer second.ChannelsMapLock.RUnlock()
		_, still := second.EndGameChannels[scenarioConnectCode]
		return !still
	})
}

// A job is counted as in flight from before it is taken off the queue, so a drain cannot close the process between
// the pop and the apply.
func TestConsumeQueue_CountsWorkInFlightBeforePopping(t *testing.T) {
	bot, deps := newTestBot(t)
	syncLogs(bot)
	seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	dgs := deps.store.getCode(scenarioConnectCode)
	dgs.GameData.UpdatePhase(game.LOBBY)
	deps.store.put(dgs)
	_, client := withRedis(t, bot)
	ctx := context.Background()

	deps.settings.SetDelay(game.LOBBY, game.TASKS, 3)
	delaying, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	t.Cleanup(release)
	bot.sleep = func(d time.Duration) {
		if d == 3*time.Second {
			close(delaying)
			<-resume
		}
	}
	ack := task.AckSubscribe(ctx, client, scenarioConnectCode)
	if _, err := ack.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	defer ack.Close()
	stop := make(chan EndGameMessage, 1)
	bot.EndGameChannels[scenarioConnectCode] = stop
	go bot.SubscribeToGameByConnectCode(scenarioGuild, scenarioConnectCode, stop)
	select {
	case <-ack.Channel():
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber never acked")
	}

	if err := task.PushJob(ctx, client, scenarioConnectCode, task.StateJob, strconv.Itoa(int(game.TASKS))); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		nudgeGame(client)
		select {
		case <-delaying:
		case <-time.After(50 * time.Millisecond):
			if time.Now().Before(deadline) {
				continue
			}
			t.Fatal("job never reached its delay")
		}
		break
	}
	if n := bot.inflight.Load(); n != 1 {
		t.Fatalf("inflight = %d while a job is being applied, want 1", n)
	}
	release()
	eventually(t, "job to finish", func() bool { return bot.inflight.Load() == 0 })
	stopSubscriber(t, bot, scenarioConnectCode)
}
