package setting

import (
	"testing"
)

func TestFnAdminUserIDs(t *testing.T) {
	sett, err := testSettingsFn(FnAdminUserIDs)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnAdminUserIDs(sett, []string{"sett", "admins", "somegarbage"})
	if valid {
		t.Error("Garbage admin IDs arg shouldn't result in valid settings change")
	}

	_, valid = FnAdminUserIDs(sett, []string{"sett", "admins", "<@!>"})
	if valid {
		t.Error("Bad mention format shouldn't result in valid settings change")
	}

	_, valid = FnAdminUserIDs(sett, []string{"sett", "admins", "888888066283941888"})
	if !valid {
		t.Error("Numeric input for admin ID should result in a valid settings change")
	}
	if len(sett.GetAdminUserIDs()) != 1 {
		t.Error("Expected 1 admin user id after setting")
	}

	_, valid = FnAdminUserIDs(sett, []string{"sett", "admins", "<@!888888066283941888>"})
	if valid {
		t.Error("Adding a pre-existing admin ID shouldn't result in a valid settings change")
	}
	if len(sett.GetAdminUserIDs()) != 1 {
		t.Error("Identical user ID shouldn't result in more than 1 admin ID")
	}

	_, valid = FnAdminUserIDs(sett, []string{"sett", "admins", "<@!140581888888888888>"})
	if !valid {
		t.Error("Adding a new admin ID should result in a valid settings change")
	}
	if len(sett.GetAdminUserIDs()) != 2 {
		t.Error("Different user ID should result in more than 1 admin ID")
	}

	_, valid = FnAdminUserIDs(sett, []string{"sett", "admins", "clear"})
	if !valid {
		t.Error("Clearing the admin IDs should always be a valid settings change")
	}
	if len(sett.GetAdminUserIDs()) != 0 {
		t.Error("Expected 0 admin user IDs after clearing")
	}
}
