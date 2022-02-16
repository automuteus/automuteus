package setting

import "testing"

func TestFnMapVersion(t *testing.T) {
	sett, err := testSettingsFn(FnMapVersion)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnMapVersion(sett, []string{"sett", "map", "invalid"})
	if valid {
		t.Error("Invalid map version should never result in a valid settings change")
	}

	_, valid = FnMapVersion(sett, []string{"sett", "map", "simple"})
	if !valid {
		t.Error("Valid map version should result in a valid settings change")
	}
	if sett.GetMapVersion() != "simple" {
		t.Error("Valid map version (\"simple\") was not set correctly")
	}

	_, valid = FnMapVersion(sett, []string{"sett", "map", "detailed"})
	if !valid {
		t.Error("Valid map version should result in a valid settings change")
	}
	if sett.GetMapVersion() != "detailed" {
		t.Error("Valid map version (\"detailed\") was not set correctly")
	}
}
