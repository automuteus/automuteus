package setting

import (
	"github.com/automuteus/utils/pkg/settings"
	"testing"
)

func TestFnMapVersion(t *testing.T) {
	sett := settings.MakeGuildSettings()

	_, valid := FnMapVersion(sett, []string{"true"})
	if !valid {
		t.Error("Valid map version should result in a valid settings change")
	}
	if !sett.GetMapDetailed() {
		t.Error("Valid map version (\"true\") was not set correctly")
	}

	_, valid = FnMapVersion(sett, []string{"false"})
	if !valid {
		t.Error("Valid map version should result in a valid settings change")
	}
	if sett.GetMapDetailed() {
		t.Error("Valid map version (\"false\") was not set correctly")
	}
}
