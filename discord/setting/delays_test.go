package setting

import (
	"github.com/automuteus/utils/pkg/game"
	"testing"
)

func TestFnDelays(t *testing.T) {
	sett, err := testSettingsFn(FnDelays)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnDelays(sett, []string{"invalid", "lobby", "2"})
	if valid {
		t.Error("Sending invalid args should never result in valid settings change")
	}

	_, valid = FnDelays(sett, []string{"lobby", "invalid", "2"})
	if valid {
		t.Error("Sending invalid args should never result in valid settings change")
	}

	_, valid = FnDelays(sett, []string{"lobby", "tasks", "-2"})
	if valid {
		t.Error("Sending invalid args should never result in valid settings change")
	}

	_, valid = FnDelays(sett, []string{"lobby", "tasks", "8"})
	if !valid {
		t.Error("Sending valid args should result in valid settings change")
	}
	if sett.GetDelay(game.LOBBY, game.TASKS) != 8 {
		t.Error("Delay was not set properly")
	}
}
