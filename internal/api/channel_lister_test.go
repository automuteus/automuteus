package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

const (
	gamesCategory = "773456789012345601"
	staffCategory = "773456789012345602"
	generalText   = "773456789012345611"
	summariesText = "773456789012345612"
	newsChannel   = "773456789012345613"
	lobbyVoice    = "773456789012345614"
	modText       = "773456789012345615"
)

// channelListStub is happyStub plus a channel list: one top-level text channel (without guild_id, which Discord
// may omit from a guild's channel list), a Staff category (position 0) with a channel the bot cannot see, and a
// Games category (position 1) with a postable text channel, an announcement channel missing Embed Links, and a
// voice channel. Foreign, malformed, and category entries are there to be skipped.
func channelListStub() *discordStub {
	deny := func(bits int64) string {
		return fmt.Sprintf(`"permission_overwrites":[{"id":"%s","type":0,"allow":"0","deny":"%d"}]`, everyoneRole, bits)
	}
	return happyStub().reply("/guilds/"+verifyGuild+"/channels", 200, `[
		{"id":"`+gamesCategory+`","guild_id":"`+verifyGuild+`","type":4,"name":"Games","position":1},
		{"id":"`+newsChannel+`","guild_id":"`+verifyGuild+`","type":5,"name":"announcements","position":1,"parent_id":"`+gamesCategory+`",`+deny(discordgo.PermissionEmbedLinks)+`},
		{"id":"`+lobbyVoice+`","guild_id":"`+verifyGuild+`","type":2,"name":"Lobby","position":0,"parent_id":"`+gamesCategory+`"},
		{"id":"`+summariesText+`","guild_id":"`+verifyGuild+`","type":0,"name":"summaries","position":0,"parent_id":"`+gamesCategory+`"},
		{"id":"`+staffCategory+`","guild_id":"`+verifyGuild+`","type":4,"name":"Staff","position":0},
		{"id":"`+modText+`","guild_id":"`+verifyGuild+`","type":0,"name":"mod-chat","position":0,"parent_id":"`+staffCategory+`",`+deny(discordgo.PermissionViewChannel|discordgo.PermissionSendMessages)+`},
		{"id":"`+generalText+`","type":0,"name":"general","position":3},
		{"id":"883456789012345678","guild_id":"923456789012345678","type":0,"name":"elsewhere","position":0},
		{"id":"nope","guild_id":"`+verifyGuild+`","type":0,"name":"bad id","position":0}
	]`)
}

func listWith(t *testing.T, stub *discordStub) ([]GuildChannel, error, *discordStub) {
	t.Helper()
	srv := stub.server(t)
	defer srv.Close()
	v := &discordChannelVerifier{client: srv.Client(), baseURL: srv.URL, token: "bot-token"}
	list, err := v.ListChannels(context.Background(), verifyGuild)
	return list, err, stub
}

func TestDiscordChannelLister_OrdersAndJudgesChannels(t *testing.T) {
	list, err, stub := listWith(t, channelListStub())
	if err != nil {
		t.Fatal(err)
	}
	want := []GuildChannel{
		{ID: generalText, Name: "general", Type: discordgo.ChannelTypeGuildText, OK: true, Problems: []string{}},
		{ID: modText, Name: "mod-chat", Type: discordgo.ChannelTypeGuildText, Category: "Staff", Problems: []string{"the bot is missing the View Channel, Send Messages permissions in this channel"}},
		{ID: summariesText, Name: "summaries", Type: discordgo.ChannelTypeGuildText, Category: "Games", OK: true, Problems: []string{}},
		{ID: newsChannel, Name: "announcements", Type: discordgo.ChannelTypeGuildNews, Category: "Games", Problems: []string{"the bot is missing the Embed Links permission in this channel"}},
	}
	if len(list) != len(want) {
		t.Fatalf("list = %+v", list)
	}
	for i := range want {
		if list[i].ID != want[i].ID || list[i].Name != want[i].Name || list[i].Type != want[i].Type || list[i].Category != want[i].Category || list[i].OK != want[i].OK || strings.Join(list[i].Problems, "|") != strings.Join(want[i].Problems, "|") {
			t.Errorf("list[%d] = %+v, want %+v", i, list[i], want[i])
		}
		if list[i].Problems == nil {
			t.Errorf("list[%d]: problems must serialise as [] not null", i)
		}
	}
	if got := strings.Join(stub.calls, " "); got != "/guilds/"+verifyGuild+"/channels /users/@me /guilds/"+verifyGuild+" /guilds/"+verifyGuild+"/members/"+verifyBot {
		t.Errorf("calls = %s", got)
	}
}

func TestDiscordChannelLister_BotOutsideGuildListsNothingPostable(t *testing.T) {
	// @everyone can post here, which must not count for a bot that is no longer a member.
	everyoneCanPost := `{"id":"` + verifyGuild + `","owner_id":"999","roles":[{"id":"` + everyoneRole + `","permissions":"` + fmt.Sprint(postBits) + `"},{"id":"` + postingRole + `","permissions":"` + fmt.Sprint(postBits) + `"}]}`
	for name, status := range map[string]int{"unknown member": 404, "missing access": 403} {
		t.Run(name, func(t *testing.T) {
			list, err, _ := listWith(t, channelListStub().
				reply("/guilds/"+verifyGuild, 200, everyoneCanPost).
				reply("/guilds/"+verifyGuild+"/members/"+verifyBot, status, `{"message":"Unknown Member"}`))
			if err != nil {
				t.Fatal(err)
			}
			if len(list) != 4 {
				t.Fatalf("list = %+v", list)
			}
			for _, ch := range list {
				if ch.OK || len(ch.Problems) != 1 {
					t.Errorf("%s should not be postable: %+v", ch.Name, ch)
				}
			}
		})
	}
}

func TestDiscordChannelLister_Failures(t *testing.T) {
	for _, tc := range []struct {
		name string
		stub *discordStub
		want error
	}{
		{"bot not in guild", channelListStub().reply("/guilds/"+verifyGuild+"/channels", 403, `{"message":"Missing Access"}`), errChannelNotFound},
		{"unknown guild", channelListStub().reply("/guilds/"+verifyGuild+"/channels", 404, `{}`), errChannelNotFound},
		{"outage", channelListStub().reply("/guilds/"+verifyGuild+"/channels", 500, ``), errChannelUnavailable},
		{"malformed list", channelListStub().reply("/guilds/"+verifyGuild+"/channels", 200, `{"not":"a list"}`), errChannelUnavailable},
		{"self lookup fails", channelListStub().reply("/users/@me", 403, `{}`), errChannelUnavailable},
		{"guild lookup fails", channelListStub().reply("/guilds/"+verifyGuild, 500, ``), errChannelUnavailable},
		{"guild reply mismatched", channelListStub().reply("/guilds/"+verifyGuild, 200, `{"id":"1","roles":[]}`), errChannelUnavailable},
		{"member lookup outage", channelListStub().reply("/guilds/"+verifyGuild+"/members/"+verifyBot, 500, ``), errChannelUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			list, err, _ := listWith(t, tc.stub)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if list != nil {
				t.Errorf("list = %+v, want nil", list)
			}
		})
	}
	srv := channelListStub().server(t)
	defer srv.Close()
	v := &discordChannelVerifier{client: srv.Client(), baseURL: srv.URL, token: "bot-token"}
	if _, err := v.ListChannels(context.Background(), "general"); !errors.Is(err, errChannelNotFound) {
		t.Errorf("bad guild ID should be refused locally: %v", err)
	}
}

// channelListerFunc adapts a function to ChannelLister for tests.
type channelListerFunc func(ctx context.Context, guildID string) ([]GuildChannel, error)

func (f channelListerFunc) ListChannels(ctx context.Context, guildID string) ([]GuildChannel, error) {
	return f(ctx, guildID)
}

func fakeChannelList(calls *int) channelListerFunc {
	return func(_ context.Context, guildID string) ([]GuildChannel, error) {
		if calls != nil {
			*calls++
		}
		if guildID != writeGuild {
			return nil, errChannelNotFound
		}
		return []GuildChannel{
			{ID: textChannelHere, Name: "match-summaries", Type: discordgo.ChannelTypeGuildText, OK: true, Problems: []string{}},
			{ID: textNoPostHere, Name: "read-only", Type: discordgo.ChannelTypeGuildText, Category: "Info", Problems: []string{"the bot is missing the Send Messages, Embed Links permissions in this channel"}},
		}, nil
	}
}

func writeRouterChannels(s *fakeStore, access VerifiedGuildAccess, channels ChannelLister) http.Handler {
	v := verifierFunc(func(_ context.Context, token, guild string) (VerifiedGuildAccess, error) {
		if token != "valid" {
			return VerifiedGuildAccess{}, errInvalidToken
		}
		if guild != access.GuildID {
			return VerifiedGuildAccess{UserID: access.UserID, GuildID: guild}, nil
		}
		return access, nil
	})
	return NewRouter(Config{GuildVerifier: v, AdminPassword: "test-password", ChannelLister: channels}, s)
}

func TestGuildChannelsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	path := "/guild/channels?guildID=" + writeGuild
	manager := memberAccess
	manager.Permissions = discordgo.PermissionManageServer
	for name, access := range map[string]VerifiedGuildAccess{"owner": ownerAccess, "administrator": adminAccess, "manage server": manager} {
		w := bearerRequest(writeRouterChannels(&fakeStore{}, access, fakeChannelList(nil)), path, "valid")
		var list []GuildChannel
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &list) != nil || len(list) != 2 || list[0].Name != "match-summaries" || !list[0].OK || list[1].OK || list[1].Category != "Info" {
			t.Fatalf("%s: got %d %s", name, w.Code, w.Body)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: missing cache protection", name)
		}
		if !strings.Contains(w.Body.String(), `"problems":[]`) {
			t.Errorf("%s: a postable channel must carry an empty problems list: %s", name, w.Body)
		}
	}

	// Repeated requests for the same guild are served from the list cache.
	calls := 0
	cached := writeRouterChannels(&fakeStore{}, ownerAccess, fakeChannelList(&calls))
	for i := 0; i < 3; i++ {
		if w := bearerRequest(cached, path, "valid"); w.Code != 200 {
			t.Fatalf("cached request %d: %d", i, w.Code)
		}
	}
	if calls != 1 {
		t.Errorf("channels fetched %d times for three requests, want 1", calls)
	}

	// Channel names can be private: a member who may read settings but not write them is refused, and the
	// lister is never asked.
	calls = 0
	if w := bearerRequest(writeRouterChannels(&fakeStore{}, memberAccess, fakeChannelList(&calls)), path, "valid"); w.Code != 403 || calls != 0 {
		t.Errorf("moderator without a settings permission: %d, lister calls %d", w.Code, calls)
	}
	departed := VerifiedGuildAccess{UserID: "999", GuildID: writeGuild, Owner: true}
	if w := bearerRequest(writeRouterChannels(&fakeStore{}, departed, fakeChannelList(&calls)), path, "valid"); w.Code != 403 || calls != 0 {
		t.Errorf("non-member: %d, lister calls %d", w.Code, calls)
	}

	r := writeRouterChannels(&fakeStore{}, ownerAccess, fakeChannelList(nil))
	if w := bearerRequest(r, "/guild/channels?guildID=general", "valid"); w.Code != 400 {
		t.Errorf("bad guild: %d", w.Code)
	}
	if w := bearerRequest(r, path, "bogus"); w.Code != 401 {
		t.Errorf("bad token: %d", w.Code)
	}
	if w := bearerRequest(writeRouterChannels(&fakeStore{}, ownerAccess, nil), path, "valid"); w.Code != http.StatusNotImplemented {
		t.Errorf("no bot token: %d", w.Code)
	}
	gone := channelListerFunc(func(context.Context, string) ([]GuildChannel, error) { return nil, errChannelNotFound })
	if w := bearerRequest(writeRouterChannels(&fakeStore{}, ownerAccess, gone), path, "valid"); w.Code != 404 {
		t.Errorf("bot not in guild: %d", w.Code)
	}
	down := channelListerFunc(func(context.Context, string) ([]GuildChannel, error) { return nil, errChannelUnavailable })
	if w := bearerRequest(writeRouterChannels(&fakeStore{}, ownerAccess, down), path, "valid"); w.Code != 503 {
		t.Errorf("outage: %d", w.Code)
	}
}
