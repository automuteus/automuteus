package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

func TestCheckSummaryChannel(t *testing.T) {
	post := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)
	for _, tc := range []struct {
		name     string
		info     ChannelInfo
		err      error
		wantOK   bool
		wantName string
		problem  string
	}{
		{"text channel with permissions", ChannelInfo{ID: "1", GuildID: writeGuild, Type: discordgo.ChannelTypeGuildText, Name: "summaries", Permissions: post}, nil, true, "summaries", ""},
		{"thread with thread permission", ChannelInfo{ID: "1", GuildID: writeGuild, Type: discordgo.ChannelTypeGuildPublicThread, Name: "t", Permissions: post | discordgo.PermissionSendMessagesInThreads}, nil, true, "t", ""},
		{"thread without thread permission", ChannelInfo{ID: "1", GuildID: writeGuild, Type: discordgo.ChannelTypeGuildPublicThread, Name: "t", Permissions: post}, nil, false, "t", "Send Messages in Threads permission in"},
		{"thread needs only the thread flavour of send", ChannelInfo{ID: "1", GuildID: writeGuild, Type: discordgo.ChannelTypeGuildNewsThread, Name: "t", Permissions: discordgo.PermissionViewChannel | discordgo.PermissionEmbedLinks | discordgo.PermissionSendMessagesInThreads}, nil, true, "t", ""},
		{"administrator can post in threads", ChannelInfo{ID: "1", GuildID: writeGuild, Type: discordgo.ChannelTypeGuildPublicThread, Name: "t", Permissions: everyPermission}, nil, true, "t", ""},
		{"thread missing everything names the thread permission", ChannelInfo{ID: "1", GuildID: writeGuild, Type: discordgo.ChannelTypeGuildPublicThread, Name: "t", Permissions: 0}, nil, false, "t", "View Channel, Send Messages in Threads, Embed Links permissions"},
		{"missing one permission", ChannelInfo{ID: "1", GuildID: writeGuild, Type: discordgo.ChannelTypeGuildText, Name: "s", Permissions: post &^ discordgo.PermissionEmbedLinks}, nil, false, "s", "missing the Embed Links permission in"},
		{"missing several permissions", ChannelInfo{ID: "1", GuildID: writeGuild, Type: discordgo.ChannelTypeGuildText, Name: "s", Permissions: 0}, nil, false, "s", "View Channel, Send Messages, Embed Links permissions"},
		{"voice channel", ChannelInfo{ID: "1", GuildID: writeGuild, Type: discordgo.ChannelTypeGuildVoice, Name: "v", Permissions: post}, nil, false, "v", problemType},
		{"other guild hides the name", ChannelInfo{ID: "1", GuildID: "923456789012345678", Type: discordgo.ChannelTypeGuildText, Name: "secret", Permissions: post}, nil, false, "", problemGuild},
		{"not found", ChannelInfo{}, errChannelNotFound, false, "", problemNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check, err := checkSummaryChannel(context.Background(), channelVerifierFunc(func(context.Context, string) (ChannelInfo, error) { return tc.info, tc.err }), writeGuild, "223456789012345678")
			if err != nil {
				t.Fatal(err)
			}
			if check.OK != tc.wantOK || check.Name != tc.wantName || check.ID != "223456789012345678" {
				t.Fatalf("check = %+v", check)
			}
			if tc.wantOK && len(check.Problems) != 0 {
				t.Fatalf("ok check has problems: %v", check.Problems)
			}
			if !tc.wantOK && (len(check.Problems) != 1 || !strings.Contains(check.Problems[0], tc.problem)) {
				t.Fatalf("problems = %v, want one containing %q", check.Problems, tc.problem)
			}
		})
	}
	if _, err := checkSummaryChannel(context.Background(), channelVerifierFunc(func(context.Context, string) (ChannelInfo, error) { return ChannelInfo{}, errChannelUnavailable }), writeGuild, "223456789012345678"); err == nil {
		t.Fatal("lookup failure must be an error, not a problem")
	}
}

func TestGuildChannelRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	path := "/guild/channel?guildID=" + writeGuild + "&channelID="
	r, _ := writeRouterWith(&fakeStore{}, memberAccess, fakeChannels(nil))

	w := bearerRequest(r, path+textChannelHere, "valid")
	var check ChannelCheck
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &check) != nil || !check.OK || check.Name != "match-summaries" || len(check.Problems) != 0 {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("missing cache protection")
	}
	for channel, want := range map[string]string{
		textNoPostHere:   "missing the Send Messages, Embed Links permissions",
		voiceChannelHere: problemType,
		textChannelThere: problemGuild,
		unknownChannel:   problemNotFound,
	} {
		w := bearerRequest(r, path+channel, "valid")
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &check) != nil || check.OK || len(check.Problems) != 1 || !strings.Contains(check.Problems[0], want) {
			t.Errorf("%s: got %d %s", channel, w.Code, w.Body)
		}
	}
	if w := bearerRequest(r, path+textChannelThere, "valid"); strings.Contains(w.Body.String(), `"name"`) {
		t.Errorf("channel in another guild leaked its name: %s", w.Body)
	}

	// Validation and authentication happen before any lookup.
	for _, bad := range []string{"/guild/channel?guildID=" + writeGuild, path + "general", "/guild/channel?channelID=" + textChannelHere} {
		if w := bearerRequest(r, bad, "valid"); w.Code != 400 {
			t.Errorf("%s: %d", bad, w.Code)
		}
	}
	if w := bearerRequest(r, path+textChannelHere, "bogus"); w.Code != 401 {
		t.Errorf("bad token: %d", w.Code)
	}
	departed := VerifiedGuildAccess{UserID: "999", GuildID: writeGuild}
	if r, _ := writeRouterWith(&fakeStore{}, departed, fakeChannels(nil)); bearerRequest(r, path+textChannelHere, "valid").Code != 403 {
		t.Error("non-member could check a channel")
	}

	// Without a bot token the answer is 501, and a Discord outage is 503.
	if r, _ := writeRouter(&fakeStore{}, memberAccess); bearerRequest(r, path+textChannelHere, "valid").Code != http.StatusNotImplemented {
		t.Error("missing verifier should be 501")
	}
	down := channelVerifierFunc(func(context.Context, string) (ChannelInfo, error) { return ChannelInfo{}, errChannelUnavailable })
	if r, _ := writeRouterWith(&fakeStore{}, memberAccess, down); bearerRequest(r, path+textChannelHere, "valid").Code != 503 {
		t.Error("lookup outage should be 503")
	}
}
