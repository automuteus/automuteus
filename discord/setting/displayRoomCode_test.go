package setting

import "testing"

func TestFnDisplayRoomCode(t *testing.T) {
	sett, err := testSettingsFn(FnDisplayRoomCode)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnDisplayRoomCode(sett, []string{"sett", "roomcode", "invalid"})
	if valid {
		t.Error("Sending invalid args should never result in valid settings change")
	}

	_, valid = FnDisplayRoomCode(sett, []string{"sett", "roomcode", "always"})
	if !valid {
		t.Error("Sending a valid arg for roomcode should result in settings change")
	}
	if sett.GetDisplayRoomCode() != "always" {
		t.Error("DisplayRoomCode should be set to always after successful change")
	}
}
