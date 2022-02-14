package setting

import (
	"testing"
)

func TestFnAutoRefresh(t *testing.T) {
	sett, err := testSettingsFn(FnAutoRefresh)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnAutoRefresh(sett, []string{"sett", "refresh", "nontrue"})
	if valid {
		t.Error("Sending invalid (non true/false) val should never result in valid settings change")
	}

	_, valid = FnAutoRefresh(sett, []string{"sett", "refresh", "false"})
	if valid {
		t.Error("Sending old val should never result in valid settings change")
	}

	_, valid = FnAutoRefresh(sett, []string{"sett", "refresh", "true"})
	if !valid {
		t.Error("Sending new true val should result in valid settings change")
	}
	if !sett.GetAutoRefresh() {
		t.Error("Autorefresh setting was not set true correctly")
	}

	_, valid = FnAutoRefresh(sett, []string{"sett", "refresh", "false"})
	if !valid {
		t.Error("Sending new false val should result in valid settings change")
	}
	if sett.GetAutoRefresh() {
		t.Error("Autorefresh setting was not set false correctly")
	}

}
