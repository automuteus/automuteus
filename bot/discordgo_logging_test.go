package bot

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestShouldDropDiscordgoMessage(t *testing.T) {
	unknown := "unknown event: Op: %d, Seq: %d, Type: %s, Data: %s"
	cases := []struct {
		name   string
		level  int
		format string
		drop   bool
	}{
		{"unknown event warning", discordgo.LogWarning, unknown, true},
		{"other warning", discordgo.LogWarning, "error unmarshalling %s event, %s", false},
		{"error level", discordgo.LogError, "error closing websocket, %s", false},
		{"bookkeeping: called", discordgo.LogInformational, "called", true},
		{"bookkeeping: exiting", discordgo.LogInformational, "exiting", true},
		{"bookkeeping: hello", discordgo.LogInformational, "Op 10 Hello Packet received from Discord", true},
		{"bookkeeping: disconnect", discordgo.LogInformational, "emit disconnect event", true},
		{"exact match only", discordgo.LogInformational, "called %s", false},
		// the reconnect lifecycle and REST rate limits are why informational is enabled at all
		{"connecting", discordgo.LogInformational, "connecting to gateway %s", false},
		{"resume", discordgo.LogInformational, "sending resume packet to gateway", false},
		{"reconnecting", discordgo.LogInformational, "trying to reconnect to gateway", false},
		{"reconnected", discordgo.LogInformational, "successfully reconnected to gateway", false},
		{"op7", discordgo.LogInformational, "Closing and reconnecting in response to Op7", false},
		{"op9", discordgo.LogInformational, "sending identify packet to gateway in response to Op9", false},
		{"rate limit", discordgo.LogInformational, "Rate Limiting %s, retry in %v", false},
		{"502 retry", discordgo.LogInformational, "%s Failed (%s), Retrying...", false},
		{"unknown event text at error level", discordgo.LogError, unknown, false},
		{"resume event dump", discordgo.LogWarning, "Expected READY/RESUMED, instead got:\n%#v\n", true},
		{"first packet dump", discordgo.LogInformational, "First Packet:\n%#v\n", true},
	}
	for _, c := range cases {
		if got := shouldDropDiscordgoMessage(c.level, c.format); got != c.drop {
			t.Errorf("%s: drop = %v, want %v", c.name, got, c.drop)
		}
	}
}

func TestDiscordgoLoggerOutput(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	// discordgo calls Logger through its msglog helper, so callers are offset by one frame relative to a
	// direct call. A direct call with caller=0 therefore resolves to this test function.
	discordgoLogger(discordgo.LogWarning, 0, "unknown event: Op: %d, Type: %s", 0, "VOICE_CHANNEL_START_TIME_UPDATE")
	if buf.Len() != 0 {
		t.Fatalf("unknown event warning should be dropped, got %q", buf.String())
	}

	discordgoLogger(discordgo.LogWarning, 0, "something %s", "bad")
	out := buf.String()
	for _, want := range []string{"level=WARN", `msg="something bad"`, "component=discordgo", "discordgo_logging_test.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestRateLimitEventCallback_LogsWaitBucketAndURL(t *testing.T) {
	bot, deps := newTestBot(t)

	bot.rateLimitEventCallback(nil, &discordgo.RateLimit{
		TooManyRequests: &discordgo.TooManyRequests{
			Bucket:     "abc123",
			Message:    "You are being rate limited.",
			RetryAfter: 1500 * time.Millisecond,
		},
		URL: "https://discord.com/api/v9/guilds/1/members/2",
	})

	out := deps.logs.String()
	for _, want := range []string{"level=WARN", "rate limited by Discord", "component=discordgo", "bucket=abc123",
		"retry_after=1.5s", "url=https://discord.com/api/v9/guilds/1/members/2"} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q: %q", want, out)
		}
	}

	// a rate limit whose body could not be parsed still logs the URL
	deps.logs.Reset()
	bot.rateLimitEventCallback(nil, &discordgo.RateLimit{URL: "https://discord.com/api/v9/x"})
	if out := deps.logs.String(); !strings.Contains(out, "rate limited by Discord") || !strings.Contains(out, "url=https://discord.com/api/v9/x") {
		t.Errorf("unparsed rate limit not logged: %q", out)
	}
}
