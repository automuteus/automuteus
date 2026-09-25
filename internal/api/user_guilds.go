package api

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GuildEntry is one guild of the signed-in user, tagged with what the web UI's pickers filter on.
type GuildEntry struct {
	UserGuild
	// BotPresent is BotInGuild for this guild: whether the bot last saw itself join rather than leave.
	BotPresent bool `json:"botPresent"`
	// HasStats is whether the guild has a finished game for the stats page to show. It stays true after the bot
	// leaves, since the games remain until reset.
	HasStats bool `json:"hasStats"`
}

// GetUserGuilds godoc
// @Summary List the User's Guilds
// @Description List every guild the Discord token's user belongs to, as Discord reports them, each tagged with
// @Description whether the bot is in it and whether it has recorded games. The list comes from the token, so it
// @Description never reveals the bot's presence in guilds the caller is not a member of.
// @Security DiscordBearer
// @Tags user
// @Produce json
// @Success 200 {array} GuildEntry
// @Failure 401 {object} HttpError
// @Failure 403 {object} HttpError
// @Failure 501 {object} HttpError
// @Failure 503 {object} HttpError
// @Router /user/guilds [get]
func handleGetUserGuilds(lister GuildLister, store Store) func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if lister == nil {
			c.JSON(http.StatusNotImplemented, HttpError{StatusCode: http.StatusNotImplemented, Error: "Guild listing is not configured"})
			return
		}
		// Only a user token has a guild list; the admin password does not stand in for one here.
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Header("WWW-Authenticate", "Bearer")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		guilds, err := lister.ListGuilds(c.Request.Context(), parts[1])
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, errInvalidToken) {
				status = http.StatusUnauthorized
				c.Header("WWW-Authenticate", "Bearer")
			}
			if errors.Is(err, errDiscordForbidden) {
				status = http.StatusForbidden
			}
			c.JSON(status, HttpError{StatusCode: status, Error: "Unable to list Discord guilds"})
			return
		}
		ids := make([]string, len(guilds))
		for i, guild := range guilds {
			ids[i] = guild.ID
		}
		present, err := store.BotInGuilds(c.Request.Context(), ids)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to check bot membership"})
			return
		}
		stats, err := store.GuildsWithStats(c.Request.Context(), ids)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to check guild stats"})
			return
		}
		entries := make([]GuildEntry, len(guilds))
		for i, guild := range guilds {
			entries[i] = GuildEntry{UserGuild: guild, BotPresent: present[i], HasStats: stats[i]}
		}
		c.JSON(http.StatusOK, entries)
	}
}
