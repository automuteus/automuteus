package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/gin-gonic/gin"
)

var (
	errInvalidToken       = errors.New("invalid Discord token")
	errDiscordForbidden   = errors.New("Discord scope missing")
	errDiscordUnavailable = errors.New("Discord authorization unavailable")
)

// GuildVerifier verifies an opaque Discord access token against Discord, not
// claims supplied by the caller. Implementations must fail closed on errors.
type GuildVerifier interface {
	VerifyGuild(context.Context, string, string) (VerifiedGuildAccess, error)
}

type discordVerifier struct {
	client  *http.Client
	baseURL string
}

func newDiscordVerifier() *discordVerifier {
	return &discordVerifier{client: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, baseURL: "https://discord.com/api/v10"}
}

func (v *discordVerifier) get(ctx context.Context, token, path string, out interface{}) error {
	return v.getAttempt(ctx, token, path, out, true)
}

func (v *discordVerifier) getAttempt(ctx context.Context, token, path string, out interface{}, retry bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+path, nil)
	if err != nil {
		return errDiscordUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := v.client.Do(req)
	if err != nil {
		return errDiscordUnavailable
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		// Loading the web guild picker and then authorizing a guild read can
		// exhaust the same Discord bucket. Honor a short cooldown once, within
		// the caller's deadline; never substitute cached authorization.
		if !retry {
			return errDiscordUnavailable
		}
		seconds, err := strconv.ParseFloat(resp.Header.Get("Retry-After"), 64)
		if err != nil {
			var limit struct {
				RetryAfter float64 `json:"retry_after"`
			}
			if json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&limit) != nil {
				return errDiscordUnavailable
			}
			seconds = limit.RetryAfter
		}
		if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds > 5 {
			return errDiscordUnavailable
		}
		resp.Body.Close()
		timer := time.NewTimer(time.Duration(seconds * float64(time.Second)))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return errDiscordUnavailable
		case <-timer.C:
			return v.getAttempt(ctx, token, path, out, false)
		}
	case http.StatusUnauthorized:
		return errInvalidToken
	case http.StatusForbidden:
		return errDiscordForbidden
	case http.StatusOK:
	default:
		return errDiscordUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 || json.Unmarshal(body, out) != nil {
		return errDiscordUnavailable
	}
	return nil
}

// Tokens from any Discord OAuth application are accepted, supporting external
// clients as well as our UI. Required scopes: identify and guilds. Read
// authorizations may be reused briefly by accessCache; writes verify live.
func (v *discordVerifier) VerifyGuild(ctx context.Context, token, guildID string) (VerifiedGuildAccess, error) {
	var user struct {
		ID string `json:"id"`
	}
	if err := v.get(ctx, token, "/users/@me", &user); err != nil {
		return VerifiedGuildAccess{}, err
	}
	if discord.ValidateSnowflake(user.ID) != nil {
		return VerifiedGuildAccess{}, errDiscordUnavailable
	}
	access := VerifiedGuildAccess{UserID: user.ID, GuildID: guildID}
	after := ""
	for page := 0; page < 100; page++ {
		var guilds []struct {
			ID          string `json:"id"`
			Owner       bool   `json:"owner"`
			Permissions string `json:"permissions"`
		}
		path := "/users/@me/guilds?limit=200"
		if after != "" {
			path += "&after=" + url.QueryEscape(after)
		}
		if err := v.get(ctx, token, path, &guilds); err != nil {
			return VerifiedGuildAccess{}, err
		}
		for _, guild := range guilds {
			if guild.ID == guildID {
				permissions, err := strconv.ParseInt(guild.Permissions, 10, 64)
				if err != nil || permissions < 0 {
					return VerifiedGuildAccess{}, errDiscordUnavailable
				}
				access.Member, access.Owner, access.Permissions = true, guild.Owner, permissions
				return access, nil
			}
		}
		if len(guilds) < 200 {
			return access, nil
		}
		next := guilds[len(guilds)-1].ID
		if discord.ValidateSnowflake(next) != nil || next == after {
			return VerifiedGuildAccess{}, errDiscordUnavailable
		}
		after = next
	}
	return VerifiedGuildAccess{}, errDiscordUnavailable
}

const memberRequestKey = "api.memberRequest"

// verifiedUserKey holds the Discord user ID the Bearer token resolved to, for audit logs. Absent on Basic Auth.
const verifiedUserKey = "api.verifiedUser"

// Explicitly configured platform credentials retain legacy access. Default
// credentials cannot bypass user authorization on these endpoints.
func guildAuthentication(config Config, verifier GuildVerifier, cache *accessCache, action GuildAction) gin.HandlerFunc {
	basic := gin.BasicAuth(gin.Accounts{"admin": config.AdminPassword})
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) == 2 && strings.EqualFold(parts[0], "Basic") && config.AdminPassword != "" && config.AdminPassword != "automuteus" {
			basic(c)
			return
		}
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Header("WWW-Authenticate", "Bearer")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, HttpError{StatusCode: 400, Error: "invalid guild ID"})
			return
		}
		access, err := cache.verify(c.Request.Context(), verifier, parts[1], guildID, action.verifiesLive())
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, errInvalidToken) {
				status = http.StatusUnauthorized
				c.Header("WWW-Authenticate", "Bearer")
			}
			if errors.Is(err, errDiscordForbidden) {
				status = http.StatusForbidden
			}
			c.AbortWithStatusJSON(status, HttpError{StatusCode: status, Error: "Unable to authorize Discord access"})
			return
		}
		if !AllowsGuildAction(access, guildID, action) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Set(memberRequestKey, true)
		c.Set(verifiedUserKey, access.UserID)
	}
}
