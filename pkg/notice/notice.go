// Package notice carries two kinds of platform events from operators and Galactus to every bot shard.
//
// A Notice is raised by an operator and stays active until cleared. Warnings are shown as a banner on every game's
// status message; a critical notice also ends every running game and blocks new ones.
//
// A Shutdown is announced by a Galactus replica that is about to exit. It names the games whose capture
// connections are being severed so the bot can end exactly those, and is never stored.
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
	// Warning is shown prominently on game status messages while active. Games keep running.
	Warning Severity = "warning"
	// Critical ends every running game (unmuting everyone, recording the matches as aborted) and blocks new games
	// while active.
	Critical Severity = "critical"
)

func (s Severity) Valid() bool {
	return s == Warning || s == Critical
}

// Notice is an operator-raised message, active until cleared.
type Notice struct {
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	IssuedAt int64    `json:"issuedAt"`
}

// Shutdown announces that the capture connections for the listed games are about to be severed.
type Shutdown struct {
	ConnectCodes []string `json:"connectCodes"`
}

// Event is what shards receive. Exactly one of the fields is set.
type Event struct {
	// NoticeChanged reports that the active notice was raised, replaced, or cleared; shards re-read it.
	NoticeChanged bool `json:"noticeChanged,omitempty"`
	// Shutdown reports capture connections going away.
	Shutdown *Shutdown `json:"shutdown,omitempty"`
}

var (
	ErrInvalid      = errors.New("notice: severity must be warning or critical, and message is required")
	ErrInvalidEvent = errors.New("notice: event must set exactly one of noticeChanged or shutdown")
)

func (e Event) valid() bool {
	return e.NoticeChanged != (e.Shutdown != nil)
}

// Raise stores n as the active notice, replacing any previous one, and tells every shard to re-read it.
func Raise(ctx context.Context, client *redis.Client, n Notice) error {
	if !n.Severity.Valid() || n.Message == "" {
		return ErrInvalid
	}
	n.IssuedAt = time.Now().Unix()
	b, err := json.Marshal(n)
	if err != nil {
		return err
	}
	if err := client.Set(ctx, rediskey.ActiveNotice, b, 0).Err(); err != nil {
		return err
	}
	return publish(ctx, client, Event{NoticeChanged: true})
}

// Clear removes the active notice and tells every shard so status messages are refreshed promptly.
func Clear(ctx context.Context, client *redis.Client) error {
	if err := client.Del(ctx, rediskey.ActiveNotice).Err(); err != nil {
		return err
	}
	return publish(ctx, client, Event{NoticeChanged: true})
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
	var n Notice
	if err := json.Unmarshal(b, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

// AnnounceShutdown tells every shard that the capture connections for connectCodes are about to be severed. An empty
// list is still announced (nothing is affected), so listeners see every replica go down.
func AnnounceShutdown(ctx context.Context, client *redis.Client, connectCodes []string) error {
	if connectCodes == nil {
		connectCodes = []string{}
	}
	return publish(ctx, client, Event{Shutdown: &Shutdown{ConnectCodes: connectCodes}})
}

func publish(ctx context.Context, client *redis.Client, e Event) error {
	if !e.valid() {
		return ErrInvalidEvent
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return client.Publish(ctx, rediskey.NoticeChannel, b).Err()
}

// Subscribe returns a subscription to events. Callers must Close it.
func Subscribe(ctx context.Context, client *redis.Client) *redis.PubSub {
	return client.Subscribe(ctx, rediskey.NoticeChannel)
}

// DecodeEvent parses an event and rejects malformed ones, so a listener never has to guess which branch applies.
func DecodeEvent(b []byte) (*Event, error) {
	var e Event
	if err := json.Unmarshal(b, &e); err != nil {
		return nil, err
	}
	if !e.valid() {
		return nil, ErrInvalidEvent
	}
	return &e, nil
}
