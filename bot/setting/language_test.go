package setting

import (
	"os"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/locale"
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

	locale.InitLangFS(os.DirFS("testdata"), "")
	_, valid = FnLanguage(sett, []string{"zu"})
	if !valid {
		t.Error("Valid language should result in a valid language change")
	}

	if sett.Language != "zu" {
		t.Error("Language was not changed successfully to zu")
	}

}
