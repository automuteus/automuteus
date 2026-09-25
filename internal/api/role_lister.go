package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sort"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

// GuildRole is a role a client may pick as a bot operator role: enough to show it the way Discord does.
type GuildRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Color is Discord's integer RGB value; zero means the role has no colour.
	Color int `json:"color"`
	// Position is Discord's display order; higher is listed first.
	Position int `json:"position"`
	// Managed roles belong to an integration or bot and cannot be given to members by hand.
	Managed bool `json:"managed"`
}

// RoleLister lists a guild's roles with the bot's own credentials. The caller's token cannot be trusted to name
// roles, and a role ID typed by hand is exactly what must be checked. Implementations fail closed.
type RoleLister interface {
	ListRoles(ctx context.Context, guildID string) ([]GuildRole, error)
}

// ListRoles returns the guild's roles in display order, without @everyone: it is never in a member's role list, so
// it can never match as an operator role. errChannelNotFound means the bot is not in the guild.
func (v *discordChannelVerifier) ListRoles(ctx context.Context, guildID string) ([]GuildRole, error) {
	if discord.ValidateSnowflake(guildID) != nil {
		return nil, errChannelNotFound
	}
	var roles []discordgo.Role
	if err := v.get(ctx, "/guilds/"+guildID+"/roles", &roles); err != nil {
		return nil, err
	}
	out := make([]GuildRole, 0, len(roles))
	for _, role := range roles {
		if role.ID == guildID || discord.ValidateSnowflake(role.ID) != nil {
			continue
		}
		out = append(out, GuildRole{ID: role.ID, Name: role.Name, Color: role.Color, Position: role.Position, Managed: role.Managed})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Position > out[j].Position })
	return out, nil
}

// unknownOperatorRoles returns a field error for every ID in ids that is not one of the guild's roles, using the
// same indexed paths as settings validation so a client can mark the offending entry.
func unknownOperatorRoles(ctx context.Context, roles RoleLister, guildID string, ids []string) ([]settings.FieldError, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	list, err := roles.ListRoles(ctx, guildID)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(list))
	for _, role := range list {
		known[role.ID] = true
	}
	var fields []settings.FieldError
	for i, id := range ids {
		if !known[id] {
			fields = append(fields, settings.FieldError{Field: "permissionRoleIDs[" + itoa(i) + "]", Message: "is not a role in this server"})
		}
	}
	return fields, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// GetGuildRoles godoc
// @Summary List Guild Roles
// @Description The guild's roles as the bot sees them, in Discord's display order and without @everyone, so a client
// @Description can offer a picker for bot operator roles and show names instead of IDs. PATCH /guild/settings rejects a
// @Description permissionRoleIDs entry that is not one of these. Any member of the guild may ask.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Success 200 {array} GuildRole
// @Failure 400 {object} HttpError
// @Failure 404 {object} HttpError "The bot is not in this guild"
// @Failure 501 {object} HttpError "The API is not configured with DISCORD_BOT_TOKEN"
// @Failure 503 {object} HttpError
// @Router /guild/roles [get]
func handleGetGuildRoles(roles RoleLister) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "invalid guild ID"})
			return
		}
		if roles == nil {
			c.JSON(http.StatusNotImplemented, HttpError{StatusCode: http.StatusNotImplemented,
				Error: "listing roles requires the API to be configured with DISCORD_BOT_TOKEN"})
			return
		}
		list, err := roles.ListRoles(c.Request.Context(), guildID)
		switch {
		case errors.Is(err, errChannelNotFound):
			c.JSON(http.StatusNotFound, HttpError{StatusCode: http.StatusNotFound, Error: "the bot is not in this guild"})
			return
		case err != nil:
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to list roles"})
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
