package command

import (
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

func TestStatsURL(t *testing.T) {
	for _, tc := range []struct{ webURL, want string }{
		{"https://automute.us", "https://automute.us/stats?guild=123456789012345678"},
		{"https://automute.us/", "https://automute.us/stats?guild=123456789012345678"},
		{"http://localhost:3000", "http://localhost:3000/stats?guild=123456789012345678"},
	} {
		if got := StatsURL(tc.webURL, "123456789012345678"); got != tc.want {
			t.Errorf("StatsURL(%q) = %q, want %q", tc.webURL, got, tc.want)
		}
	}
}

func TestMatchURL(t *testing.T) {
	if got, want := MatchURL("https://automute.us/", "123456789012345678", 42), "https://automute.us/stats/match?guild=123456789012345678&match=42"; got != want {
		t.Errorf("MatchURL = %q, want %q", got, want)
	}
	if got := MatchURL("https://automute.us", "123456789012345678", -1); got != "" {
		t.Errorf("a match that was never recorded has no page: %q", got)
	}
}

func TestStatsResponse(t *testing.T) {
	resp := StatsResponse("https://automute.us/", "123456789012345678", settings.MakeGuildSettings())
	want := "https://automute.us/stats?guild=123456789012345678"

	if resp.Type != discordgo.InteractionResponseChannelMessageWithSource {
		t.Errorf("type = %v", resp.Type)
	}
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Error("the link should be a private reply")
	}
	if !strings.Contains(resp.Data.Content, "<"+want+">") {
		t.Errorf("content should carry the link without a preview: %q", resp.Data.Content)
	}
	if len(resp.Data.Components) != 1 {
		t.Fatalf("components = %+v", resp.Data.Components)
	}
	row, ok := resp.Data.Components[0].(discordgo.ActionsRow)
	if !ok || len(row.Components) != 1 {
		t.Fatalf("expected one action row with one button, got %+v", resp.Data.Components[0])
	}
	button, ok := row.Components[0].(discordgo.Button)
	if !ok {
		t.Fatalf("expected a button, got %T", row.Components[0])
	}
	if button.Style != discordgo.LinkButton || button.URL != want || button.Label == "" {
		t.Errorf("button = %+v", button)
	}
}

func TestStatsCommandHasNoOptions(t *testing.T) {
	if len(Stats.Options) != 0 {
		t.Errorf("/stats should take no options now that stats live on the dashboard: %+v", Stats.Options)
	}
}

func TestStatsResponseButtonHasAnEmoji(t *testing.T) {
	resp := StatsResponse("https://automute.us/", "123456789012345678", settings.MakeGuildSettings())
	row, ok := resp.Data.Components[0].(discordgo.ActionsRow)
	if !ok || len(row.Components) != 1 {
		t.Fatalf("expected one action row with one button, got %+v", resp.Data.Components)
	}
	button, ok := row.Components[0].(discordgo.Button)
	if !ok {
		t.Fatalf("expected a button, got %T", row.Components[0])
	}
	// discordgo 0.27 always serializes the emoji; an empty name is rejected by Discord (COMPONENT_INVALID_EMOJI)
	if button.Style != discordgo.LinkButton || button.Emoji.Name == "" {
		t.Errorf("button = %+v", button)
	}
}
