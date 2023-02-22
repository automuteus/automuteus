package tokenprovider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/automuteus/automuteus/v8/pkg/token"
	"github.com/bsm/redislock"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
	"golang.org/x/exp/constraints"
	"log"
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
}

func NewTokenProvider(client *redis.Client, sess *discordgo.Session, taskTimeout time.Duration, maxReq int64) *TokenProvider {
	return &TokenProvider{
		client:              client,
		primarySession:      sess,
		activeSessions:      make(map[string]*discordgo.Session),
		maxRequests5Seconds: maxReq,
		sessionLock:         sync.RWMutex{},
		taskTimeoutMs:       taskTimeout,
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

func (tokenProvider *TokenProvider) getSession(guildID string, hTokenSubset map[string]struct{}) (*discordgo.Session, string) {
	tokenProvider.sessionLock.RLock()
	defer tokenProvider.sessionLock.RUnlock()

	for hToken, sess := range tokenProvider.activeSessions {
		// if we have already used this token successfully, or haven't set any restrictions
		if hTokenSubset == nil || mapHasEntry(hTokenSubset, hToken) {
			// if this token isn't potentially rate-limited
			if tokenProvider.IncrAndTestGuildTokenComboLock(guildID, hToken) {
				return sess, hToken
			} else {
				log.Println("Secondary token is potentially rate-limited. Skipping")
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

func (tokenProvider *TokenProvider) IncrAndTestGuildTokenComboLock(guildID, hashToken string) bool {
	i, err := tokenProvider.client.Incr(context.Background(), rediskey.GuildTokenLock(guildID, hashToken)).Result()
	if err != nil {
		log.Println(err)
	}
	usable := i < tokenProvider.maxRequests5Seconds
	log.Printf("Token/capture %s on guild %s is at count %d. Using?: %v", hashToken, guildID, i, usable)
	if !usable {
		return false
	}

	// set the expiry only if the mute/deafen was successful, because we want to preserve any existing blacklist expiries
	err = tokenProvider.client.Expire(context.Background(), rediskey.GuildTokenLock(guildID, hashToken), time.Second*5).Err()
	if err != nil {
		log.Println(err)
	}

	return true
}

// BlacklistTokenForDuration sets a guild token (or connect code ala capture bot) to the maximum value allowed before
// attempting other non-rate-limited mute/deafen methods.
// NOTE: this will manifest as the capture/token in question appearing like it "has been used <maxnum> times" in logs,
// even if this is not technically accurate. A more accurate approach would probably use a totally separate Redis key,
// as opposed to this approach, which simply uses the ratelimiting counter key(s) to achieve blacklisting
func (tokenProvider *TokenProvider) BlacklistTokenForDuration(guildID, hashToken string, duration time.Duration) error {
	return tokenProvider.client.Set(context.Background(), rediskey.GuildTokenLock(guildID, hashToken), tokenProvider.maxRequests5Seconds, duration).Err()
}

const DefaultMaxWorkers = 8

var UnresponsiveCaptureBlacklistDuration = time.Minute * time.Duration(5)

func (tokenProvider *TokenProvider) ModifyUsers(guildID, connectCode string, request task.UserModifyRequest, voicelock *redislock.Lock) error {
	if voicelock != nil {
		defer voicelock.Release(context.Background())
	}

	gid, gerr := strconv.ParseUint(guildID, 10, 64)
	if gerr != nil {
		return gerr
	}
	limit := PremiumBotConstraints[request.Premium]

	tasksChannel := make(chan task.UserModify, len(request.Users))
	wg := sync.WaitGroup{}

	mdsc := task.MuteDeafenSuccessCounts{
		Worker:    0,
		Capture:   0,
		Official:  0,
		RateLimit: 0,
	}
	uniqueTokensUsed := make(map[string]struct{})
	lock := sync.Mutex{}
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
					lock.Lock()
					mdsc.Worker++
					lock.Unlock()

					tokenLock.Lock()
					uniqueTokensUsed[hToken] = struct{}{}
					tokenLock.Unlock()
				} else {
					success := tokenProvider.attemptOnCaptureBot(guildID, connectCode, gid, req)
					if success {
						lock.Lock()
						mdsc.Capture++
						lock.Unlock()
					} else {
						log.Printf("Applying mute=%v, deaf=%v using primary bot\n", req.Mute, req.Deaf)
						err := task.ApplyMuteDeaf(tokenProvider.primarySession, guildID, userIDStr, req.Mute, req.Deaf)
						if err != nil {
							lock.Lock()
							latestErr = err
							lock.Unlock()
							log.Println("Error on primary bot:")
							log.Println(err)
						} else {
							lock.Lock()
							mdsc.Official++
							lock.Unlock()
						}
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

	RecordDiscordRequestsByCounts(tokenProvider.client, mdsc)

	// note, this should probably be more systematic on startup, not when a mute/deafen task comes in. But this is a
	// context in which we already have the guildID, successful tokens, AND the premium limit...
	go tokenProvider.verifyBotMembership(guildID, limit, uniqueTokensUsed)

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
