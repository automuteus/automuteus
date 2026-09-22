package bot

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redis_common "github.com/automuteus/automuteus/v8/common"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
)

// withRedis gives the test bot a real (miniredis-backed) Redis client for the code paths that bypass the store seam:
// capture job subscriptions and platform notices.
func withRedis(t *testing.T, bot *Bot) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	bot.RedisInterface = &RedisInterface{client: client}
	return mr, client
}

// stopSubscriber ends a running capture subscription and waits for it to unregister.
func stopSubscriber(t *testing.T, bot *Bot, connectCode string) {
	t.Helper()
	bot.ChannelsMapLock.RLock()
	stop, ok := bot.EndGameChannels[connectCode]
	bot.ChannelsMapLock.RUnlock()
	if !ok {
		return
	}
	select {
	case stop <- EndGameMessage{reason: "test cleanup"}:
	case <-time.After(time.Second):
		t.Fatal("subscriber did not accept the stop request")
	}
	eventually(t, "subscriber to exit", func() bool {
		bot.ChannelsMapLock.RLock()
		defer bot.ChannelsMapLock.RUnlock()
		_, still := bot.EndGameChannels[connectCode]
		return !still
	})
}

func TestHandleInteractionCreate_DrainingRefusesBeforeTouchingAnything(t *testing.T) {
	bot, _ := newTestBot(t)
	bot.Drain()

	// a nil session and no Redis client would panic if the handler went any further
	bot.handleInteractionCreate(nil, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{ID: "1", GuildID: scenarioGuild}})

	if n := bot.inflight.Load(); n != 0 {
		t.Fatalf("inflight = %d after a refused interaction", n)
	}
}

func TestHandleVoiceStateChange_DrainingLeavesEventToTwin(t *testing.T) {
	bot, deps := newTestBot(t)
	dgs := runningGame(deps, game.TASKS, inChannel("10", trackedChannel))
	addLinkedUser(dgs, "10", "alice", true, true, true)
	addLinkedUser(dgs, "11", "bob", true, false, false)
	deps.store.put(dgs)
	bot.Drain()

	bot.handleVoiceStateChange(nil, &discordgo.VoiceStateUpdate{VoiceState: &discordgo.VoiceState{
		GuildID: scenarioGuild, ChannelID: trackedChannel, UserID: "11", SessionID: "sess",
	}})

	if reqs := deps.voice.all(); len(reqs) != 0 {
		t.Fatalf("draining shard issued voice changes: %+v", reqs)
	}
	if u, _ := deps.store.get().GetUser("11"); u.ShouldBeMute {
		t.Fatal("draining shard modified game state")
	}
}

func TestDrain_IsIdempotentAndCountsInflight(t *testing.T) {
	bot, _ := newTestBot(t)
	if bot.Draining() {
		t.Fatal("new bot should not be draining")
	}
	bot.Drain()
	bot.Drain()
	if !bot.Draining() {
		t.Fatal("expected draining")
	}

	done := bot.beginWork()
	start := time.Now()
	if n := bot.WaitForInflight(50 * time.Millisecond); n != 1 {
		t.Fatalf("WaitForInflight = %d with one handler running, want 1", n)
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("WaitForInflight returned before its timeout with work still in flight")
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		done()
	}()
	if n := bot.WaitForInflight(time.Second); n != 0 {
		t.Fatalf("WaitForInflight = %d after the handler finished, want 0", n)
	}
}

func TestDrain_AnnouncesRunningGames(t *testing.T) {
	bot, deps := newTestBot(t)
	first, second := seedTwoMatches(t, bot, deps)
	_, client := withRedis(t, bot)
	sub := notice.Subscribe(context.Background(), client)
	if _, err := sub.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	bot.Drain()

	select {
	case msg := <-sub.Channel():
		e, err := notice.DecodeEvent([]byte(msg.Payload))
		if err != nil {
			t.Fatal(err)
		}
		if e.GamesAvailable == nil || len(e.GamesAvailable.Games) != 2 {
			t.Fatalf("event = %+v, want both games announced", e)
		}
		got := map[string]string{}
		for _, g := range e.GamesAvailable.Games {
			got[g.ConnectCode] = g.GuildID
		}
		if got[first.ConnectCode] != first.GuildID || got[second.ConnectCode] != second.GuildID {
			t.Fatalf("handoff games = %v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no games announced")
	}
}

func TestSubscriber_DrainingLeavesQueuedJobsForAdopter(t *testing.T) {
	bot, deps := newTestBot(t)
	seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	dgs := deps.store.getCode(scenarioConnectCode)
	dgs.GameData.UpdatePhase(game.LOBBY)
	deps.store.put(dgs)
	mr, client := withRedis(t, bot)
	ctx := context.Background()

	// the subscriber acks once it is listening; wait for that so no notify is lost
	ack := task.AckSubscribe(ctx, client, scenarioConnectCode)
	if _, err := ack.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	defer ack.Close()
	stop := make(chan EndGameMessage, 1)
	bot.EndGameChannels[scenarioConnectCode] = stop
	go bot.SubscribeToGameByConnectCode(scenarioGuild, scenarioConnectCode, stop)
	defer stopSubscriber(t, bot, scenarioConnectCode)
	select {
	case <-ack.Channel():
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber never acked")
	}

	// nudge re-publishes the subscriber's wake-up, in case the first notify raced the subscription itself
	nudge := func() { client.Publish(ctx, rediskey.JobNamespace+scenarioConnectCode+":notify", "1") }

	// control: while serving, a phase change is consumed and applied
	if err := task.PushJob(ctx, client, scenarioConnectCode, task.StateJob, strconv.Itoa(int(game.TASKS))); err != nil {
		t.Fatal(err)
	}
	eventually(t, "job to be consumed", func() bool {
		nudge()
		return deps.store.getCode(scenarioConnectCode).GameData.GetPhase() == game.TASKS
	})
	queue := rediskey.JobNamespace + scenarioConnectCode
	if mr.Exists(queue) {
		t.Fatal("consumed job should have left the queue")
	}

	// draining: the next job stays queued for whichever process adopts the game
	bot.Drain()
	if err := task.PushJob(ctx, client, scenarioConnectCode, task.StateJob, strconv.Itoa(int(game.DISCUSS))); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		nudge()
		time.Sleep(20 * time.Millisecond)
	}
	entries, err := mr.List(queue)
	if err != nil || len(entries) != 1 {
		t.Fatalf("queue = %v, %v; want the job left in place", entries, err)
	}
	if got := deps.store.getCode(scenarioConnectCode).GameData.GetPhase(); got != game.TASKS {
		t.Fatalf("phase = %v; draining shard must not apply queued jobs", got)
	}
}

func TestHandleEvent_AnnouncedGamesAreAdoptedForGuildsOnThisShard(t *testing.T) {
	bot, deps := newTestBot(t)
	seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	// a game in a guild this shard does not serve
	other := NewDiscordGameState("999")
	other.ConnectCode = "OTHERCOD"
	deps.store.put(other)
	withRedis(t, bot)

	bot.handleEvent(&notice.Event{GamesAvailable: &notice.GamesAvailable{Games: []notice.GameRef{
		{GuildID: scenarioGuild, ConnectCode: scenarioConnectCode},
		{GuildID: "999", ConnectCode: "OTHERCOD"},
		{GuildID: scenarioGuild, ConnectCode: "GONE0000"}, // no longer in the store
	}}})
	defer stopSubscriber(t, bot, scenarioConnectCode)

	bot.ChannelsMapLock.RLock()
	stop, ours := bot.EndGameChannels[scenarioConnectCode]
	_, foreign := bot.EndGameChannels["OTHERCOD"]
	_, gone := bot.EndGameChannels["GONE0000"]
	bot.ChannelsMapLock.RUnlock()
	if !ours {
		t.Fatal("game in a served guild was not adopted")
	}
	if foreign {
		t.Fatal("game in a guild served by another shard was adopted")
	}
	if gone {
		t.Fatal("game missing from the store was adopted")
	}
	if deps.store.getCode("GONE0000") != nil {
		t.Fatal("adoption must not create state for a missing game")
	}

	// a repeated announcement for a game already attached changes nothing
	bot.handleEvent(&notice.Event{GamesAvailable: &notice.GamesAvailable{Games: []notice.GameRef{
		{GuildID: scenarioGuild, ConnectCode: scenarioConnectCode},
	}}})
	bot.ChannelsMapLock.RLock()
	again := bot.EndGameChannels[scenarioConnectCode]
	bot.ChannelsMapLock.RUnlock()
	if again != stop {
		t.Fatal("repeated announcement replaced the existing subscription")
	}
}

func TestHandleEvent_DrainingShardAdoptsNothing(t *testing.T) {
	bot, deps := newTestBot(t)
	seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	withRedis(t, bot)
	bot.Drain()

	bot.handleEvent(&notice.Event{GamesAvailable: &notice.GamesAvailable{Games: []notice.GameRef{
		{GuildID: scenarioGuild, ConnectCode: scenarioConnectCode},
	}}})

	bot.ChannelsMapLock.RLock()
	defer bot.ChannelsMapLock.RUnlock()
	if _, attached := bot.EndGameChannels[scenarioConnectCode]; attached {
		t.Fatal("a draining shard must not adopt games")
	}
}

func TestRateLimitReservation_ReservedAtAdmissionLiftedOnFailure(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	var none *rateLimitReservation
	none.reserve("42", "info", time.Second)
	none.release()
	(&rateLimitReservation{}).reserve("42", "info", time.Second) // no Redis: nothing to do, no panic
	(&rateLimitReservation{client: client}).release()            // nothing reserved: no-op
	if keys := mr.Keys(); len(keys) != 0 {
		t.Fatalf("nothing reserved, but keys were written: %v", keys)
	}

	r := &rateLimitReservation{client: client}
	r.reserve("42", "info", 3*time.Second)
	if !redis_common.IsUserRateLimitedGeneral(client, "42") || !redis_common.IsUserRateLimitedSpecific(client, "42", "info") {
		t.Fatal("reservation should mark both rate limits immediately")
	}
	// while the interaction runs, the reservation guards for longer than the command's cooldown
	if ttl := mr.TTL(redis_common.UserRateLimitSpecificKey("42", "info")); ttl != reservationGuardTTL {
		t.Fatalf("guard TTL = %v, want %v", ttl, reservationGuardTTL)
	}

	// a response that reached Discord shortens it to the normal cooldowns
	r.commit()
	if ttl := mr.TTL(redis_common.UserRateLimitSpecificKey("42", "info")); ttl != 3*time.Second {
		t.Fatalf("command cooldown after commit = %v, want 3s", ttl)
	}
	if ttl := mr.TTL(redis_common.UserRateLimitGeneralKey("42")); ttl != redis_common.GlobalUserRateLimitDuration {
		t.Fatalf("general cooldown after commit = %v, want %v", ttl, redis_common.GlobalUserRateLimitDuration)
	}
	r.release() // after commit there is nothing to lift
	if !redis_common.IsUserRateLimitedGeneral(client, "42") {
		t.Fatal("release after commit must not lift the cooldown")
	}

	// a dropped interaction lifts both keys
	r2 := &rateLimitReservation{client: client}
	r2.reserve("43", "info", 3*time.Second)
	r2.release()
	if redis_common.IsUserRateLimitedGeneral(client, "43") || redis_common.IsUserRateLimitedSpecific(client, "43", "info") {
		t.Fatal("a dropped interaction should lift both rate limits")
	}
	r2.release() // second release is a no-op
}

func TestDiscoverGamesOnce_AttachesRecentGamesInServedGuilds(t *testing.T) {
	bot, deps := newTestBot(t)
	seedMatch(t, bot, deps, scenarioConnectCode, scenarioTextChannel, trackedChannel, "10", "11")
	other := NewDiscordGameState("999")
	other.ConnectCode = "OTHERCOD"
	deps.store.put(other)
	mr, _ := withRedis(t, bot)

	bot.RedisInterface.RefreshActiveGame(scenarioGuild, scenarioConnectCode)
	bot.RedisInterface.RefreshActiveGame("999", "OTHERCOD")
	// a game whose last activity predates the timeout is not discovered
	mr.ZAdd(rediskey.ActiveGamesByGuildZSet, float64(time.Now().Add(-(GameTimeoutSeconds+1)*time.Second).Unix()), scenarioGuild+":STALE000")

	if n := bot.discoverGamesOnce(); n != 1 {
		t.Fatalf("adopted %d games, want 1", n)
	}
	defer stopSubscriber(t, bot, scenarioConnectCode)
	bot.ChannelsMapLock.RLock()
	_, ours := bot.EndGameChannels[scenarioConnectCode]
	_, foreign := bot.EndGameChannels["OTHERCOD"]
	_, stale := bot.EndGameChannels["STALE000"]
	bot.ChannelsMapLock.RUnlock()
	if !ours || foreign || stale {
		t.Fatalf("attached: ours=%v foreign=%v stale=%v", ours, foreign, stale)
	}
	if n := bot.discoverGamesOnce(); n != 0 {
		t.Fatalf("second discovery adopted %d games, want 0", n)
	}
	bot.Drain()
	if n := bot.discoverGamesOnce(); n != 0 {
		t.Fatal("a draining shard must not discover games")
	}
}

func TestAnnounceGame_PublishesOneGame(t *testing.T) {
	bot, _ := newTestBot(t)
	_, client := withRedis(t, bot)
	sub := notice.Subscribe(context.Background(), client)
	if _, err := sub.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	bot.announceGame(scenarioGuild, scenarioConnectCode)

	select {
	case msg := <-sub.Channel():
		e, err := notice.DecodeEvent([]byte(msg.Payload))
		if err != nil {
			t.Fatal(err)
		}
		if e.GamesAvailable == nil || len(e.GamesAvailable.Games) != 1 || e.GamesAvailable.Games[0] != (notice.GameRef{GuildID: scenarioGuild, ConnectCode: scenarioConnectCode}) {
			t.Fatalf("event = %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("new game not announced")
	}
}
