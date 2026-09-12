// Package notice carries platform-wide notices from operators (or from Galactus shutting down) to every bot shard,
// which surfaces them to players and, for critical notices, ends running games.
package notice

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/go-redis/redis/v8"
)

// Severity determines what the bot does with a notice.
type Severity string

const (
	// Info is shown on game status messages while active. Nothing else happens.
	Info Severity = "info"
	// Warning is shown prominently on game status messages while active. Games keep running.
	Warning Severity = "warning"
	// Critical ends every running game (unmuting everyone), records those matches as aborted, and blocks new
	// games while active.
	Critical Severity = "critical"
)

func (s Severity) Valid() bool {
	switch s {
	case Info, Warning, Critical:
		return true
	}
	return false
}

// GalactusShutdownMessageID identifies the notice Galactus raises on shutdown, so the bot can show it in each
// guild's language instead of the English text carried in the notice.
const GalactusShutdownMessageID = "notices.galactus.shutdown"

// Notice is the message published to every shard and stored as the active notice.
type Notice struct {
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	// Source identifies who raised the notice, e.g. "galactus" or "admin-api".
	Source   string `json:"source"`
	IssuedAt int64  `json:"issuedAt"`
	// ExpiresAt is the unix time the notice stops being active, or 0 if it stays until cleared.
	ExpiresAt int64 `json:"expiresAt"`
	// Cleared is set on the message published when an operator clears the active notice early.
	Cleared bool `json:"cleared,omitempty"`
	// MessageID, when set, names a message the bot knows how to translate; Message is then the English fallback.
	MessageID string `json:"messageId,omitempty"`
	// ConnectCodes, when non-nil, limits a critical notice to those games: only they are ended, the notice is not
	// stored as active, and new games are not blocked. A nil value means the whole platform. An empty, non-nil
	// slice is a targeted notice that affects no games (Galactus shutting down with no clients connected).
	ConnectCodes []string `json:"connectCodes"`
}

// Targeted reports whether the notice is limited to specific games rather than the whole platform.
func (n Notice) Targeted() bool { return n.ConnectCodes != nil }

// Targets reports whether the notice applies to the game with the given connect code.
func (n Notice) Targets(connectCode string) bool {
	if !n.Targeted() {
		return true
	}
	for _, code := range n.ConnectCodes {
		if code == connectCode {
			return true
		}
	}
	return false
}

var ErrInvalid = errors.New("notice: severity and message are required")

// Raise stores n as the active notice (for ttl, or until cleared if ttl is 0) and publishes it to every shard.
// A targeted notice (see Notice.ConnectCodes) is only published, never stored, since it concerns specific games
// rather than the platform.
func Raise(ctx context.Context, client *redis.Client, n Notice, ttl time.Duration) error {
	if !n.Severity.Valid() || n.Message == "" {
		return ErrInvalid
	}
	now := time.Now()
	n.IssuedAt = now.Unix()
	n.Cleared = false
	if ttl > 0 {
		n.ExpiresAt = now.Add(ttl).Unix()
	} else {
		n.ExpiresAt = 0
	}
	b, err := json.Marshal(n)
	if err != nil {
		return err
	}
	if !n.Targeted() {
		if err := client.Set(ctx, rediskey.ActiveNotice, b, ttl).Err(); err != nil {
			return err
		}
	}
	return client.Publish(ctx, rediskey.NoticeChannel, b).Err()
}

// Clear removes the active notice and tells every shard so status messages are refreshed promptly.
func Clear(ctx context.Context, client *redis.Client) error {
	if err := client.Del(ctx, rediskey.ActiveNotice).Err(); err != nil {
		return err
	}
	b, err := json.Marshal(Notice{Cleared: true, IssuedAt: time.Now().Unix()})
	if err != nil {
		return err
	}
	return client.Publish(ctx, rediskey.NoticeChannel, b).Err()
}

// Active returns the current notice, or nil if there is none.
func Active(ctx context.Context, client *redis.Client) (*Notice, error) {
	b, err := client.Get(ctx, rediskey.ActiveNotice).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	n, err := Decode(b)
	if err != nil {
		return nil, err
	}
	// ExpiresAt is also the bot's refresh deadline. Redis's relative TTL can outlive that whole-second
	// timestamp, so key existence alone must not keep the banner (or the maintenance lockout) active.
	if n.ExpiresAt > 0 && n.ExpiresAt <= time.Now().Unix() {
		return nil, nil
	}
	return n, nil
}

// Subscribe returns a subscription to notice messages. Callers must Close it.
func Subscribe(ctx context.Context, client *redis.Client) *redis.PubSub {
	return client.Subscribe(ctx, rediskey.NoticeChannel)
}

func Decode(b []byte) (*Notice, error) {
	var n Notice
	if err := json.Unmarshal(b, &n); err != nil {
		return nil, err
	}
	return &n, nil
}
