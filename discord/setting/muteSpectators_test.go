package setting

import "testing"

func TestFnMuteSpectators(t *testing.T) {
	sett, err := testSettingsFn(FnMuteSpectators)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnMuteSpectators(sett, []string{"sett", "mutespec", "nottrueorfalse"})
	if valid {
		t.Error("Invalid mute spectators arg should never result in a valid settings change")
	}

	_, valid = FnMuteSpectators(sett, []string{"sett", "mutespec", "false"})
	if valid {
		t.Error("Identical mute spectator arg to default should never result in a valid settings change")
	}

	_, valid = FnMuteSpectators(sett, []string{"sett", "mutespec", "true"})
	if !valid {
		t.Error("Valid mute spectator arg should result in a valid settings change")
	}
	if !sett.GetMuteSpectator() {
		t.Error("Valid match summary (\"true\") was not set correctly")
	}

	_, valid = FnMuteSpectators(sett, []string{"sett", "mutespec", "false"})
	if !valid {
		t.Error("Valid mute spectator arg should result in a valid settings change")
	}
	if sett.GetMuteSpectator() {
		t.Error("Valid match summary (\"false\") was not set correctly")
	}
}
