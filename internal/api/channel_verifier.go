package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/bwmarrin/discordgo"
)

// ChannelInfo is what the API needs to know about a channel a client wants the
// bot to post into: which guild it belongs to, whether it can hold messages,
// what to call it, and what the bot itself may do there.
type ChannelInfo struct {
	ID      string                `json:"id"`
	GuildID string                `json:"guild_id"`
	Type    discordgo.ChannelType `json:"type"`
	Name    string                `json:"name"`
	// Permissions is the bot's effective permission bitfield in this channel, computed the way Discord does:
	// @everyone and the bot's roles, then the channel's overwrites (a thread uses its parent's). Zero when the
	// bot is not a member of the guild.
	Permissions int64 `json:"-"`
}

// ChannelVerifier resolves a channel ID through Discord using the bot's own
// credentials, not the caller's. A user token cannot read channels, and the
// caller's word about which guild a channel belongs to is exactly what must
// not be trusted. Implementations fail closed.
type ChannelVerifier interface {
	VerifyChannel(ctx context.Context, channelID string) (ChannelInfo, error)
}

var (
	// errChannelNotFound covers a channel Discord does not know or the bot cannot see; either way the bot could
	// not post there, so the client's value is wrong rather than the service being down.
	errChannelNotFound = errors.New("channel not found or not visible to the bot")
	// errChannelUnavailable is a misconfigured bot token, a rate limit, an outage, or a malformed reply.
	errChannelUnavailable = errors.New("channel lookup unavailable")
)

type discordChannelVerifier struct {
	client  *http.Client
	baseURL string
	token   string
	now     func() time.Time // nil means time.Now

	mu    sync.Mutex
	botID string // the bot's own user ID, learned once from /users/@me
	// cooldowns holds, per route path, the earliest moment a 429 allows another attempt; globalUntil does the same
	// for every route when Discord flagged the limit as global. Calls during a cooldown fail without leaving the
	// process: every 429 counts toward Discord's invalid-request limit, past which it blocks the whole host.
	// Guarded by limitMu, separate from mu because self holds mu across a request.
	limitMu     sync.Mutex
	cooldowns   map[string]time.Time
	globalUntil time.Time
}

const (
	defaultRetryAfter = time.Second
	maxRetryAfter     = 5 * time.Minute
)

func (v *discordChannelVerifier) clock() time.Time {
	if v.now != nil {
		return v.now()
	}
	return time.Now()
}

// rateLimitBucket is the key a 429 cooldown is kept under. Discord limits routes per major parameter (the guild,
// channel, or webhook ID directly after that segment), not per full path: every member of one guild shares the
// guild's member-lookup bucket, and every user shares the user-lookup bucket. Other IDs are folded to "*" so a
// limit hit on one member's lookup pauses the rest rather than each one asking and being refused in turn.
func rateLimitBucket(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if i > 0 && discord.ValidateSnowflake(part) == nil {
			major := parts[i-1] == "guilds" || parts[i-1] == "channels" || parts[i-1] == "webhooks"
			if !major {
				parts[i] = "*"
			}
		}
	}
	return strings.Join(parts, "/")
}

// coolingDown reports whether a recent 429 says path's bucket must not be called yet.
func (v *discordChannelVerifier) coolingDown(path string) bool {
	bucket := rateLimitBucket(path)
	v.limitMu.Lock()
	defer v.limitMu.Unlock()
	now := v.clock()
	if now.Before(v.globalUntil) {
		return true
	}
	until, ok := v.cooldowns[bucket]
	if !ok {
		return false
	}
	if now.Before(until) {
		return true
	}
	delete(v.cooldowns, bucket)
	return false
}

// noteRateLimit records a 429 for path's bucket, or for every route when Discord marks it global.
func (v *discordChannelVerifier) noteRateLimit(path string, resp *http.Response, body []byte) {
	bucket := rateLimitBucket(path)
	until := v.clock().Add(retryAfter(resp, body))
	v.limitMu.Lock()
	defer v.limitMu.Unlock()
	if strings.EqualFold(resp.Header.Get("X-RateLimit-Global"), "true") {
		if until.After(v.globalUntil) {
			v.globalUntil = until
		}
		return
	}
	if v.cooldowns == nil {
		v.cooldowns = map[string]time.Time{}
	}
	if until.After(v.cooldowns[bucket]) {
		v.cooldowns[bucket] = until
	}
}

// retryAfter is how long a 429 asks us to wait: the Retry-After header in seconds, else the body's retry_after in
// fractional seconds. A missing or nonsense value waits a second; nothing waits more than five minutes, so a bad
// reply cannot switch the lookups off for good.
func retryAfter(resp *http.Response, body []byte) time.Duration {
	wait := defaultRetryAfter
	if secs, err := strconv.ParseFloat(strings.TrimSpace(resp.Header.Get("Retry-After")), 64); err == nil && secs > 0 {
		wait = time.Duration(secs * float64(time.Second))
	} else {
		var payload struct {
			RetryAfter float64 `json:"retry_after"`
		}
		if json.Unmarshal(body, &payload) == nil && payload.RetryAfter > 0 {
			wait = time.Duration(payload.RetryAfter * float64(time.Second))
		}
	}
	if wait > maxRetryAfter {
		wait = maxRetryAfter
	}
	return wait
}

func newDiscordChannelVerifier(token string) *discordChannelVerifier {
	return &discordChannelVerifier{client: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, baseURL: "https://discord.com/api/v10", token: token}
}

// get performs one bot-authenticated GET and decodes the body. 404 and 403 are errChannelNotFound: the object
// does not exist or the bot is not allowed to see it. Anything else that is not a 200 is errChannelUnavailable,
// and a 429 also starts a cooldown for the route (or everything) that later calls honour without asking Discord.
func (v *discordChannelVerifier) get(ctx context.Context, path string, out interface{}) error {
	if v.coolingDown(path) {
		return errChannelUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+path, nil)
	if err != nil {
		return errChannelUnavailable
	}
	req.Header.Set("Authorization", "Bot "+v.token)
	resp, err := v.client.Do(req)
	if err != nil {
		return errChannelUnavailable
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusForbidden:
		return errChannelNotFound
	case http.StatusTooManyRequests:
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		v.noteRateLimit(path, resp, limited)
		return errChannelUnavailable
	default:
		return errChannelUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (256<<10)+1))
	if err != nil || len(body) > 256<<10 {
		return errChannelUnavailable
	}
	if json.Unmarshal(body, out) != nil {
		return errChannelUnavailable
	}
	return nil
}

func (v *discordChannelVerifier) self(ctx context.Context) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.botID != "" {
		return v.botID, nil
	}
	var user struct {
		ID string `json:"id"`
	}
	if err := v.get(ctx, "/users/@me", &user); err != nil || discord.ValidateSnowflake(user.ID) != nil {
		// A 403/404 here is a bad token, not a missing channel.
		return "", errChannelUnavailable
	}
	v.botID = user.ID
	return user.ID, nil
}

// VerifyChannel fetches the channel, then what is needed to compute the bot's permissions in it: the parent
// channel for a thread (overwrites live on the parent), the guild's roles and owner, and the bot's own member
// record. Each lookup uses the bot's credentials only.
func (v *discordChannelVerifier) VerifyChannel(ctx context.Context, channelID string) (ChannelInfo, error) {
	if discord.ValidateSnowflake(channelID) != nil {
		return ChannelInfo{}, errChannelNotFound
	}
	var channel discordgo.Channel
	if err := v.get(ctx, "/channels/"+channelID, &channel); err != nil {
		return ChannelInfo{}, err
	}
	if channel.ID != channelID || discord.ValidateSnowflake(channel.GuildID) != nil {
		return ChannelInfo{}, errChannelUnavailable
	}
	info := ChannelInfo{ID: channel.ID, GuildID: channel.GuildID, Type: channel.Type, Name: channel.Name}

	overwrites := &channel
	if channel.IsThread() {
		if discord.ValidateSnowflake(channel.ParentID) != nil {
			return ChannelInfo{}, errChannelUnavailable
		}
		var parent discordgo.Channel
		if err := v.get(ctx, "/channels/"+channel.ParentID, &parent); err != nil {
			return ChannelInfo{}, err
		}
		if parent.ID != channel.ParentID || parent.GuildID != channel.GuildID {
			return ChannelInfo{}, errChannelUnavailable
		}
		overwrites = &parent
	}

	botID, err := v.self(ctx)
	if err != nil {
		return ChannelInfo{}, err
	}
	var guild discordgo.Guild
	if err := v.get(ctx, "/guilds/"+channel.GuildID, &guild); err != nil {
		return ChannelInfo{}, err
	}
	if guild.ID != channel.GuildID {
		return ChannelInfo{}, errChannelUnavailable
	}
	var member discordgo.Member
	if err := v.get(ctx, "/guilds/"+channel.GuildID+"/members/"+botID, &member); err != nil {
		if errors.Is(err, errChannelNotFound) {
			// The channel exists but the bot is not a member of its guild, so it holds no permissions there.
			return info, nil
		}
		return ChannelInfo{}, err
	}
	info.Permissions = memberChannelPermissions(&guild, overwrites, botID, member.Roles)
	return info, nil
}

// everyPermission is what the owner and Administrators hold: every bit, including ones newer than the pinned
// discordgo's PermissionAll (which lacks Send Messages in Threads, for instance).
const everyPermission = ^int64(0)

// memberChannelPermissions is Discord's permission algorithm for one member in one channel, the same as
// discordgo's unexported memberPermissions: the owner has everything; otherwise start from @everyone plus the
// member's roles, promote Administrator to everything, then apply the channel's @everyone overwrite, the union
// of its role overwrites, and finally the member's own overwrite.
func memberChannelPermissions(guild *discordgo.Guild, channel *discordgo.Channel, userID string, roles []string) int64 {
	if userID == guild.OwnerID {
		return everyPermission
	}
	var permissions int64
	for _, role := range guild.Roles {
		if role.ID == guild.ID {
			permissions |= role.Permissions
			break
		}
	}
	for _, role := range guild.Roles {
		for _, roleID := range roles {
			if role.ID == roleID {
				permissions |= role.Permissions
				break
			}
		}
	}
	if permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		return everyPermission
	}
	for _, overwrite := range channel.PermissionOverwrites {
		if overwrite.ID == guild.ID {
			permissions &= ^overwrite.Deny
			permissions |= overwrite.Allow
			break
		}
	}
	var denies, allows int64
	for _, overwrite := range channel.PermissionOverwrites {
		if overwrite.Type != discordgo.PermissionOverwriteTypeRole {
			continue
		}
		for _, roleID := range roles {
			if overwrite.ID == roleID {
				denies |= overwrite.Deny
				allows |= overwrite.Allow
				break
			}
		}
	}
	permissions &= ^denies
	permissions |= allows
	for _, overwrite := range channel.PermissionOverwrites {
		if overwrite.Type == discordgo.PermissionOverwriteTypeMember && overwrite.ID == userID {
			permissions &= ^overwrite.Deny
			permissions |= overwrite.Allow
			break
		}
	}
	return permissions
}
