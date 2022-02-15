package setting

import "testing"

func TestFnMatchSummary(t *testing.T) {
	sett, err := testSettingsFn(FnMatchSummary)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnMatchSummary(sett, []string{"sett", "matchsumm", "notanumber"})
	if valid {
		t.Error("Invalid match summary should never result in a valid settings change")
	}

	_, valid = FnMatchSummary(sett, []string{"sett", "matchsumm", "-1"})
	if !valid {
		t.Error("Valid match summary should result in a valid settings change")
	}
	if sett.GetDeleteGameSummaryMinutes() != -1 {
		t.Error("Valid match summary (\"-1\") was not set correctly")
	}

	_, valid = FnMatchSummary(sett, []string{"sett", "matchsumm", "0"})
	if !valid {
		t.Error("Valid match summary should result in a valid settings change")
	}
	if sett.GetDeleteGameSummaryMinutes() != 0 {
		t.Error("Valid match summary (\"0\") was not set correctly")
	}

	_, valid = FnMatchSummary(sett, []string{"sett", "matchsumm", "6"})
	if !valid {
		t.Error("Valid match summary should result in a valid settings change")
	}
	if sett.GetDeleteGameSummaryMinutes() != 6 {
		t.Error("Valid match summary (\"6\") was not set correctly")
	}
}

func TestFnMatchSummaryChannel(t *testing.T) {
	sett, err := testSettingsFn(FnMatchSummaryChannel)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnMatchSummaryChannel(sett, []string{"sett", "matchsumchan", "notanumber"})
	if valid {
		t.Error("Invalid match summary channel should never result in a valid settings change")
	}

	_, valid = FnMatchSummaryChannel(sett, []string{"sett", "matchsumchan", "12345"})
	if valid {
		t.Error("Invalid match summary channel should never result in a valid settings change")
	}

	_, valid = FnMatchSummaryChannel(sett, []string{"sett", "matchsumchan", "-754788173384777943"})
	if valid {
		t.Error("Invalid match summary channel should never result in a valid settings change")
	}

	_, valid = FnMatchSummaryChannel(sett, []string{"sett", "matchsumchan", "754788173384777943"})
	if !valid {
		t.Error("Valid match summary channel should result in a valid settings change")
	}
	if sett.GetMatchSummaryChannelID() != "754788173384777943" {
		t.Error("Valid match summary (\"754788173384777943\") was not set correctly")
	}
}
