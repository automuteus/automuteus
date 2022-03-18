package setting

import (
	"github.com/automuteus/utils/pkg/game"
	"testing"
)

func TestFnVoiceRules(t *testing.T) {
	sett, err := testSettingsFn(FnVoiceRules)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnVoiceRules(sett, []string{"notenough"})
	if valid {
		t.Error("Insufficient VR args should never result in a valid settings change")
	}

	_, valid = FnVoiceRules(sett, []string{"notenough", "notenoughstill"})
	if valid {
		t.Error("Insufficient VR args should never result in a valid settings change")
	}

	_, valid = FnVoiceRules(sett, []string{"invalid", "invalid2", "invalid2"})
	if valid {
		t.Error("Invalid VR args should never result in a valid settings change")
	}

	_, valid = FnVoiceRules(sett, []string{"deaf", "invalid2", "invalid2"})
	if valid {
		t.Error("Invalid VR args should never result in a valid settings change")
	}

	_, valid = FnVoiceRules(sett, []string{"deaf", "lobby", "invalid2"})
	if valid {
		t.Error("Invalid VR args should never result in a valid settings change")
	}

	_, valid = FnVoiceRules(sett, []string{"deaf", "lobby", "alive"})
	if valid {
		t.Error("Querying VR args should never result in a valid settings change")
	}

	_, valid = FnVoiceRules(sett, []string{"deaf", "lobby", "alive", "notbool"})
	if valid {
		t.Error("Invalid VR args should never result in a valid settings change")
	}

	sett.VoiceRules.DeafRules[game.PhaseNames[game.LOBBY]]["alive"] = false
	_, valid = FnVoiceRules(sett, []string{"deaf", "lobby", "alive", "false"})
	if valid {
		t.Error("Setting VR rules to the existing values should never result in a valid settings change")
	}

	_, valid = FnVoiceRules(sett, []string{"deaf", "lobby", "alive", "true"})
	if !valid {
		t.Error("Valid VR rules should result in a valid settings change")
	}
}
