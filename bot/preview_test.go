package bot

import (
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/settings"
)

func TestPreviewEmbeds_AreValidAndCarryNotice(t *testing.T) {
	sett := settings.MakeGuildSettings()
	plain := PreviewEmbeds(sett, nil)
	if len(plain) < 6 {
		t.Fatalf("only %d previews", len(plain))
	}
	seen := map[string]bool{}
	for _, p := range plain {
		if p.Name == "" || p.Embed == nil || seen[p.Name] {
			t.Errorf("bad preview %q", p.Name)
		}
		seen[p.Name] = true
		// the bot refuses to edit a status message into an embed with empty fields; previews must pass the same check
		if !ValidFields(p.Embed) {
			t.Errorf("%s: embed has an empty field; the bot would refuse to send it", p.Name)
		}
	}

	warned := PreviewEmbeds(sett, &notice.Notice{Severity: notice.Warning, Message: "degraded"})
	for _, p := range warned {
		if len(p.Embed.Fields) == 0 || !strings.Contains(p.Embed.Fields[0].Name, "WARNING") {
			t.Errorf("%s: notice banner missing", p.Name)
		}
	}
}
