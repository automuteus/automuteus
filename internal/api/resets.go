package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	pgstorage "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/gin-gonic/gin"
)

// StatsReset is the response to a stats reset: what was cleared and how much. For a guild reset Games counts the
// games deleted; for a player reset it counts the games the player was removed from.
type StatsReset struct {
	GuildID string `json:"guildId"`
	UserID  string `json:"userId,omitempty"`
	Games   int64  `json:"games"`
}

func (s *DataStore) ResetGuildStats(ctx context.Context, guildID string) (int64, error) {
	return pgstorage.ResetGuildStats(ctx, s.postgres, guildID)
}

func (s *DataStore) ResetUserStats(ctx context.Context, guildID, userID string) (int64, error) {
	return pgstorage.ResetGuildUserStats(ctx, s.postgres, guildID, userID)
}

// statsCaches are the router's cached stats documents, so a reset can drop everything it made stale.
type statsCaches struct {
	guild *listCache[GuildStats]
	match *listCache[MatchSummary]
	user  *listCache[UserStats]
}

// forgetGuild drops the guild's rollup, match summaries, and player documents. A player reset clears the whole guild
// too, since the other players' ranks and the match rosters change with it.
func (s statsCaches) forgetGuild(guildID string) {
	match := func(key string) bool { return key == guildID || strings.HasPrefix(key, guildID+"/") }
	s.guild.forget(match)
	s.match.forget(match)
	s.user.forget(match)
}

// ResetGuildStats godoc
// @Summary Reset Guild Stats
// @Description Delete every recorded game of the guild, for every player, like the /stats guild reset slash command.
// @Description It cannot be undone. Requires the guild owner or the Discord Administrator or Manage Server
// @Description permission.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags guild
// @Produce json
// @Param guildID query string true "Guild ID"
// @Success 200 {object} StatsReset
// @Failure 400 {object} HttpError
// @Failure 401 {object} HttpError
// @Failure 403 {object} HttpError
// @Failure 500 {object} HttpError
// @Router /guild/stats/reset [post]
func handleResetGuildStats(store Store, caches statsCaches) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "invalid guild ID"})
			return
		}
		games, err := store.ResetGuildStats(c.Request.Context(), guildID)
		// Forget even on failure: the delete may have happened before the error came back.
		caches.forgetGuild(guildID)
		if err != nil {
			log.Printf("Guild %s stats reset: %v\n", guildID, err)
			c.JSON(http.StatusInternalServerError, HttpError{StatusCode: http.StatusInternalServerError, Error: "Unable to reset guild statistics"})
			return
		}
		log.Printf("[API] Guild %s stats reset by %s: %d games deleted", guildID, requestActor(c), games)
		c.JSON(http.StatusOK, StatsReset{GuildID: guildID, Games: games})
	}
}

// ResetUserStats godoc
// @Summary Reset Player Stats
// @Description Remove one player from every recorded game of this guild. Their games in other guilds are kept, and
// @Description so are the games themselves, so the other players' stats do not change. It cannot be undone.
// @Description Requires the guild owner or the Discord Administrator or Manage Server permission, including to
// @Description reset one's own stats.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags guild
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param userID query string true "User ID"
// @Success 200 {object} StatsReset
// @Failure 400 {object} HttpError
// @Failure 401 {object} HttpError
// @Failure 403 {object} HttpError
// @Failure 500 {object} HttpError
// @Router /guild/user/reset [post]
func handleResetUserStats(store Store, caches statsCaches) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "invalid guild ID"})
			return
		}
		userID := c.Query("userID")
		if discord.ValidateSnowflake(userID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "invalid user ID"})
			return
		}
		games, err := store.ResetUserStats(c.Request.Context(), guildID, userID)
		caches.forgetGuild(guildID)
		if err != nil {
			log.Printf("Guild %s user %s stats reset: %v\n", guildID, userID, err)
			c.JSON(http.StatusInternalServerError, HttpError{StatusCode: http.StatusInternalServerError, Error: "Unable to reset player statistics"})
			return
		}
		log.Printf("[API] Guild %s stats for user %s reset by %s: removed from %d games", guildID, userID, requestActor(c), games)
		c.JSON(http.StatusOK, StatsReset{GuildID: guildID, UserID: userID, Games: games})
	}
}

// ResetGuildSettings godoc
// @Summary Reset Guild Settings
// @Description Put every setting of the guild back to its default, like the /settings reset slash command. This
// @Description includes the match summary channel and operator roles. Premium is not checked, since the defaults
// @Description need none. Requires the guild owner or the Discord Administrator or Manage Server permission, and
// @Description counts against the same per-guild budget as PATCH /guild/settings.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags guild
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param If-Match header string false "ETag from a previous GET or PATCH; the reset is refused with 412 if the settings changed since"
// @Success 200 {object} settings.GuildSettings "The default settings, now stored"
// @Header 200 {string} ETag "Version of the stored settings after the reset"
// @Failure 400 {object} HttpError
// @Failure 401 {object} HttpError
// @Failure 403 {object} HttpError
// @Failure 409 {object} HttpError "Settings were changed concurrently; reload and retry"
// @Failure 412 {object} HttpError "If-Match did not match the stored version"
// @Failure 429 {object} HttpError
// @Failure 503 {object} HttpError
// @Router /guild/settings/reset [post]
func handleResetGuildSettings(store Store) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "invalid guild ID"})
			return
		}
		retryAfter, err := store.ReserveSettingsWrite(ctx, guildID)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to reset guild settings"})
			return
		}
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			c.JSON(http.StatusTooManyRequests, HttpError{StatusCode: http.StatusTooManyRequests,
				Error: fmt.Sprintf("Too many settings changes for this guild; at most %d per %s", SettingsWriteLimit, SettingsWriteWindow)})
			return
		}
		_, version, err := store.Settings(ctx, guildID)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to load guild settings"})
			return
		}
		if !ifMatchAllows(c.GetHeader("If-Match"), version) {
			c.Header("ETag", settingsETag(version))
			c.JSON(http.StatusPreconditionFailed, HttpError{StatusCode: http.StatusPreconditionFailed,
				Error: "settings changed since they were loaded; reload and retry"})
			return
		}
		// Written over the row rather than deleting it, so the version keeps counting up and an ETag from before the
		// reset can never match the settings after it.
		defaults := settings.MakeGuildSettings()
		if err := store.SetSettings(ctx, guildID, defaults, version); err != nil {
			if errors.Is(err, storage.ErrSettingsConflict) {
				c.JSON(http.StatusConflict, HttpError{StatusCode: http.StatusConflict,
					Error: "settings were changed concurrently; reload and retry"})
				return
			}
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to reset guild settings"})
			return
		}
		log.Printf("[API] Guild %s settings reset to defaults by %s (version %d -> %d)", guildID, requestActor(c), version, version+1)
		c.Header("ETag", settingsETag(version+1))
		c.JSON(http.StatusOK, defaults)
	}
}
