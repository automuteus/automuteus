package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestDiscordChannelVerifier(t *testing.T) {
	const channel = "223456789012345678"
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr error
		want    ChannelInfo
	}{
		{"text channel", 200, `{"id":"` + channel + `","guild_id":"123456789012345678","type":0}`, nil,
			ChannelInfo{ID: channel, GuildID: "123456789012345678", Type: discordgo.ChannelTypeGuildText}},
		{"voice channel", 200, `{"id":"` + channel + `","guild_id":"123456789012345678","type":2}`, nil,
			ChannelInfo{ID: channel, GuildID: "123456789012345678", Type: discordgo.ChannelTypeGuildVoice}},
		{"unknown channel", 404, `{"message":"Unknown Channel"}`, errChannelNotFound, ChannelInfo{}},
		{"bot not in guild", 403, `{"message":"Missing Access"}`, errChannelNotFound, ChannelInfo{}},
		{"bad bot token", 401, `{"message":"401: Unauthorized"}`, errChannelUnavailable, ChannelInfo{}},
		{"rate limited", 429, `{"retry_after":1}`, errChannelUnavailable, ChannelInfo{}},
		{"outage", 500, ``, errChannelUnavailable, ChannelInfo{}},
		{"malformed reply", 200, `{`, errChannelUnavailable, ChannelInfo{}},
		{"reply for a different channel", 200, `{"id":"999","guild_id":"123456789012345678","type":0}`, errChannelUnavailable, ChannelInfo{}},
		{"DM channel has no guild", 200, `{"id":"` + channel + `","type":1}`, errChannelUnavailable, ChannelInfo{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/channels/"+channel {
					t.Errorf("path = %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bot bot-token" {
					t.Errorf("Authorization = %q, want the bot token", r.Header.Get("Authorization"))
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			v := &discordChannelVerifier{client: srv.Client(), baseURL: srv.URL, token: "bot-token"}
			got, err := v.VerifyChannel(context.Background(), channel)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("info = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// A malformed ID never reaches Discord.
func TestDiscordChannelVerifier_RejectsBadIDLocally(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Discord should not be called for a malformed channel ID")
	}))
	defer srv.Close()
	v := &discordChannelVerifier{client: srv.Client(), baseURL: srv.URL, token: "bot-token"}
	for _, id := range []string{"", "general", "<#223456789012345678>", "123"} {
		if _, err := v.VerifyChannel(context.Background(), id); !errors.Is(err, errChannelNotFound) {
			t.Errorf("%q: err = %v, want errChannelNotFound", id, err)
		}
	}
}
