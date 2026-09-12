package tokenprovider

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
)

// These tests pin down how ModifyUsers routes mute/deafen requests between the capture client and the bot's own
// tokens, and in particular what happens when the capture client cannot apply mutes, which is the common case for
// any game whose capture app has no bot token configured. Every user must end up muted regardless; the benchmarks
// measure how quickly and cheaply that happens.

const (
	testGuild = "1"
	testCode  = "ABCDEFGH"
)

// commandCounter is a go-redis hook that counts commands by name, so a test can see how many capture tasks were
// published and how many blacklist writes were made.
type commandCounter struct {
	mu     sync.Mutex
	counts map[string]int
}

func (c *commandCounter) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	c.add(cmd)
	return ctx, nil
}

func (c *commandCounter) AfterProcess(context.Context, redis.Cmder) error { return nil }

func (c *commandCounter) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	for _, cmd := range cmds {
		c.add(cmd)
	}
	return ctx, nil
}

func (c *commandCounter) AfterProcessPipeline(context.Context, []redis.Cmder) error { return nil }

func (c *commandCounter) add(cmd redis.Cmder) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.counts == nil {
		c.counts = map[string]int{}
	}
	c.counts[strings.ToLower(cmd.Name())]++
}

func (c *commandCounter) get(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[name]
}

func (c *commandCounter) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts = map[string]int{}
}

// primaryRecorder stands in for the primary bot's Discord API call.
type primaryRecorder struct {
	mu    sync.Mutex
	calls []task.UserModify
}

func (p *primaryRecorder) apply(_, userID string, mute, deaf bool) error {
	uid, _ := strconv.ParseUint(userID, 10, 64)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, task.UserModify{UserID: uid, Mute: mute, Deaf: deaf})
	return nil
}

func (p *primaryRecorder) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

func (p *primaryRecorder) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = nil
}

type harness struct {
	tp      *TokenProvider
	redis   *miniredis.Miniredis
	client  *redis.Client // the provider's client; commands are counted
	admin   *redis.Client // for test setup; commands are not counted
	cmds    *commandCounter
	primary *primaryRecorder
}

func newHarness(tb testing.TB, ackTimeout time.Duration) *harness {
	tb.Helper()
	// the provider logs through the default logger; keep test and benchmark output readable
	prevLog := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	tb.Cleanup(func() { slog.SetDefault(prevLog) })

	mr := miniredis.RunT(tb)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	admin := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tb.Cleanup(func() { _ = client.Close(); _ = admin.Close() })
	cmds := &commandCounter{}
	client.AddHook(cmds)

	primary := &primaryRecorder{}
	// the per-game rate limit is not what most of these tests exercise; make it effectively unlimited
	tp := NewTokenProvider(client, nil, ackTimeout, 1<<40)
	tp.applyPrimary = primary.apply
	return &harness{tp: tp, redis: mr, client: client, admin: admin, cmds: cmds, primary: primary}
}

// captureReady does what galactus does when a capture client reports it can apply mutes.
func (h *harness) captureReady(tb testing.TB) {
	tb.Helper()
	if err := h.admin.Set(context.Background(), rediskey.CaptureMuteReady(testCode), "1", time.Hour).Err(); err != nil {
		tb.Fatal(err)
	}
}

// startFakeCaptureClient plays the role of galactus plus a capture app that acks every task: it marks itself ready,
// listens for tasks published for the connect code, and immediately reports each one complete.
func (h *harness) startFakeCaptureClient(tb testing.TB) {
	tb.Helper()
	h.captureReady(tb)
	sub := h.admin.Subscribe(context.Background(), rediskey.TasksList(testCode))
	// make sure the subscription is live before returning
	if _, err := sub.Receive(context.Background()); err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = sub.Close() })
	go func() {
		for msg := range sub.Channel() {
			var mt task.ModifyTask
			if err := json.Unmarshal([]byte(msg.Payload), &mt); err != nil {
				continue
			}
			h.admin.Publish(context.Background(), rediskey.CompleteTask(mt.TaskID), "true")
		}
	}()
}

func batch(n int) task.UserModifyRequest {
	req := task.UserModifyRequest{Premium: premium.FreeTier}
	for i := 1; i <= n; i++ {
		req.Users = append(req.Users, task.UserModify{UserID: uint64(i), Mute: true, Deaf: true})
	}
	return req
}

func TestModifyUsers_NoCaptureClient_GoesStraightToPrimary(t *testing.T) {
	const ackTimeout = 20 * time.Millisecond
	h := newHarness(t, ackTimeout)

	start := time.Now()
	if err := h.tp.ModifyUsers(testGuild, testCode, batch(6), nil); err != nil {
		t.Fatalf("ModifyUsers: %v", err)
	}
	if got := h.primary.count(); got != 6 {
		t.Fatalf("primary bot applied %d mutes, want 6", got)
	}
	if n := h.cmds.get("publish"); n != 0 {
		t.Errorf("published %d capture tasks with no capture client, want 0", n)
	}
	if elapsed := time.Since(start); elapsed >= ackTimeout {
		t.Errorf("took %v; should not wait on an ack when no capture client can apply mutes", elapsed)
	}
}

func TestModifyUsers_UnresponsiveCapture_EveryUserStillMutedByPrimary(t *testing.T) {
	const ackTimeout = 20 * time.Millisecond
	h := newHarness(t, ackTimeout)
	h.captureReady(t) // the client claimed it can mute, but never acks

	// first batch of the game: the capture path is tried and found unresponsive
	start := time.Now()
	if err := h.tp.ModifyUsers(testGuild, testCode, batch(6), nil); err != nil {
		t.Fatalf("ModifyUsers: %v", err)
	}
	if got := h.primary.count(); got != 6 {
		t.Fatalf("primary bot applied %d mutes, want 6", got)
	}
	for _, c := range h.primary.calls {
		if !c.Mute || !c.Deaf {
			t.Errorf("user %d applied as mute=%v deaf=%v, want both true", c.UserID, c.Mute, c.Deaf)
		}
	}
	if elapsed := time.Since(start); elapsed < ackTimeout {
		t.Errorf("first batch returned in %v, before the %v ack timeout; capture path was not actually tried", elapsed, ackTimeout)
	}
	if n := h.cmds.get("set"); n != 1 {
		t.Errorf("blacklist written %d times for one unresponsive client, want exactly 1", n)
	}

	// second batch: the capture client is now known to be unresponsive, so no task is published and nothing waits
	h.cmds.reset()
	start = time.Now()
	if err := h.tp.ModifyUsers(testGuild, testCode, batch(6), nil); err != nil {
		t.Fatalf("ModifyUsers: %v", err)
	}
	if got := h.primary.count(); got != 12 {
		t.Fatalf("primary bot applied %d mutes total, want 12", got)
	}
	if n := h.cmds.get("publish"); n != 0 {
		t.Errorf("second batch published %d capture tasks, want 0 while blacklisted", n)
	}
	if elapsed := time.Since(start); elapsed >= ackTimeout {
		t.Errorf("second batch took %v, should not wait on the ack timeout once blacklisted", elapsed)
	}
}

func TestModifyUsers_ResponsiveCapture_UsesCaptureClientNotPrimary(t *testing.T) {
	h := newHarness(t, time.Second)
	h.startFakeCaptureClient(t)

	if err := h.tp.ModifyUsers(testGuild, testCode, batch(3), nil); err != nil {
		t.Fatalf("ModifyUsers: %v", err)
	}
	if got := h.primary.count(); got != 0 {
		t.Errorf("primary bot applied %d mutes, want 0 when the capture client acks", got)
	}
	if n := h.cmds.get("publish"); n != 3 {
		t.Errorf("published %d capture tasks, want 3", n)
	}
	if n := h.cmds.get("set"); n != 0 {
		t.Errorf("blacklist written %d times for a working client, want 0", n)
	}
}

func TestModifyUsers_CaptureRateLimit_DefersToPrimaryWithoutBlacklisting(t *testing.T) {
	h := newHarness(t, time.Second)
	h.startFakeCaptureClient(t)
	h.tp.maxRequests5Seconds = 2

	if err := h.tp.ModifyUsers(testGuild, testCode, batch(6), nil); err != nil {
		t.Fatalf("ModifyUsers: %v", err)
	}
	// the capture client got at most its budget; everyone else went to the primary bot; nothing was blacklisted
	published := h.cmds.get("publish")
	if published < 1 || published > 2 {
		t.Errorf("published %d capture tasks, want 1 or 2 under a budget of 2", published)
	}
	if got := h.primary.count(); got != 6-published {
		t.Errorf("primary bot applied %d mutes, want %d", got, 6-published)
	}
	if n := h.cmds.get("set"); n != 0 {
		t.Errorf("rate limiting should not blacklist, but wrote the blacklist %d times", n)
	}
}

func TestTokenUsable_FollowsDiscordBucketAndBlacklist(t *testing.T) {
	h := newHarness(t, time.Second)
	sess, err := discordgo.New("Bot fake")
	if err != nil {
		t.Fatal(err)
	}
	bucketID := discordgo.EndpointGuildMember(testGuild, "")

	if !h.tp.tokenUsable(testGuild, "tok", sess) {
		t.Fatal("a fresh session should be usable")
	}

	// simulate Discord's response headers reporting the guild-member bucket exhausted for a while
	bucket := sess.Ratelimiter.LockBucket(bucketID)
	if err := bucket.Release(http.Header{
		"X-Ratelimit-Remaining":   []string{"0"},
		"X-Ratelimit-Reset-After": []string{"5"},
		"Date":                    []string{time.Now().UTC().Format(http.TimeFormat)},
	}); err != nil {
		t.Fatal(err)
	}
	if h.tp.tokenUsable(testGuild, "tok", sess) {
		t.Error("session with an exhausted bucket should not be usable")
	}
	if !h.tp.tokenUsable("2", "tok", sess) {
		t.Error("buckets are per guild; the same session should be usable for another guild")
	}

	// a fresh session that has been blacklisted for this guild is also not usable
	sess2, _ := discordgo.New("Bot fake2")
	if err := h.tp.BlacklistTokenForDuration(testGuild, "tok2", time.Minute); err != nil {
		t.Fatal(err)
	}
	if h.tp.tokenUsable(testGuild, "tok2", sess2) {
		t.Error("blacklisted token should not be usable")
	}
}

// Benchmarks. The interesting numbers are ns/op (how long a batch blocks the game loop), publishes/op (capture tasks
// sent to a client that will never act on them), and blacklists/op (redundant blacklist writes). Compare
// before/after with benchstat.

func reportRedisMetrics(b *testing.B, h *harness) {
	b.ReportMetric(float64(h.cmds.get("publish"))/float64(b.N), "publishes/op")
	b.ReportMetric(float64(h.cmds.get("set"))/float64(b.N), "blacklists/op")
	b.ReportMetric(float64(h.primary.count())/float64(b.N), "primary/op")
}

// BenchmarkModifyUsers_NoCaptureClient_FirstBatch measures the first mute batch of a game whose capture app has no bot
// token, the common case. Every iteration starts with a fresh Redis.
func BenchmarkModifyUsers_NoCaptureClient_FirstBatch(b *testing.B) {
	h := newHarness(b, 5*time.Millisecond)
	req := batch(6)
	for b.Loop() {
		b.StopTimer()
		h.redis.FlushAll()
		b.StartTimer()
		if err := h.tp.ModifyUsers(testGuild, testCode, req, nil); err != nil {
			b.Fatal(err)
		}
	}
	reportRedisMetrics(b, h)
}

// BenchmarkModifyUsers_UnresponsiveCapture_FirstBatch measures the first mute batch of a game whose capture client
// claimed it can mute but never acks. Every iteration starts with a fresh Redis, so nothing is blacklisted yet.
func BenchmarkModifyUsers_UnresponsiveCapture_FirstBatch(b *testing.B) {
	h := newHarness(b, 5*time.Millisecond)
	req := batch(6)
	for b.Loop() {
		b.StopTimer()
		h.redis.FlushAll()
		h.captureReady(b)
		b.StartTimer()
		if err := h.tp.ModifyUsers(testGuild, testCode, req, nil); err != nil {
			b.Fatal(err)
		}
	}
	reportRedisMetrics(b, h)
}

// BenchmarkModifyUsers_UnresponsiveCapture_Blacklisted measures every later batch in that same game, once the capture
// client has been blacklisted.
func BenchmarkModifyUsers_UnresponsiveCapture_Blacklisted(b *testing.B) {
	h := newHarness(b, 5*time.Millisecond)
	h.captureReady(b)
	req := batch(6)
	if err := h.tp.ModifyUsers(testGuild, testCode, req, nil); err != nil {
		b.Fatal(err)
	}
	h.cmds.reset()
	h.primary.reset()
	for b.Loop() {
		if err := h.tp.ModifyUsers(testGuild, testCode, req, nil); err != nil {
			b.Fatal(err)
		}
	}
	reportRedisMetrics(b, h)
}

// BenchmarkModifyUsers_ResponsiveCapture measures the happy path, where a capture client acks every task.
func BenchmarkModifyUsers_ResponsiveCapture(b *testing.B) {
	h := newHarness(b, time.Second)
	h.startFakeCaptureClient(b)
	req := batch(6)
	for b.Loop() {
		if err := h.tp.ModifyUsers(testGuild, testCode, req, nil); err != nil {
			b.Fatal(err)
		}
	}
	reportRedisMetrics(b, h)
}
