// Package notice carries two kinds of platform events from operators and Galactus to every bot shard.
//
// A Notice is raised by an operator and stays active until cleared. It is one of a fixed set of kinds rather than free
// text, so every server sees it in its own language. Warnings are shown as a banner on every game's status message;
// a critical notice also ends every running game and blocks new ones.
//
// A Shutdown is announced by a Galactus replica that is about to exit. It names the games whose capture
// connections are being severed so the bot can end exactly those, and is never stored.
//
// GamesAvailable is announced by a bot process when it creates a game and when it is about to exit. It names games
// that every process serving their guilds should be subscribed to, so that a game always has a standby consumer
// and a process exit hands its games over before their queued capture events go stale. It is never stored either.
package notice

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/go-redis/redis/v8"
	"github.com/nicksnyder/go-i18n/v2/i18n"
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

// Kind is one of the preset notices an operator can raise. Each kind has a fixed severity and a localized message.
type Kind string

const (
	// BotUpdate warns that the bot is being rolled out and may be slow to respond for a few minutes.
	BotUpdate Kind = "bot_update"
	// Maintenance ends every game and blocks new ones until it is cleared.
	Maintenance Kind = "maintenance"
)

var kinds = map[Kind]struct {
	severity Severity
	message  *i18n.Message
}{
	BotUpdate: {Warning, &i18n.Message{
		ID:    "notices.kind.botUpdate",
		Other: "AutoMuteUs is being updated, so the bot may be slow to respond for a few minutes.",
	}},
	Maintenance: {Critical, &i18n.Message{
		ID:    "notices.kind.maintenance",
		Other: "AutoMuteUs is down for maintenance.",
	}},
}

func (k Kind) Valid() bool {
	_, ok := kinds[k]
	return ok
}

// Severity returns what the bot does with a notice of this kind, or "" for an unknown kind.
func (k Kind) Severity() Severity {
	return kinds[k].severity
}

// Message returns the text shown to players for this kind, or nil for an unknown kind.
func (k Kind) Message() *i18n.Message {
	return kinds[k].message
}

// Notice returns a notice of this kind with its derived fields filled in. IssuedAt is left for Raise to set.
func (k Kind) Notice() Notice {
	n := Notice{Kind: k, Severity: k.Severity()}
	if m := k.Message(); m != nil {
		n.Message = m.Other
	}
	return n
}

// Notice is an operator-raised message, active until cleared.
type Notice struct {
	Kind Kind `json:"kind"`
	// Severity and Message are derived from Kind when the notice is raised. Message is the English text, for bots
	// that predate kinds and still render it as the banner.
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	IssuedAt int64    `json:"issuedAt"`
}

// Shutdown announces that the capture connections for the listed games are about to be severed.
type Shutdown struct {
	ConnectCodes []string `json:"connectCodes"`
}

// GameRef identifies one game.
type GameRef struct {
	GuildID     string `json:"guildID"`
	ConnectCode string `json:"connectCode"`
}

// GamesAvailable announces games that every process serving their guilds should subscribe to.
type GamesAvailable struct {
	Games []GameRef `json:"games"`
}

// Event is what shards receive. Exactly one of the fields is set.
type Event struct {
	// NoticeChanged reports that the active notice was raised, replaced, or cleared; shards re-read it.
	NoticeChanged bool `json:"noticeChanged,omitempty"`
	// Shutdown reports capture connections going away.
	Shutdown *Shutdown `json:"shutdown,omitempty"`
	// GamesAvailable reports games to subscribe to.
	GamesAvailable *GamesAvailable `json:"gamesAvailable,omitempty"`
}

var (
	ErrInvalid      = errors.New("notice: unknown kind")
	ErrInvalidEvent = errors.New("notice: event must set exactly one of noticeChanged, shutdown, or gamesAvailable")
)

func (e Event) valid() bool {
	set := 0
	if e.NoticeChanged {
		set++
	}
	if e.Shutdown != nil {
		set++
	}
	if e.GamesAvailable != nil {
		set++
	}
	return set == 1
}

// Raise stores a notice of kind k as the active notice, replacing any previous one, and tells every shard to re-read
// it.
func Raise(ctx context.Context, client *redis.Client, k Kind) error {
	if !k.Valid() {
		return ErrInvalid
	}
	n := k.Notice()
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

// AnnounceGames tells every shard about games it should be subscribed to: a game just created, or the games of a
// process that is exiting. Shards that serve the games' guilds and are not already attached subscribe to them.
func AnnounceGames(ctx context.Context, client *redis.Client, games []GameRef) error {
	if games == nil {
		games = []GameRef{}
	}
	return publish(ctx, client, Event{GamesAvailable: &GamesAvailable{Games: games}})
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
