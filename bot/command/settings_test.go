package command

import (
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

func TestSettingsURL(t *testing.T) {
	for _, tc := range []struct{ webURL, want string }{
		{"https://automute.us", "https://automute.us/settings?guild=123456789012345678"},
		{"https://automute.us/", "https://automute.us/settings?guild=123456789012345678"},
		{"http://localhost:3000", "http://localhost:3000/settings?guild=123456789012345678"},
	} {
		if got := SettingsURL(tc.webURL, "123456789012345678"); got != tc.want {
			t.Errorf("SettingsURL(%q) = %q, want %q", tc.webURL, got, tc.want)
		}
	}
}

func TestSettingsResponse(t *testing.T) {
	resp := SettingsResponse("https://automute.us/", "123456789012345678", settings.MakeGuildSettings())
	want := "https://automute.us/settings?guild=123456789012345678"

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

func TestSettingsCommandHasNoOptions(t *testing.T) {
	if len(Settings.Options) != 0 {
		t.Errorf("/settings should take no options now that settings live on the dashboard: %+v", Settings.Options)
	}
}
