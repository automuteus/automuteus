package bot

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

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
		{"informational", discordgo.LogInformational, "called", false},
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
