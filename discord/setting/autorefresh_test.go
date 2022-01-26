package setting

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"testing"
)

func TestFnAutoRefresh(t *testing.T) {
	_, valid := FnAutoRefresh(nil, []string{})
	if valid {
		t.Error("Sending nil settings should never result in valid settings change")
	}

	sett := settings.MakeGuildSettings("", false)
	_, valid = FnAutoRefresh(sett, []string{})
	if valid {
		t.Error("Sending no args should never result in valid settings change")
	}

	_, valid = FnAutoRefresh(sett, []string{"sett"})
	if valid {
		t.Error("Sending no args should never result in valid settings change")
	}

	msg, valid := FnAutoRefresh(sett, []string{"sett", "refresh"})
	if valid {
		t.Error("Sending no args should never result in valid settings change")
	}
	// test the return type
	_ = msg.(discordgo.MessageEmbed)

	_, valid = FnAutoRefresh(sett, []string{"sett", "refresh", "nontrue"})
	if valid {
		t.Error("Sending invalid (non true/false) val should never result in valid settings change")
	}

	_, valid = FnAutoRefresh(sett, []string{"sett", "refresh", "false"})
	if valid {
		t.Error("Sending old val should never result in valid settings change")
	}

	_, valid = FnAutoRefresh(sett, []string{"sett", "refresh", "true"})
	if !valid {
		t.Error("Sending new true val should result in valid settings change")
	}
	if !sett.GetAutoRefresh() {
		t.Error("Autorefresh setting was not set true correctly")
	}

	_, valid = FnAutoRefresh(sett, []string{"sett", "refresh", "false"})
	if !valid {
		t.Error("Sending new false val should result in valid settings change")
	}
	if sett.GetAutoRefresh() {
		t.Error("Autorefresh setting was not set false correctly")
	}

}
