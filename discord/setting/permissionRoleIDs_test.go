package setting

import "testing"

func TestFnPermissionRoleIDs(t *testing.T) {
	sett, err := testSettingsFn(FnPermissionRoleIDs)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnPermissionRoleIDs(sett, []string{"sett", "prids", "notpridorclear"})
	if valid {
		t.Error("Invalid prids should never result in a valid settings change")
	}

	_, valid = FnPermissionRoleIDs(sett, []string{"sett", "prids", "notpridorclear", "alsobad"})
	if valid {
		t.Error("Invalid prids should never result in a valid settings change")
	}

	_, valid = FnPermissionRoleIDs(sett, []string{"sett", "prids", "141100845902200999"})
	if !valid {
		t.Error("Valid prid arg should result in a valid settings change")
	}
	if len(sett.GetPermissionRoleIDs()) != 1 {
		if sett.GetPermissionRoleIDs()[0] != "141100845902200999" {
			t.Error("Valid prid arg didn't result in 1 prid set correctly")
		}
	}

	_, valid = FnPermissionRoleIDs(sett, []string{"sett", "prids", "notpridorclear", "nextisvalid", "141100845902200999"})
	if !valid {
		t.Error("Valid prid arg should result in a valid settings change")
	}
	if len(sett.GetPermissionRoleIDs()) != 1 {
		if sett.GetPermissionRoleIDs()[0] != "141100845902200999" {
			t.Error("Valid prid arg didn't result in 1 prid set correctly")
		}
	}

	_, valid = FnPermissionRoleIDs(sett, []string{"sett", "prids", "clear"})
	if !valid {
		t.Error("Valid prid clear should result in a valid settings change")
	}
	if len(sett.GetPermissionRoleIDs()) != 0 {
		t.Error("Valid prid clear didn't clear the prids correctly")
	}
}
