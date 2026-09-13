package tokenprovider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/lock"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/automuteus/automuteus/v8/pkg/token"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
	"golang.org/x/exp/constraints"
	"log"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

var PremiumBotConstraints = map[premium.Tier]int{
	0: 0,
	1: 0,   // Free and Bronze have no premium bots
	2: 1,   // Silver has 1 bot
	3: 3,   // Gold has 3 bots
	4: 10,  // Platinum (TBD)
	5: 100, // Selfhost; 100 bots(!)
}

type TokenProvider struct {
	client         *redis.Client
	primarySession *discordgo.Session

	// maps hashed tokens to active discord sessions
	activeSessions      map[string]*discordgo.Session
	maxRequests5Seconds int64
	sessionLock         sync.RWMutex
	taskTimeoutMs       time.Duration

	// applyPrimary, when set, replaces the Discord API call the primary bot makes to mute/deafen a user. Tests use
	// it to observe the fallback path without a live session.
	applyPrimary func(guildID, userID string, mute, deaf bool) error

	// metrics receives voice-change outcomes and batch timings; production uses server.DefaultMetrics.
	metrics *server.Metrics
}

// applyWithPrimary mutes/deafens a user with the primary bot's own session.
func (tokenProvider *TokenProvider) applyWithPrimary(guildID, userID string, mute, deaf bool) error {
	if tokenProvider.applyPrimary != nil {
		return tokenProvider.applyPrimary(guildID, userID, mute, deaf)
	}
	return task.ApplyMuteDeaf(tokenProvider.primarySession, guildID, userID, mute, deaf)
}

func NewTokenProvider(client *redis.Client, sess *discordgo.Session, taskTimeout time.Duration, maxReq int64) *TokenProvider {
	return &TokenProvider{
		client:              client,
		primarySession:      sess,
		activeSessions:      make(map[string]*discordgo.Session),
		maxRequests5Seconds: maxReq,
		sessionLock:         sync.RWMutex{},
		taskTimeoutMs:       taskTimeout,
		metrics:             server.DefaultMetrics,
	}
}

func (tp *TokenProvider) Init(client *redis.Client, sess *discordgo.Session) {
	tp.client = client
	tp.primarySession = sess
}

//func rateLimitEventCallback(sess *discordgo.Session, rl *discordgo.RateLimit) {
//	log.Println(rl.Message)
//}

func (tokenProvider *TokenProvider) PopulateAndStartSessions(tokens []string) {
	for _, v := range tokens {
		tokenProvider.openAndStartSessionWithToken(v)
	}
}

func (tokenProvider *TokenProvider) openAndStartSessionWithToken(botToken string) bool {
	k := hashToken(botToken)
	tokenProvider.sessionLock.Lock()
	defer tokenProvider.sessionLock.Unlock()

	if _, ok := tokenProvider.activeSessions[k]; !ok {
		token.WaitForToken(tokenProvider.client, botToken)
		token.LockForToken(tokenProvider.client, botToken)
		sess, err := discordgo.New("Bot " + botToken)
		if err != nil {
			log.Println(err)
			return false
		}
		// Worker sessions only issue REST calls (mute/deafen, membership checks); they never read the
		// guild cache. Disabling state avoids holding a full copy of every guild per worker token.
		// State.User is still populated from the Ready event with state disabled.
		sess.StateEnabled = false
		sess.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuilds)
		err = sess.Open()
		if err != nil {
			log.Println(err)
			return false
		}
		// associates the guilds with this token to be used for requests
		sess.AddHandler(tokenProvider.newGuild)
		log.Println("Opened session on startup for " + k)
		tokenProvider.activeSessions[k] = sess
		return true
	}
	return false
}

// logger returns a logger tagged for this component and guild.
func (tokenProvider *TokenProvider) logger(guildID string) *slog.Logger {
	return slog.Default().With("component", "tokenprovider", "guild", guildID)
}

func (tokenProvider *TokenProvider) getSession(guildID string, hTokenSubset map[string]struct{}) (*discordgo.Session, string) {
	tokenProvider.sessionLock.RLock()
	defer tokenProvider.sessionLock.RUnlock()

	for hToken, sess := range tokenProvider.activeSessions {
		// if we have already used this token successfully, or haven't set any restrictions
		if hTokenSubset == nil || mapHasEntry(hTokenSubset, hToken) {
			if tokenProvider.tokenUsable(guildID, hToken, sess) {
				return sess, hToken
			}
		}
	}

	return nil, ""
}

func mapHasEntry[T constraints.Ordered, K any](dict map[T]K, key T) bool {
	if dict == nil {
		return false
	}
	_, ok := dict[key]
	return ok
}

// IncrAndTestGuildTokenComboLock counts a request against the capture client's per-guild budget and reports whether
// it was within the limit. Bot-owned tokens do not use this; discordgo tracks their real buckets from response headers.
func (tokenProvider *TokenProvider) IncrAndTestGuildTokenComboLock(guildID, hashToken string) bool {
	l := tokenProvider.logger(guildID).With("token", hashToken)
	i, err := tokenProvider.client.Incr(context.Background(), rediskey.GuildTokenLock(guildID, hashToken)).Result()
	if err != nil {
		l.Error("failed to increment token usage counter", "err", err)
	}
	usable := i < tokenProvider.maxRequests5Seconds
	l.Debug("token usage checked", "count", i, "usable", usable)
	if !usable {
		return false
	}

	err = tokenProvider.client.Expire(context.Background(), rediskey.GuildTokenLock(guildID, hashToken), time.Second*5).Err()
	if err != nil {
		l.Error("failed to set token usage expiry", "err", err)
	}

	return true
}

const DefaultMaxWorkers = 8

var UnresponsiveCaptureBlacklistDuration = time.Minute * time.Duration(5)

func (tokenProvider *TokenProvider) ModifyUsers(guildID, connectCode string, request task.UserModifyRequest, voicelock lock.Lock) error {
	if voicelock != nil {
		defer voicelock.Release(context.Background())
	}
	l := tokenProvider.logger(guildID).With("code", connectCode)
	start := time.Now()

	gid, gerr := strconv.ParseUint(guildID, 10, 64)
	if gerr != nil {
		return gerr
	}
	limit := PremiumBotConstraints[request.Premium]
	capture := tokenProvider.newCaptureRoute(guildID, connectCode, l)

	tasksChannel := make(chan task.UserModify, len(request.Users))
	wg := sync.WaitGroup{}

	mdsc := task.MuteDeafenSuccessCounts{
		Worker:    0,
		Capture:   0,
		Official:  0,
		RateLimit: 0,
	}
	uniqueTokensUsed := make(map[string]struct{})
	mu := sync.Mutex{}
	tokenLock := sync.RWMutex{}

	var latestErr error
	// start a handful of workers to handle the tasks
	for i := 0; i < DefaultMaxWorkers; i++ {
		go func() {
			for req := range tasksChannel {
				userIDStr := strconv.FormatUint(req.UserID, 10)
				hToken := ""
				if limit > 0 {
					tokenLock.RLock()
					if len(uniqueTokensUsed) >= limit {
						hToken = tokenProvider.attemptOnSecondaryTokens(guildID, userIDStr, uniqueTokensUsed, req)
						tokenLock.RUnlock()
					} else {
						tokenLock.RUnlock()
						hToken = tokenProvider.attemptOnSecondaryTokens(guildID, userIDStr, nil, req)
					}
				}
				if hToken != "" {
					mu.Lock()
					mdsc.Worker++
					mu.Unlock()
					tokenProvider.metrics.RecordVoiceChange(server.VoiceRouteWorker, true)

					tokenLock.Lock()
					uniqueTokensUsed[hToken] = struct{}{}
					tokenLock.Unlock()
				} else {
					result := captureUnavailable
					if capture.usable() {
						result = tokenProvider.attemptOnCaptureBot(capture, gid, req)
					}
					switch result {
					case captureApplied:
						mu.Lock()
						mdsc.Capture++
						mu.Unlock()
						tokenProvider.metrics.RecordVoiceChange(server.VoiceRouteCapture, true)
					case captureRateLimited:
						mu.Lock()
						mdsc.RateLimit++
						mu.Unlock()
						fallthrough
					default:
						l.Debug("applying voice change via primary bot", "user", userIDStr, "mute", req.Mute, "deaf", req.Deaf)
						err := tokenProvider.applyWithPrimary(guildID, userIDStr, req.Mute, req.Deaf)
						if err != nil {
							mu.Lock()
							latestErr = err
							mu.Unlock()
							l.Error("primary bot voice change failed", "user", userIDStr, "err", err)
						} else {
							mu.Lock()
							mdsc.Official++
							mu.Unlock()
						}
						tokenProvider.metrics.RecordVoiceChange(server.VoiceRoutePrimary, err == nil)
					}
				}
				wg.Done()
			}
		}()
	}

	for _, modifyReq := range request.Users {
		wg.Add(1)
		tasksChannel <- modifyReq
	}
	wg.Wait()
	close(tasksChannel)

	elapsed := time.Since(start)
	tokenProvider.metrics.ObserveMuteBatch(elapsed)

	// note, this should probably be more systematic on startup, not when a mute/deafen task comes in. But this is a
	// context in which we already have the guildID, successful tokens, AND the premium limit...
	go tokenProvider.verifyBotMembership(guildID, limit, uniqueTokensUsed)

	summary := l.With("users", len(request.Users), "worker", mdsc.Worker, "capture", mdsc.Capture,
		"official", mdsc.Official, "capture_throttled", mdsc.RateLimit, "elapsed", elapsed)
	if latestErr != nil {
		summary.Warn("voice changes issued with errors", "err", latestErr)
	} else {
		summary.Info("voice changes issued")
	}

	return latestErr
}

func (tokenProvider *TokenProvider) rateLimitEventCallback(sess *discordgo.Session, rl *discordgo.RateLimit) {
	log.Println(rl.Message)
}

func (tokenProvider *TokenProvider) waitForAck(pubsub *redis.PubSub, result chan<- bool) {
	t := time.NewTimer(tokenProvider.taskTimeoutMs)
	defer pubsub.Close()
	channel := pubsub.Channel()

	for {
		select {
		case <-t.C:
			t.Stop()
			result <- false
			return
		case val := <-channel:
			t.Stop()
			result <- val.Payload == "true"
			return
		}
	}
}

func hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func (tokenProvider *TokenProvider) Close() {
	tokenProvider.sessionLock.Lock()
	for _, v := range tokenProvider.activeSessions {
		v.Close()
	}

	tokenProvider.activeSessions = map[string]*discordgo.Session{}
	tokenProvider.sessionLock.Unlock()
	tokenProvider.primarySession.Close()
}

func (tokenProvider *TokenProvider) newGuild(s *discordgo.Session, m *discordgo.GuildCreate) {
	log.Println("added to " + m.ID)
}
