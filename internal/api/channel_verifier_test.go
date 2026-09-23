package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

const (
	verifyGuild   = "123456789012345678"
	verifyChannel = "223456789012345678"
	verifyParent  = "223456789012345600"
	verifyBot     = "553456789012345678"
	everyoneRole  = verifyGuild
	postingRole   = "663456789012345678"
	postBits      = discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks
)

// discordStub answers the handful of bot-token GETs the verifier makes. Unlisted paths are 404 so a test that
// forgets a fixture fails as "not found" rather than hanging.
type discordStub struct {
	replies map[string]struct {
		status int
		body   string
	}
	calls []string
}

func (d *discordStub) reply(path string, status int, body string) *discordStub {
	if d.replies == nil {
		d.replies = map[string]struct {
			status int
			body   string
		}{}
	}
	d.replies[path] = struct {
		status int
		body   string
	}{status, body}
	return d
}

func (d *discordStub) server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bot bot-token" {
			t.Errorf("Authorization = %q, want the bot token", r.Header.Get("Authorization"))
		}
		d.calls = append(d.calls, r.URL.Path)
		rep, ok := d.replies[r.URL.Path]
		if !ok {
			w.WriteHeader(404)
			fmt.Fprint(w, `{"message":"Unknown"}`)
			return
		}
		w.WriteHeader(rep.status)
		fmt.Fprint(w, rep.body)
	}))
}

// happyStub is a text channel in a guild where the bot's role grants posting.
func happyStub() *discordStub {
	return (&discordStub{}).
		reply("/channels/"+verifyChannel, 200, `{"id":"`+verifyChannel+`","guild_id":"`+verifyGuild+`","type":0,"name":"match-summaries","permission_overwrites":[]}`).
		reply("/users/@me", 200, `{"id":"`+verifyBot+`","bot":true}`).
		reply("/guilds/"+verifyGuild, 200, `{"id":"`+verifyGuild+`","owner_id":"999","roles":[{"id":"`+everyoneRole+`","permissions":"0"},{"id":"`+postingRole+`","permissions":"`+fmt.Sprint(postBits)+`"}]}`).
		reply("/guilds/"+verifyGuild+"/members/"+verifyBot, 200, `{"user":{"id":"`+verifyBot+`"},"roles":["`+postingRole+`"]}`)
}

func verifyWith(t *testing.T, stub *discordStub) (ChannelInfo, error, *discordStub) {
	t.Helper()
	srv := stub.server(t)
	defer srv.Close()
	v := &discordChannelVerifier{client: srv.Client(), baseURL: srv.URL, token: "bot-token"}
	info, err := v.VerifyChannel(context.Background(), verifyChannel)
	return info, err, stub
}

func TestDiscordChannelVerifier_ResolvesChannelAndBotPermissions(t *testing.T) {
	info, err, stub := verifyWith(t, happyStub())
	if err != nil {
		t.Fatal(err)
	}
	want := ChannelInfo{ID: verifyChannel, GuildID: verifyGuild, Type: discordgo.ChannelTypeGuildText, Name: "match-summaries", Permissions: postBits}
	if info != want {
		t.Fatalf("info = %+v, want %+v", info, want)
	}
	if got := strings.Join(stub.calls, " "); got != "/channels/"+verifyChannel+" /users/@me /guilds/"+verifyGuild+" /guilds/"+verifyGuild+"/members/"+verifyBot {
		t.Errorf("calls = %s", got)
	}
}

func TestDiscordChannelVerifier_PermissionAlgorithm(t *testing.T) {
	overwrite := func(id string, kind int, allow, deny int64) string {
		return fmt.Sprintf(`{"id":"%s","type":%d,"allow":"%d","deny":"%d"}`, id, kind, allow, deny)
	}
	channel := func(overwrites ...string) string {
		return `{"id":"` + verifyChannel + `","guild_id":"` + verifyGuild + `","type":0,"name":"c","permission_overwrites":[` + strings.Join(overwrites, ",") + `]}`
	}
	guild := func(ownerID string, everyone, role int64) string {
		return fmt.Sprintf(`{"id":"%s","owner_id":"%s","roles":[{"id":"%s","permissions":"%d"},{"id":"%s","permissions":"%d"}]}`, verifyGuild, ownerID, everyoneRole, everyone, postingRole, role)
	}
	for _, tc := range []struct {
		name    string
		channel string
		guild   string
		want    int64 // bits of postBits the bot should hold
	}{
		{"everyone grants", channel(), guild("999", postBits, 0), postBits},
		{"role grants", channel(), guild("999", 0, postBits), postBits},
		{"nothing grants", channel(), guild("999", 0, 0), 0},
		{"everyone overwrite denies", channel(overwrite(everyoneRole, 0, 0, discordgo.PermissionSendMessages)), guild("999", postBits, 0), postBits &^ discordgo.PermissionSendMessages},
		{"role overwrite re-allows", channel(overwrite(everyoneRole, 0, 0, postBits), overwrite(postingRole, 0, postBits, 0)), guild("999", postBits, 0), postBits},
		{"member overwrite wins", channel(overwrite(postingRole, 0, postBits, 0), overwrite(verifyBot, 1, 0, discordgo.PermissionEmbedLinks)), guild("999", 0, 0), postBits &^ discordgo.PermissionEmbedLinks},
		{"administrator ignores overwrites", channel(overwrite(everyoneRole, 0, 0, postBits)), guild("999", 0, discordgo.PermissionAdministrator), postBits},
		{"owner ignores everything", channel(overwrite(everyoneRole, 0, 0, postBits)), guild(verifyBot, 0, 0), postBits},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := happyStub().reply("/channels/"+verifyChannel, 200, tc.channel).reply("/guilds/"+verifyGuild, 200, tc.guild)
			info, err, _ := verifyWith(t, stub)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Permissions & postBits; got != tc.want {
				t.Errorf("permissions & post = %d, want %d", got, tc.want)
			}
			if strings.HasPrefix(tc.name, "administrator") || strings.HasPrefix(tc.name, "owner") {
				if info.Permissions&discordgo.PermissionSendMessagesInThreads == 0 {
					t.Error("owner/administrator must hold thread posting too")
				}
			}
		})
	}
}

func TestDiscordChannelVerifier_ThreadUsesParentOverwrites(t *testing.T) {
	stub := happyStub().
		reply("/channels/"+verifyChannel, 200, `{"id":"`+verifyChannel+`","guild_id":"`+verifyGuild+`","type":11,"name":"game-1","parent_id":"`+verifyParent+`"}`).
		reply("/channels/"+verifyParent, 200, `{"id":"`+verifyParent+`","guild_id":"`+verifyGuild+`","type":0,"name":"summaries","permission_overwrites":[{"id":"`+everyoneRole+`","type":0,"allow":"0","deny":"`+fmt.Sprint(discordgo.PermissionEmbedLinks)+`"}]}`)
	info, err, s := verifyWith(t, stub)
	if err != nil {
		t.Fatal(err)
	}
	if info.Type != discordgo.ChannelTypeGuildPublicThread || info.Name != "game-1" {
		t.Errorf("info = %+v", info)
	}
	if info.Permissions&discordgo.PermissionEmbedLinks != 0 || info.Permissions&discordgo.PermissionSendMessages == 0 {
		t.Errorf("parent overwrite not applied: %d", info.Permissions)
	}
	if !strings.Contains(strings.Join(s.calls, " "), "/channels/"+verifyParent) {
		t.Error("parent channel was not fetched")
	}
	// A thread whose parent cannot be fetched is not vouched for.
	if _, err, _ := verifyWith(t, stub.reply("/channels/"+verifyParent, 404, `{}`)); !errors.Is(err, errChannelNotFound) {
		t.Errorf("missing parent: err = %v", err)
	}
}

func TestDiscordChannelVerifier_BotOutsideGuildHasNoPermissions(t *testing.T) {
	info, err, _ := verifyWith(t, happyStub().reply("/guilds/"+verifyGuild+"/members/"+verifyBot, 404, `{"message":"Unknown Member"}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Permissions != 0 || info.GuildID != verifyGuild {
		t.Errorf("info = %+v", info)
	}
}

func TestDiscordChannelVerifier_CachesBotID(t *testing.T) {
	stub := happyStub()
	srv := stub.server(t)
	defer srv.Close()
	v := &discordChannelVerifier{client: srv.Client(), baseURL: srv.URL, token: "bot-token"}
	for i := 0; i < 3; i++ {
		if _, err := v.VerifyChannel(context.Background(), verifyChannel); err != nil {
			t.Fatal(err)
		}
	}
	if n := strings.Count(strings.Join(stub.calls, " "), "/users/@me"); n != 1 {
		t.Errorf("/users/@me fetched %d times, want 1", n)
	}
}

func TestDiscordChannelVerifier_Failures(t *testing.T) {
	for _, tc := range []struct {
		name string
		stub *discordStub
		want error
	}{
		{"unknown channel", happyStub().reply("/channels/"+verifyChannel, 404, `{"message":"Unknown Channel"}`), errChannelNotFound},
		{"bot cannot see channel", happyStub().reply("/channels/"+verifyChannel, 403, `{"message":"Missing Access"}`), errChannelNotFound},
		{"bad bot token", happyStub().reply("/channels/"+verifyChannel, 401, `{"message":"401: Unauthorized"}`), errChannelUnavailable},
		{"rate limited", happyStub().reply("/channels/"+verifyChannel, 429, `{"retry_after":1}`), errChannelUnavailable},
		{"outage", happyStub().reply("/channels/"+verifyChannel, 500, ``), errChannelUnavailable},
		{"malformed channel", happyStub().reply("/channels/"+verifyChannel, 200, `{`), errChannelUnavailable},
		{"reply for a different channel", happyStub().reply("/channels/"+verifyChannel, 200, `{"id":"999","guild_id":"`+verifyGuild+`","type":0}`), errChannelUnavailable},
		{"DM channel has no guild", happyStub().reply("/channels/"+verifyChannel, 200, `{"id":"`+verifyChannel+`","type":1}`), errChannelUnavailable},
		{"self lookup forbidden is a token problem", happyStub().reply("/users/@me", 403, `{}`), errChannelUnavailable},
		{"guild lookup fails", happyStub().reply("/guilds/"+verifyGuild, 500, ``), errChannelUnavailable},
		{"guild reply mismatched", happyStub().reply("/guilds/"+verifyGuild, 200, `{"id":"1","roles":[]}`), errChannelUnavailable},
		{"member lookup outage", happyStub().reply("/guilds/"+verifyGuild+"/members/"+verifyBot, 500, ``), errChannelUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info, err, _ := verifyWith(t, tc.stub)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if info != (ChannelInfo{}) {
				t.Errorf("info = %+v, want zero", info)
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
