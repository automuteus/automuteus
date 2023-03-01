package redis

import (
	"context"
	"log"
	"time"
)

func (redisDriver *Driver) LockForToken(token string) {
	log.Println("Locking token for 5 seconds")
	err := redisDriver.client.Set(context.Background(), BotTokenIdentifyLock(token), "", time.Second*5).Err()
	if err != nil {
		log.Println(err)
	}
}

func (redisDriver *Driver) WaitForToken(token string) {
	for redisDriver.isTokenLocked(token) {
		log.Println("Sleeping for 5 seconds while waiting for token to become available")
		time.Sleep(time.Second * 5)
	}
}

func (redisDriver *Driver) isTokenLocked(token string) bool {
	v, err := redisDriver.client.Exists(context.Background(), BotTokenIdentifyLock(token)).Result()
	if err != nil {
		return false
	}

	return v == 1 //=1 means the rediskey is present, hence locked
}

func (redisDriver *Driver) IncrAndTestGuildTokenComboLock(guildID, hashToken string, max int64) bool {
	i, err := redisDriver.client.Incr(context.Background(), GuildTokenLock(guildID, hashToken)).Result()
	if err != nil {
		log.Println(err)
	}
	usable := i < max
	log.Printf("Token/capture %s on guild %s is at count %d. Using?: %v", hashToken, guildID, i, usable)
	if !usable {
		return false
	}

	// set the expiry only if the mute/deafen was successful, because we want to preserve any existing blacklist expiries
	err = redisDriver.client.Expire(context.Background(), GuildTokenLock(guildID, hashToken), time.Second*5).Err()
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
func (redisDriver *Driver) BlacklistTokenForDuration(guildID, hashToken string, duration time.Duration, val int64) error {
	return redisDriver.client.Set(context.Background(), GuildTokenLock(guildID, hashToken), val, duration).Err()
}
