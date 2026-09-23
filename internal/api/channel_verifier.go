package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/bwmarrin/discordgo"
)

// ChannelInfo is what the API needs to know about a channel a client wants the
// bot to post into: which guild it belongs to and whether it can hold messages.
type ChannelInfo struct {
	ID      string                `json:"id"`
	GuildID string                `json:"guild_id"`
	Type    discordgo.ChannelType `json:"type"`
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
}

func newDiscordChannelVerifier(token string) *discordChannelVerifier {
	return &discordChannelVerifier{client: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, baseURL: "https://discord.com/api/v10", token: token}
}

func (v *discordChannelVerifier) VerifyChannel(ctx context.Context, channelID string) (ChannelInfo, error) {
	if discord.ValidateSnowflake(channelID) != nil {
		return ChannelInfo{}, errChannelNotFound
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/channels/"+channelID, nil)
	if err != nil {
		return ChannelInfo{}, errChannelUnavailable
	}
	req.Header.Set("Authorization", "Bot "+v.token)
	resp, err := v.client.Do(req)
	if err != nil {
		return ChannelInfo{}, errChannelUnavailable
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusForbidden:
		return ChannelInfo{}, errChannelNotFound
	default:
		return ChannelInfo{}, errChannelUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(body) > 64<<10 {
		return ChannelInfo{}, errChannelUnavailable
	}
	var info ChannelInfo
	if json.Unmarshal(body, &info) != nil || info.ID != channelID || discord.ValidateSnowflake(info.GuildID) != nil {
		return ChannelInfo{}, errChannelUnavailable
	}
	return info, nil
}
