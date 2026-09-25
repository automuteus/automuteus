package tokenprovider

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/bwmarrin/discordgo"
)

// isBlacklisted reports whether a token (or the capture client, keyed by connect code) was recently marked unusable
// for the guild.
func (tokenProvider *TokenProvider) isBlacklisted(guildID, hToken string) bool {
	n, err := tokenProvider.client.Exists(context.Background(), rediskey.MuteBlacklist(guildID, hToken)).Result()
	if err != nil {
		tokenProvider.logger(guildID).Error("failed to check mute blacklist", "token", hToken, "err", err)
		return false
	}
	return n > 0
}

// BlacklistTokenForDuration marks a token (or the capture client, keyed by connect code) as unusable for mute/deafen
// requests in a guild for the given duration.
func (tokenProvider *TokenProvider) BlacklistTokenForDuration(guildID, hToken string, duration time.Duration) error {
	return tokenProvider.client.Set(context.Background(), rediskey.MuteBlacklist(guildID, hToken), "1", duration).Err()
}

// tokenUsable skips workers known to be absent or blacklisted, and requires budget
// in the guild's Discord rate-limit bucket (tracked by discordgo from response headers).
func (tokenProvider *TokenProvider) tokenUsable(guildID, hToken string, sess *discordgo.Session) bool {
	if tokenProvider.workerKnownAbsent(hToken, guildID) {
		return false
	}
	if tokenProvider.isBlacklisted(guildID, hToken) {
		return false
	}
	bucket := sess.Ratelimiter.GetBucket(discordgo.EndpointGuildMember(guildID, ""))
	if wait := sess.Ratelimiter.GetWaitTime(bucket, 1); wait > 0 {
		tokenProvider.logger(guildID).Debug("secondary token rate limited for guild; skipping", "token", hToken, "wait", wait)
		return false
	}
	return true
}

func (tokenProvider *TokenProvider) attemptOnSecondaryTokens(guildID, userID string, tokenSubset map[string]struct{}, request task.UserModify) string {
	l := tokenProvider.logger(guildID).With("user", userID)
	if len(tokenProvider.activeSessions) > 0 {
		sess, hToken := tokenProvider.getSession(guildID, tokenSubset)
		if sess != nil {
			// Maintenance yields briefly to workers currently handling voice traffic.
			tokenProvider.pauseWorker(hToken, 5*time.Second)
			err := task.ApplyMuteDeaf(sess, guildID, userID, request.Mute, request.Deaf)
			if err != nil {
				l.Error("secondary bot voice change failed", "token", hToken, "err", err)
				tokenProvider.metrics.RecordWorkerFailure()

				// don't attempt this token for this guild for another 5 minutes
				err = tokenProvider.BlacklistTokenForDuration(guildID, hToken, UnresponsiveCaptureBlacklistDuration)
				if err != nil {
					l.Error("failed to blacklist token", "token", hToken, "err", err)
				}
			} else {
				l.Debug("voice change applied via secondary bot", "token", hToken, "mute", request.Mute, "deaf", request.Deaf)
				return hToken
			}
		} else {
			l.Debug("no usable secondary token; trying other methods")
		}
	} else {
		l.Debug("no secondary tokens configured; skipping")
	}
	return ""
}

// captureRoute is the per-batch decision about whether mute tasks may be sent to the capture client. It is resolved
// once per batch (rather than once per user) and shared by the workers, so a capture client that stops responding
// is blacklisted once and skipped by every remaining worker.
type captureRoute struct {
	tp          *TokenProvider
	guildID     string
	connectCode string
	log         *slog.Logger

	available bool        // a capture client able to apply mutes is connected, and is not blacklisted
	dead      atomic.Bool // set by the first worker whose task goes unacknowledged
}

func (tokenProvider *TokenProvider) newCaptureRoute(guildID, connectCode string, l *slog.Logger) *captureRoute {
	route := &captureRoute{tp: tokenProvider, guildID: guildID, connectCode: connectCode, log: l}
	ready, err := tokenProvider.client.Exists(context.Background(), rediskey.CaptureMuteReady(connectCode)).Result()
	if err != nil {
		l.Error("failed to check capture client readiness", "err", err)
		return route
	}
	if ready == 0 {
		l.Debug("no capture client able to apply mutes; using bot tokens only")
		return route
	}
	if tokenProvider.isBlacklisted(guildID, connectCode) {
		l.Debug("capture client is blacklisted; using bot tokens only")
		return route
	}
	route.available = true
	return route
}

func (route *captureRoute) usable() bool {
	return route.available && !route.dead.Load()
}

// markDead blacklists the capture client for the game, once, no matter how many workers observe the failure.
func (route *captureRoute) markDead() {
	if !route.dead.CompareAndSwap(false, true) {
		return
	}
	err := route.tp.BlacklistTokenForDuration(route.guildID, route.connectCode, UnresponsiveCaptureBlacklistDuration)
	if err != nil {
		route.log.Error("failed to blacklist capture client", "err", err)
		return
	}
	route.log.Warn("no ack from capture client; blacklisting it", "duration", UnresponsiveCaptureBlacklistDuration)
}

type captureResult int

const (
	captureUnavailable captureResult = iota // no capture client able to mute; nothing was attempted
	captureApplied
	captureRateLimited
	captureUnacked // the task was sent but never acknowledged
	captureFailed  // the task could not be sent
)

// attemptOnCaptureBot asks the capture client to apply the change and waits for its ack.
func (tokenProvider *TokenProvider) attemptOnCaptureBot(route *captureRoute, gid uint64, request task.UserModify) captureResult {
	l := route.log.With("user", request.UserID)
	// the capture client has its own Discord token whose rate limits the bot cannot see; count our requests to it
	// so we stop sending before Discord starts refusing
	if !tokenProvider.IncrAndTestGuildTokenComboLock(route.guildID, route.connectCode) {
		l.Debug("capture client near rate limit; deferring to bot tokens")
		tokenProvider.metrics.RecordCaptureTask(server.CaptureTaskThrottled)
		return captureRateLimited
	}

	taskObj := task.NewModifyTask(gid, request.UserID, task.PatchParams{
		Deaf: request.Deaf,
		Mute: request.Mute,
	})
	jBytes, err := json.Marshal(taskObj)
	if err != nil {
		l.Error("failed to marshal capture task", "err", err)
		tokenProvider.metrics.RecordCaptureTask(server.CaptureTaskError)
		return captureFailed
	}

	pubsub := tokenProvider.client.Subscribe(context.Background(), rediskey.CompleteTask(taskObj.TaskID))
	// wait for Redis to confirm the subscription before publishing the task; otherwise a fast ack can arrive
	// before we are listening and be lost, which would blacklist a working capture client
	if _, err = pubsub.Receive(context.Background()); err != nil {
		l.Error("failed to subscribe for capture task ack", "err", err)
		_ = pubsub.Close()
		tokenProvider.metrics.RecordCaptureTask(server.CaptureTaskError)
		return captureFailed
	}
	err = tokenProvider.client.Publish(context.Background(), rediskey.TasksList(route.connectCode), jBytes).Err()
	if err != nil {
		l.Error("failed to publish capture task", "err", err)
		_ = pubsub.Close()
		tokenProvider.metrics.RecordCaptureTask(server.CaptureTaskError)
		return captureFailed
	}

	acked := make(chan bool)
	go tokenProvider.waitForAck(pubsub, acked)
	if <-acked {
		l.Debug("voice change applied via capture client")
		tokenProvider.metrics.RecordCaptureTask(server.CaptureTaskApplied)
		return captureApplied
	}
	route.markDead()
	tokenProvider.metrics.RecordCaptureTask(server.CaptureTaskUnacked)
	return captureUnacked
}
