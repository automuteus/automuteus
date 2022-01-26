package setting

import (
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/settings"
	"testing"
)

func TestFnDelays(t *testing.T) {
	err := testSettingsFn(FnDelays)
	if err != nil {
		t.Error(err)
	}

	sett := settings.MakeGuildSettings("", false)

	_, valid := FnDelays(sett, []string{"sett", "delays", "incomplete"})
	if valid {
		t.Error("Sending insufficient args should never result in valid settings change")
	}

	_, valid = FnDelays(sett, []string{"sett", "delays", "invalid", "lobby", "2"})
	if valid {
		t.Error("Sending invalid args should never result in valid settings change")
	}

	_, valid = FnDelays(sett, []string{"sett", "delays", "lobby", "invalid", "2"})
	if valid {
		t.Error("Sending invalid args should never result in valid settings change")
	}

	_, valid = FnDelays(sett, []string{"sett", "delays", "lobby", "tasks", "-2"})
	if valid {
		t.Error("Sending invalid args should never result in valid settings change")
	}

	_, valid = FnDelays(sett, []string{"sett", "delays", "lobby", "tasks", "8"})
	if !valid {
		t.Error("Sending valid args should result in valid settings change")
	}
	if sett.GetDelay(game.LOBBY, game.TASKS) != 8 {
		t.Error("Delay was not set properly")
	}
}
