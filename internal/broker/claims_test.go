package broker

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
)

// fakeConn records what the broker does to a capture client's socket.
type fakeConn struct {
	mu      sync.Mutex
	emitted []string
	left    []string
	closed  bool
}

func (c *fakeConn) Emit(msg string, _ ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emitted = append(c.emitted, msg)
}

func (c *fakeConn) Leave(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.left = append(c.left, room)
}

func (c *fakeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeConn) toldToQuit() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.emitted) == 1 && c.emitted[0] == capture.KillSelfEvent
}

// listening starts a broker's claims listener and waits until it is subscribed, so no claim is missed.
func listening(t *testing.T, b *Broker) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	pubsub, err := b.subscribeClaims(ctx)
	if err != nil {
		t.Fatal(err)
	}
	go b.serveClaims(ctx, pubsub)
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting until %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func isClosed(kill <-chan struct{}) bool {
	select {
	case <-kill:
		return true
	default:
		return false
	}
}

func (broker *Broker) holds(socketID string) bool {
	_, ok := broker.connectCode(socketID)
	return ok
}

// Replicas number their sockets independently, so the same socket ID on two brokers must not be mistaken for one.
func TestClaim_NewestCaptureOnAnotherReplicaRetiresTheOld(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	one, two := NewBroker(mr.Addr(), "", ""), NewBroker(mr.Addr(), "", "")
	listening(t, one)
	listening(t, two)

	old, newcomer := &fakeConn{}, &fakeConn{}
	oldKill := one.claim(ctx, old, "1", "ABCDEFGH")
	newKill := two.claim(ctx, newcomer, "1", "ABCDEFGH")

	// killself is the last thing a retired capture client is sent
	eventually(t, "the old capture client is told to quit", old.toldToQuit)
	if one.holds("1") {
		t.Error("the old capture client is still listened to")
	}
	if len(old.left) != 1 || old.left[0] != "ABCDEFGH" {
		t.Errorf("old capture client left %v, want the connect code's room", old.left)
	}
	if !isClosed(oldKill) {
		t.Error("old capture client's workers were not stopped")
	}
	if !two.holds("1") || isClosed(newKill) || newcomer.toldToQuit() {
		t.Error("the newcomer must be left alone")
	}
}

func TestClaim_NewestCaptureOnTheSameReplicaRetiresTheOld(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	b := NewBroker(mr.Addr(), "", "")
	listening(t, b)

	old, newcomer := &fakeConn{}, &fakeConn{}
	b.claim(ctx, old, "1", "ABCDEFGH")
	b.claim(ctx, newcomer, "2", "ABCDEFGH")

	eventually(t, "the old capture client is told to quit", old.toldToQuit)
	if b.holds("1") || !b.holds("2") || newcomer.toldToQuit() {
		t.Error("only the old capture client should be retired")
	}
}

func TestClaim_LeavesOtherConnectCodesAlone(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	b := NewBroker(mr.Addr(), "", "")
	listening(t, b)

	other, replaced := &fakeConn{}, &fakeConn{}
	b.claim(ctx, other, "1", "IJKLMNOP")
	b.claim(ctx, replaced, "2", "ABCDEFGH")
	b.claim(ctx, &fakeConn{}, "3", "ABCDEFGH")

	eventually(t, "the replaced capture client is told to quit", replaced.toldToQuit)
	if !b.holds("1") || other.toldToQuit() {
		t.Error("a capture client on another connect code was retired")
	}
}

// The replaced capture client may have been the one applying mutes; the newcomer has to send its own bot ID.
func TestClaim_WithdrawsCaptureMuteReady(t *testing.T) {
	mr := miniredis.RunT(t)
	b := NewBroker(mr.Addr(), "", "")
	if err := mr.Set(rediskey.CaptureMuteReady("ABCDEFGH"), "1"); err != nil {
		t.Fatal(err)
	}

	b.claim(context.Background(), &fakeConn{}, "1", "ABCDEFGH")

	if mr.Exists(rediskey.CaptureMuteReady("ABCDEFGH")) {
		t.Error("capture-ready flag survived a new capture client claiming the connect code")
	}
}

func TestClaim_ReplacedCaptureIsClosedAfterItHadTimeToQuit(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	b := NewBroker(mr.Addr(), "", "")

	old := &fakeConn{}
	b.claim(ctx, old, "1", "ABCDEFGH")
	b.claim(ctx, &fakeConn{}, "2", "ABCDEFGH")
	b.retireReplaced(ctx, "ABCDEFGH")

	old.mu.Lock()
	closed := old.closed
	old.mu.Unlock()
	if closed {
		t.Fatal("closing straight away races killself, and a capture client reconnects after a plain disconnect")
	}
}

// If the claim message never reached the old capture client's broker, its disconnect must still not unlink the game.
func TestRelease_ReplacedCaptureLeavesTheGameAlone(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	one, two := NewBroker(mr.Addr(), "", ""), NewBroker(mr.Addr(), "", "")
	one.claim(ctx, &fakeConn{}, "1", "ABCDEFGH")
	two.claim(ctx, &fakeConn{}, "1", "ABCDEFGH")
	if err := mr.Set(rediskey.CaptureMuteReady("ABCDEFGH"), "1"); err != nil {
		t.Fatal(err)
	}
	mr.Del(rediskey.JobNamespace + "ABCDEFGH")

	if _, ok := one.release(ctx, "1"); ok {
		t.Error("a replaced capture client's room should not be cleared")
	}
	if mr.Exists(rediskey.JobNamespace + "ABCDEFGH") {
		t.Error("a replaced capture client's disconnect told the bot the game was unlinked")
	}
	if !mr.Exists(rediskey.CaptureMuteReady("ABCDEFGH")) {
		t.Error("a replaced capture client's disconnect withdrew its successor's capture-ready flag")
	}
}

func TestRelease_HolderUnlinksTheGame(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	b := NewBroker(mr.Addr(), "", "")
	kill := b.claim(ctx, &fakeConn{}, "1", "ABCDEFGH")
	if err := mr.Set(rediskey.CaptureMuteReady("ABCDEFGH"), "1"); err != nil {
		t.Fatal(err)
	}
	mr.Del(rediskey.JobNamespace + "ABCDEFGH")

	code, ok := b.release(ctx, "1")
	if !ok || code != "ABCDEFGH" {
		t.Fatalf("release = %q, %v", code, ok)
	}
	jobs, err := mr.List(rediskey.JobNamespace + "ABCDEFGH")
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs = %v, %v; want one connection job", jobs, err)
	}
	if want := `{"type":` + strconv.Itoa(int(task.ConnectionJob)) + `,"payload":"false"}`; jobs[0] != want {
		t.Errorf("job = %s, want %s", jobs[0], want)
	}
	if mr.Exists(rediskey.CaptureMuteReady("ABCDEFGH")) {
		t.Error("capture-ready flag survived its capture client disconnecting")
	}
	if !isClosed(kill) || b.holds("1") {
		t.Error("a disconnected capture client must be forgotten and its workers stopped")
	}
}

// The holder key only garbage-collects; once it expires, a disconnect is treated as the holder's.
func TestRelease_ExpiredHolderStillUnlinksTheGame(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	b := NewBroker(mr.Addr(), "", "")
	b.claim(ctx, &fakeConn{}, "1", "ABCDEFGH")
	mr.FastForward(CaptureHolderTTL + time.Second)

	if _, ok := b.release(ctx, "1"); !ok {
		t.Error("with no holder recorded, the disconnecting capture client should unlink its game")
	}
}
