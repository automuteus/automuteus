package setting

import "testing"

func TestFnLeaderboardMin(t *testing.T) {
	sett, err := testSettingsFn(FnLeaderboardMin)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnLeaderboardMin(sett, []string{"notanumber"})
	if valid {
		t.Error("Sending invalid args should never result in valid settings change")
	}

	_, valid = FnLeaderboardMin(sett, []string{"-1"})
	if valid {
		t.Error("Invalid leaderboard min should never result in a valid settings change")
	}

	_, valid = FnLeaderboardMin(sett, []string{"-1"})
	if valid {
		t.Error("Invalid leaderboard min should never result in a valid settings change")
	}

	_, valid = FnLeaderboardMin(sett, []string{"4.5"})
	if valid {
		t.Error("Invalid leaderboard min should never result in a valid settings change")
	}

	_, valid = FnLeaderboardMin(sett, []string{"4"})
	if !valid {
		t.Error("Valid leaderboard min should result in a valid settings change")
	}
	if sett.GetLeaderboardMin() != 4 {
		t.Error("Valid leaderboard min was not set correctly")
	}
}

func TestFnLeaderboardNameMention(t *testing.T) {
	sett, err := testSettingsFn(FnLeaderboardNameMention)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnLeaderboardNameMention(sett, []string{"false"})
	if !valid {
		t.Error("Valid leaderboard name mention should result in a valid settings change")
	}
	if sett.GetLeaderboardMention() {
		t.Error("Valid leaderboard name mention (\"false\") was not set correctly")
	}

	_, valid = FnLeaderboardNameMention(sett, []string{"true"})
	if !valid {
		t.Error("Valid leaderboard name mention should result in a valid settings change")
	}
	if !sett.GetLeaderboardMention() {
		t.Error("Valid leaderboard name mention (\"t\") was not set correctly")
	}
}

func TestFnLeaderboardSize(t *testing.T) {
	sett, err := testSettingsFn(FnLeaderboardSize)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnLeaderboardSize(sett, []string{"notanumber"})
	if valid {
		t.Error("Sending invalid args should never result in valid settings change")
	}

	_, valid = FnLeaderboardSize(sett, []string{"invalid"})
	if valid {
		t.Error("Invalid leaderboard size should never result in a valid settings change")
	}

	_, valid = FnLeaderboardSize(sett, []string{"-1"})
	if valid {
		t.Error("Invalid leaderboard size should never result in a valid settings change")
	}

	_, valid = FnLeaderboardSize(sett, []string{"2.5"})
	if valid {
		t.Error("Invalid leaderboard size should never result in a valid settings change")
	}

	_, valid = FnLeaderboardSize(sett, []string{"11"})
	if valid {
		t.Error("Invalid leaderboard size should never result in a valid settings change")
	}

	_, valid = FnLeaderboardSize(sett, []string{"2"})
	if !valid {
		t.Error("Valid leaderboard size should result in a valid settings change")
	}
	if sett.GetLeaderboardSize() != 2 {
		t.Error("Valid leaderboard size (2) was not set correctly")
	}
}
