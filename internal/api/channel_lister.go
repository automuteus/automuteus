package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sort"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

// GuildChannel is a channel a client may pick as the match summary destination, with the same verdict about it
// that GET /guild/channel and PATCH /guild/settings would give, so a picker can grey out channels the bot cannot
// post in and say why.
type GuildChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Type is Discord's channel type: 0 for a text channel, 5 for an announcement channel. Threads are never
	// listed; Discord does not include them in a guild's channel list.
	Type discordgo.ChannelType `json:"type"`
	// Category is the name of the parent category, empty for a channel at the top level.
	Category string `json:"category"`
	// Problems is empty exactly when OK is true; each entry is a sentence a client can show for the channel.
	OK       bool     `json:"ok"`
	Problems []string `json:"problems"`
}

// ChannelLister lists a guild's text-capable channels with the bot's own credentials, in Discord's display order,
// each judged as a summary destination. Implementations fail closed.
type ChannelLister interface {
	ListChannels(ctx context.Context, guildID string) ([]GuildChannel, error)
}

// ListChannels fetches the guild's channels, roles, and the bot's member record (three calls, plus the bot's own ID
// once) and judges every text or announcement channel the way checkSummaryChannel would. errChannelNotFound means
// the bot is not in the guild.
func (v *discordChannelVerifier) ListChannels(ctx context.Context, guildID string) ([]GuildChannel, error) {
	if discord.ValidateSnowflake(guildID) != nil {
		return nil, errChannelNotFound
	}
	var channels []discordgo.Channel
	if err := v.get(ctx, "/guilds/"+guildID+"/channels", &channels); err != nil {
		return nil, err
	}
	botID, err := v.self(ctx)
	if err != nil {
		return nil, err
	}
	var guild discordgo.Guild
	if err := v.get(ctx, "/guilds/"+guildID, &guild); err != nil {
		return nil, err
	}
	if guild.ID != guildID {
		return nil, errChannelUnavailable
	}
	// The bot may have been removed between the channel listing and this lookup. It then holds no permissions
	// at all, the same verdict VerifyChannel gives: not even what @everyone grants, so every channel is listed
	// and none is postable.
	var member discordgo.Member
	isMember := true
	if err := v.get(ctx, "/guilds/"+guildID+"/members/"+botID, &member); err != nil {
		if !errors.Is(err, errChannelNotFound) {
			return nil, err
		}
		isMember = false
	}

	type category struct {
		name     string
		position int
	}
	categories := map[string]category{}
	for _, ch := range channels {
		if ch.Type == discordgo.ChannelTypeGuildCategory && discord.ValidateSnowflake(ch.ID) == nil {
			categories[ch.ID] = category{name: ch.Name, position: ch.Position}
		}
	}
	type ordered struct {
		channel  GuildChannel
		category int // -1 for the top level, which Discord shows first
		position int
	}
	var out []ordered
	for i := range channels {
		ch := &channels[i]
		// Discord may omit guild_id from a guild's channel list, since the guild is already named by the URL;
		// only an entry that names a different guild is foreign.
		if !discord.IsTextCapableChannel(ch.Type) || discord.ValidateSnowflake(ch.ID) != nil || (ch.GuildID != "" && ch.GuildID != guildID) {
			continue
		}
		entry := ordered{channel: GuildChannel{ID: ch.ID, Name: ch.Name, Type: ch.Type, Problems: []string{}}, category: -1, position: ch.Position}
		if parent, ok := categories[ch.ParentID]; ok {
			entry.channel.Category = parent.name
			entry.category = parent.position
		}
		var permissions int64
		if isMember {
			permissions = memberChannelPermissions(&guild, ch, botID, member.Roles)
		}
		if problem := summaryProblem(ch.Type, permissions); problem != "" {
			entry.channel.Problems = append(entry.channel.Problems, problem)
		} else {
			entry.channel.OK = true
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].category != out[j].category {
			return out[i].category < out[j].category
		}
		return out[i].position < out[j].position
	})
	list := make([]GuildChannel, 0, len(out))
	for _, entry := range out {
		list = append(list, entry.channel)
	}
	return list, nil
}

// GetGuildChannels godoc
// @Summary List Guild Channels
// @Description The guild's text and announcement channels as the bot sees them, in Discord's display order with
// @Description their category, each marked with whether the bot could post match summaries there and, if not, why:
// @Description the same verdict GET /guild/channel gives for one channel and PATCH /guild/settings applies on save.
// @Description Threads are not listed; one can still be set by ID. Channel names can be private, so this requires
// @Description the same permission as changing settings: the guild owner, or Administrator or Manage Server.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Success 200 {array} GuildChannel
// @Failure 400 {object} HttpError
// @Failure 403 {object} HttpError "The caller may not change this guild's settings"
// @Failure 404 {object} HttpError "The bot is not in this guild"
// @Failure 501 {object} HttpError "The API is not configured with DISCORD_BOT_TOKEN"
// @Failure 503 {object} HttpError
// @Router /guild/channels [get]
func handleGetGuildChannels(channels ChannelLister) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "invalid guild ID"})
			return
		}
		if channels == nil {
			c.JSON(http.StatusNotImplemented, HttpError{StatusCode: http.StatusNotImplemented,
				Error: "listing channels requires the API to be configured with DISCORD_BOT_TOKEN"})
			return
		}
		list, err := channels.ListChannels(c.Request.Context(), guildID)
		switch {
		case errors.Is(err, errChannelNotFound):
			c.JSON(http.StatusNotFound, HttpError{StatusCode: http.StatusNotFound, Error: "the bot is not in this guild"})
			return
		case err != nil:
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to list channels"})
			return
		}
		c.JSON(http.StatusOK, list)
	}
}
