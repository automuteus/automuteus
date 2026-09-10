package bot

import (
	"bytes"
	"log"
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
	}
	for _, c := range cases {
		if got := shouldDropDiscordgoMessage(c.level, c.format); got != c.drop {
			t.Errorf("%s: drop = %v, want %v", c.name, got, c.drop)
		}
	}
}

func TestDiscordgoLoggerOutput(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	// discordgo calls Logger through its msglog helper, so callers are offset by one frame relative to a
	// direct call. A direct call with caller=0 therefore resolves to this test function.
	discordgoLogger(discordgo.LogWarning, 0, "unknown event: Op: %d, Type: %s", 0, "VOICE_CHANNEL_START_TIME_UPDATE")
	if buf.Len() != 0 {
		t.Fatalf("unknown event warning should be dropped, got %q", buf.String())
	}

	discordgoLogger(discordgo.LogWarning, 0, "something %s", "bad")
	out := buf.String()
	if !strings.HasPrefix(out[strings.Index(out, "["):], "[DG1] ") {
		t.Errorf("missing discordgo level prefix: %q", out)
	}
	if !strings.Contains(out, "something bad") {
		t.Errorf("message not passed through: %q", out)
	}
	if !strings.Contains(out, "discordgo_logging_test.go") {
		t.Errorf("caller should resolve to the message source, got %q", out)
	}
}
