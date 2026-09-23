package bot

import (
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

const (
	summaryGuild      = "100000000000000001"
	otherGuild        = "100000000000000002"
	gameChannel       = "200000000000000001"
	summaryText       = "200000000000000002"
	summaryVoice      = "200000000000000003"
	summaryThread     = "200000000000000004"
	summaryAnnounce   = "200000000000000005"
	summaryCategory   = "200000000000000006"
	otherGuildChannel = "300000000000000001"
)

func newSummaryBot(t *testing.T) (*Bot, *testDeps) {
	t.Helper()
	bot, deps := newTestBot(t)
	if err := deps.guilds.GuildAdd(&discordgo.Guild{ID: summaryGuild,
		Channels: []*discordgo.Channel{
			{ID: gameChannel, GuildID: summaryGuild, Type: discordgo.ChannelTypeGuildText},
			{ID: summaryText, GuildID: summaryGuild, Type: discordgo.ChannelTypeGuildText},
			{ID: summaryVoice, GuildID: summaryGuild, Type: discordgo.ChannelTypeGuildVoice},
			{ID: summaryAnnounce, GuildID: summaryGuild, Type: discordgo.ChannelTypeGuildNews},
			{ID: summaryCategory, GuildID: summaryGuild, Type: discordgo.ChannelTypeGuildCategory},
		},
		Threads: []*discordgo.Channel{
			{ID: summaryThread, GuildID: summaryGuild, Type: discordgo.ChannelTypeGuildPublicThread},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := deps.guilds.GuildAdd(&discordgo.Guild{ID: otherGuild,
		Channels: []*discordgo.Channel{{ID: otherGuildChannel, GuildID: otherGuild, Type: discordgo.ChannelTypeGuildText}},
	}); err != nil {
		t.Fatal(err)
	}
	return bot, deps
}

func TestSummaryChannel(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		want       string
		logged     string
	}{
		{"unset uses game channel", "", gameChannel, ""},
		{"text channel in guild", summaryText, summaryText, ""},
		{"announcement channel in guild", summaryAnnounce, summaryAnnounce, ""},
		{"thread in guild", summaryThread, summaryThread, ""},
		{"voice channel falls back", summaryVoice, gameChannel, "not a text channel"},
		{"category falls back", summaryCategory, gameChannel, "not a text channel"},
		{"channel of another guild falls back", otherGuildChannel, gameChannel, "not in this guild"},
		{"unknown channel falls back", "999999999999999999", gameChannel, "not in this guild"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bot, deps := newSummaryBot(t)
			sett := settings.MakeGuildSettings()
			sett.SetMatchSummaryChannelID(tc.configured)
			if got := bot.summaryChannel(summaryGuild, sett, gameChannel); got != tc.want {
				t.Errorf("summaryChannel = %s, want %s", got, tc.want)
			}
			if tc.logged != "" && !strings.Contains(deps.logs.String(), tc.logged) {
				t.Errorf("expected a log line containing %q, got:\n%s", tc.logged, deps.logs.String())
			}
			if tc.logged == "" && strings.Contains(deps.logs.String(), "match summary channel") {
				t.Errorf("unexpected warning for a valid channel:\n%s", deps.logs.String())
			}
		})
	}
}

// A guild the bot has no cached state for cannot vouch for the channel, so the game channel is used.
func TestSummaryChannel_UncachedGuildFallsBack(t *testing.T) {
	bot, _ := newTestBot(t)
	sett := settings.MakeGuildSettings()
	sett.SetMatchSummaryChannelID(summaryText)
	if got := bot.summaryChannel(summaryGuild, sett, gameChannel); got != gameChannel {
		t.Errorf("summaryChannel = %s, want fallback %s", got, gameChannel)
	}
}
