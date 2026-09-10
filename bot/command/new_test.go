package command

import (
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

func TestNewResponseSuccess(t *testing.T) {
	info := NewInfo{
		Hyperlink:    "aucapture://localhost:8123/ABCDEFGH?insecure",
		ApiHyperlink: "http://localhost/open/link?connectCode=ABCDEFGH",
		MinimalURL:   "http://localhost:8123",
		ConnectCode:  "ABCDEFGH",
	}
	resp := NewResponse(NewSuccess, info, settings.MakeGuildSettings())

	if resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Error("success response should be ephemeral")
	}
	if len(resp.Data.Embeds) != 1 {
		t.Fatalf("expected exactly one embed, got %d", len(resp.Data.Embeds))
	}
	embed := resp.Data.Embeds[0]

	for _, want := range []string{info.Hyperlink, info.ApiHyperlink, CaptureDownloadURL, DotNetRuntimeDownloadURL} {
		if !strings.Contains(embed.Description, want) {
			t.Errorf("embed description is missing %q", want)
		}
	}
	if strings.Contains(embed.Description, "{{") {
		t.Errorf("embed description has an unexpanded template: %q", embed.Description)
	}

	fieldValues := map[string]string{}
	for _, f := range embed.Fields {
		fieldValues[f.Name] = f.Value
	}
	if fieldValues["URL"] != info.MinimalURL {
		t.Errorf("URL field = %q, want %q", fieldValues["URL"], info.MinimalURL)
	}
	if fieldValues["Code"] != info.ConnectCode {
		t.Errorf("Code field = %q, want %q", fieldValues["Code"], info.ConnectCode)
	}
	if embed.Footer == nil || embed.Footer.Text == "" {
		t.Error("expected a footer with troubleshooting text")
	}
}

func TestNewResponseNoVoiceChannel(t *testing.T) {
	resp := NewResponse(NewNoVoiceChannel, NewInfo{}, settings.MakeGuildSettings())
	if resp.Data.Content == "" || len(resp.Data.Embeds) != 0 {
		t.Errorf("no-channel response should be text only, got content=%q embeds=%d", resp.Data.Content, len(resp.Data.Embeds))
	}
}
