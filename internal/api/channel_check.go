package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

// ChannelCheck is the answer to "can the bot post match summaries into this channel of this guild?". Problems is
// empty exactly when OK is true; each entry is a sentence a client can show next to the field.
type ChannelCheck struct {
	ID string `json:"id"`
	// Name is the channel name without the leading '#'. Omitted when the channel is not in the requested guild,
	// so a caller cannot use this route to learn channel names in guilds it shares with the bot but not with us.
	Name     string   `json:"name,omitempty"`
	OK       bool     `json:"ok"`
	Problems []string `json:"problems"`
}

type permission struct {
	bit  int64
	name string
}

// summaryPermissions are what posting a match summary embed needs. In a thread Discord checks Send Messages in
// Threads instead of Send Messages, so a bot that may only reply in threads of an announcement channel is fine.
func summaryPermissions(t discordgo.ChannelType) []permission {
	send := permission{discordgo.PermissionSendMessages, "Send Messages"}
	if isThreadType(t) {
		send = permission{discordgo.PermissionSendMessagesInThreads, "Send Messages in Threads"}
	}
	return []permission{{discordgo.PermissionViewChannel, "View Channel"}, send, {discordgo.PermissionEmbedLinks, "Embed Links"}}
}

const (
	problemNotFound = "channel not found, or the bot cannot see it"
	problemGuild    = "must be a channel in this guild"
	problemType     = "must be a text or announcement channel, or a thread in one"
)

// checkSummaryChannel resolves a channel with the bot's credentials and reports whether it is a usable summary
// destination for guildID. Only a lookup failure is an error; every judgement about the channel itself comes back
// in the check so PATCH and the lookup route say the same things.
func checkSummaryChannel(ctx context.Context, channels ChannelVerifier, guildID, channelID string) (ChannelCheck, error) {
	check := ChannelCheck{ID: channelID, Problems: []string{}}
	info, err := channels.VerifyChannel(ctx, channelID)
	switch {
	case errors.Is(err, errChannelNotFound):
		check.Problems = append(check.Problems, problemNotFound)
		return check, nil
	case err != nil:
		return ChannelCheck{}, err
	case info.GuildID != guildID:
		check.Problems = append(check.Problems, problemGuild)
		return check, nil
	}
	check.Name = info.Name
	if !discord.IsTextCapableChannel(info.Type) {
		check.Problems = append(check.Problems, problemType)
		return check, nil
	}
	var missing []string
	for _, p := range summaryPermissions(info.Type) {
		if info.Permissions&p.bit == 0 {
			missing = append(missing, p.name)
		}
	}
	if len(missing) > 0 {
		check.Problems = append(check.Problems, "the bot is missing the "+strings.Join(missing, ", ")+" permission"+plural(len(missing))+" in this channel")
		return check, nil
	}
	check.OK = true
	return check, nil
}

func isThreadType(t discordgo.ChannelType) bool {
	return t == discordgo.ChannelTypeGuildNewsThread || t == discordgo.ChannelTypeGuildPublicThread || t == discordgo.ChannelTypeGuildPrivateThread
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// GetGuildChannel godoc
// @Summary Check a Summary Channel
// @Description Report whether the bot could post match summaries into a channel of the given guild: the channel
// @Description must exist and be visible to the bot, belong to the guild, be a text or announcement channel or a
// @Description thread in one, and grant the bot View Channel, Embed Links, and Send Messages (Send Messages in Threads
// @Description for a thread). The same checks are applied when PATCH /guild/settings changes
// @Description matchSummaryChannelID, so a client can validate before saving. Any member of the guild may ask.
// @Security BasicAuth
// @Security DiscordBearer
// @Tags guild
// @Accept json
// @Produce json
// @Param guildID query string true "Guild ID"
// @Param channelID query string true "Channel ID"
// @Success 200 {object} ChannelCheck
// @Failure 400 {object} HttpError
// @Failure 501 {object} HttpError "The API is not configured with DISCORD_BOT_TOKEN"
// @Failure 503 {object} HttpError
// @Router /guild/channel [get]
func handleGetGuildChannel(channels ChannelVerifier) func(c *gin.Context) {
	return func(c *gin.Context) {
		guildID := c.Query("guildID")
		if discord.ValidateSnowflake(guildID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "invalid guild ID"})
			return
		}
		channelID := c.Query("channelID")
		if discord.ValidateSnowflake(channelID) != nil {
			c.JSON(http.StatusBadRequest, HttpError{StatusCode: http.StatusBadRequest, Error: "invalid channel ID"})
			return
		}
		if channels == nil {
			c.JSON(http.StatusNotImplemented, HttpError{StatusCode: http.StatusNotImplemented,
				Error: "checking channels requires the API to be configured with DISCORD_BOT_TOKEN"})
			return
		}
		check, err := checkSummaryChannel(c.Request.Context(), channels, guildID, channelID)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusServiceUnavailable, HttpError{StatusCode: http.StatusServiceUnavailable, Error: "Unable to check channel"})
			return
		}
		c.JSON(http.StatusOK, check)
	}
}
