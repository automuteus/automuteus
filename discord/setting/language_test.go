package setting

import (
	"github.com/automuteus/utils/pkg/locale"
	"testing"
)

func TestFnLanguage(t *testing.T) {
	sett, err := testSettingsFn(FnLanguage)
	if err != nil {
		t.Error(err)
	}

	_, valid := FnLanguage(sett, []string{"invalid"})
	if valid {
		t.Error("Sending invalid args should never result in valid settings change")
	}

	_, valid = FnLanguage(sett, []string{"p"})
	if valid {
		t.Error("Invalid language format should never result in a valid language change")
	}

	_, valid = FnLanguage(sett, []string{"pe"})
	if valid {
		t.Error("Unimplemented language should never result in a valid language change")
	}

	_, valid = FnLanguage(sett, []string{"list"})
	if valid {
		t.Error("Listing languages should never result in a valid language change")
	}

	_, valid = FnLanguage(sett, []string{"reload"})
	if valid {
		t.Error("Reloading languages should never result in a valid language change")
	}

	locale.InitLang("testdata", "")
	_, valid = FnLanguage(sett, []string{"zu"})
	if !valid {
		t.Error("Valid language should result in a valid language change")
	}

	if sett.Language != "zu" {
		t.Error("Language was not changed successfully to zu")
	}

}
