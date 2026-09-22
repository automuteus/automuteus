package bot

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/go-redis/redis/v8"
)

// ConsumerLeaseTTL is how long a game's consumer lease lives without renewal. A process that crashes or loses its
// connection hands the game over to a standby within this time; a draining process hands over immediately.
const ConsumerLeaseTTL = 10 * time.Second

// consumerLease makes one process at a time the consumer of a game's capture events. Several processes subscribe
// to every game (twins, and adopters), which gives failover, but if they all popped events the events could be
// applied out of order: a mute computed for one phase, delayed, could land after the unmute for the next.
//
// The lease is taken for one burst of queued events and released when the queue is empty, so another subscriber
// can end the game (unmuting everyone and deleting its state) only between bursts, never under a pending mute. Every
// pop is fenced: it only succeeds while the lease key still carries this process's token, so a holder that outlived
// its lease finds out at its next pop instead of consuming alongside the new holder. The holder renews the lease in
// the background while it holds it.
type consumerLease struct {
	client      *redis.Client
	connectCode string
	key         string
	token       string
	log         *slog.Logger

	mu        sync.Mutex
	holding   bool
	stopRenew chan struct{}
}

const (
	// renewScript extends the lease only if this process still holds it.
	renewScript = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("pexpire", KEYS[1], ARGV[2]) else return 0 end`
	// releaseScript deletes the lease only if this process still holds it.
	releaseScript = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`
	// popScript pops the next queued job only if this process still holds the lease: {1, job} on success,
	// {1, nil} when the queue is empty, {0} when the lease is held by someone else or has lapsed.
	popScript = `if redis.call("get", KEYS[1]) ~= ARGV[1] then return {0} end
local v = redis.call("lpop", KEYS[2])
if v then return {1, v} else return {1} end`
)

var errNoRedis = errors.New("no redis client")

func (bot *Bot) newConsumerLease(connectCode string) *consumerLease {
	var id [8]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return &consumerLease{
		client:      bot.redisClient(),
		connectCode: connectCode,
		key:         rediskey.GameConsumerLease(connectCode),
		token:       hex.EncodeToString(id[:]),
		log:         bot.gameLog(GameStateRequest{ConnectCode: connectCode}),
	}
}

// acquire takes the lease if it is free, and reports whether this process now holds it. Holding it already counts.
func (l *consumerLease) acquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.holding {
		return true
	}
	if l.client == nil {
		return false
	}
	ok, err := l.client.SetNX(ctx, l.key, l.token, ConsumerLeaseTTL).Result()
	if err != nil {
		l.log.Error("failed to acquire consumer lease", "err", err)
		return false
	}
	if !ok {
		return false
	}
	l.holding = true
	l.stopRenew = make(chan struct{})
	go l.renewLoop(l.stopRenew)
	l.log.Debug("acquired consumer lease")
	return true
}

// acquireBlocking waits for the lease for as long as it takes, logging while it waits. A healthy holder yields
// between bursts and a dead holder's lease lapses, so in practice the wait is bounded by one burst. There is
// deliberately no timeout: a timeout would let cleanup proceed while another process is still applying a mute,
// which is the race the lease exists to prevent.
func (l *consumerLease) acquireBlocking(what string) {
	const poll = 100 * time.Millisecond
	var waited time.Duration
	for !l.acquire() {
		time.Sleep(poll)
		waited += poll
		if waited%ConsumerLeaseTTL == 0 {
			l.log.Warn("still waiting for the consumer lease", "for", what, "waited", waited)
		}
	}
}

func (l *consumerLease) held() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.holding
}

// pop takes the next queued job for the game, provided this process still holds the lease. An empty payload with
// owner true means the queue is empty. owner false means the lease is no longer this process's; the local hold is
// dropped so the caller stops consuming.
func (l *consumerLease) pop() (payload string, owner bool, err error) {
	if l.client == nil {
		return "", false, errNoRedis
	}
	raw, err := l.client.Eval(ctx, popScript, []string{l.key, rediskey.JobNamespace + l.connectCode}, l.token).Result()
	if err != nil {
		return "", false, err
	}
	res, _ := raw.([]interface{})
	if len(res) == 0 || res[0] != int64(1) {
		l.lost()
		return "", false, nil
	}
	if len(res) < 2 || res[1] == nil {
		return "", true, nil
	}
	s, _ := res[1].(string)
	return s, true, nil
}

// lost drops the local hold after Redis showed the lease belongs to someone else.
func (l *consumerLease) lost() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.holding {
		return
	}
	l.holding = false
	close(l.stopRenew)
	l.stopRenew = nil
	l.log.Warn("consumer lease lost; another process is consuming this game")
}

// renewLoop keeps the lease alive until stop is closed or the lease turns out to be lost.
func (l *consumerLease) renewLoop(stop chan struct{}) {
	ticker := time.NewTicker(ConsumerLeaseTTL / 3)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			n, err := l.client.Eval(ctx, renewScript, []string{l.key}, l.token, ConsumerLeaseTTL.Milliseconds()).Int()
			if err != nil {
				l.log.Error("failed to renew consumer lease", "err", err)
				continue
			}
			if n == 0 {
				l.mu.Lock()
				if l.stopRenew == stop { // still the goroutine for the current hold
					l.holding = false
					l.stopRenew = nil
					l.log.Warn("consumer lease lost; another process is consuming this game")
				}
				l.mu.Unlock()
				return
			}
		}
	}
}

// release gives the lease up if held. With wake set, the game's subscribers are notified so a standby takes over
// at once; that is for handovers only, since waking standbys after every burst would have them wake each other
// forever.
func (l *consumerLease) release(wake bool) {
	l.mu.Lock()
	if !l.holding {
		l.mu.Unlock()
		return
	}
	l.holding = false
	close(l.stopRenew)
	l.stopRenew = nil
	l.mu.Unlock()

	if _, err := l.client.Eval(ctx, releaseScript, []string{l.key}, l.token).Int(); err != nil {
		l.log.Error("failed to release consumer lease", "err", err)
	}
	if wake {
		task.Notify(ctx, l.client, l.connectCode)
	}
}
