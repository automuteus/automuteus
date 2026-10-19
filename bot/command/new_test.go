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

	// Mobile users copy the link with long-press > Copy Text, which copies the raw message content, so the content
	// must be exactly the link with nothing around it
	if resp.Data.Content != info.Hyperlink {
		t.Errorf("content = %q, want exactly the capture link %q", resp.Data.Content, info.Hyperlink)
	}

	for _, want := range []string{info.ApiHyperlink, CaptureDownloadURL} {
		if !strings.Contains(embed.Description, want) {
			t.Errorf("embed description is missing %q", want)
		}
	}
	// Capture Mod users are pointed at the aucapture:// link, which the click-here link would hide behind a redirect
	if !strings.Contains(embed.Description, "Capture Mod") {
		t.Error("embed description doesn't tell Capture Mod users what to do")
	}

	fieldValues := map[string]string{}
	for _, f := range embed.Fields {
		fieldValues[f.Name] = f.Value
	}
	// Inline code, so the server URL isn't a clickable link that people open in a browser
	if got := fieldValues["Server URL (manual connect only)"]; got != "`"+info.MinimalURL+"`" {
		t.Errorf("server URL field = %q, want %q in inline code", got, info.MinimalURL)
	}
	if got := fieldValues["Code"]; got != "`"+info.ConnectCode+"`" {
		t.Errorf("Code field = %q, want %q in inline code", got, info.ConnectCode)
	}
	for _, text := range []string{resp.Data.Content, embed.Description} {
		if strings.Contains(text, "{{") {
			t.Errorf("response has an unexpanded template: %q", text)
		}
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
