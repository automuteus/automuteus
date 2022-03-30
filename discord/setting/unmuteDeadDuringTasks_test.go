package setting

import "testing"

func TestFnUnmuteDeadDuringTasks(t *testing.T) {
	sett, err := testSettingsFn(FnUnmuteDeadDuringTasks)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnUnmuteDeadDuringTasks(sett, []string{"nottrueorfalse"})
	if valid {
		t.Error("Invalid unmutedead should never result in a valid settings change")
	}

	sett.UnmuteDeadDuringTasks = false
	_, valid = FnUnmuteDeadDuringTasks(sett, []string{"false"})
	if valid {
		t.Error("Valid unmutedead that is already set should never result in a valid settings change")
	}

	_, valid = FnUnmuteDeadDuringTasks(sett, []string{"true"})
	if !valid {
		t.Error("Valid unmutedead should result in a valid settings change")
	}
	if !sett.GetUnmuteDeadDuringTasks() {
		t.Error("Valid unmutedead (\"true\") didn't result in a valid settings change")
	}

	_, valid = FnUnmuteDeadDuringTasks(sett, []string{"false"})
	if !valid {
		t.Error("Valid unmutedead should result in a valid settings change")
	}
	if sett.GetUnmuteDeadDuringTasks() {
		t.Error("Valid unmutedead (\"false\") didn't result in a valid settings change")
	}
}
