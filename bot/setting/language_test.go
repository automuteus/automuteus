package setting

import (
	"os"
	"strings"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/locale"
	"github.com/bwmarrin/discordgo"
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

// Switching to a non-English language should point users at Crowdin; English should not.
func TestFnLanguage_CrowdinHint(t *testing.T) {
	sett, err := testSettingsFn(FnLanguage)
	if err != nil {
		t.Fatal(err)
	}
	locale.InitLang("")

	msg, valid := FnLanguage(sett, []string{"en"})
	if !valid {
		t.Fatal("en should be a valid language")
	}
	if s, _ := msg.(string); strings.Contains(s, locale.CrowdinURL) {
		t.Errorf("English confirmation should not carry the Crowdin hint: %q", s)
	}
	embed, _ := FnLanguage(sett, nil)
	if e, ok := embed.(discordgo.MessageEmbed); !ok || strings.Contains(e.Description, locale.CrowdinURL) {
		t.Errorf("English language embed should not carry the Crowdin hint: %+v", embed)
	}

	msg, valid = FnLanguage(sett, []string{"ja"})
	if !valid {
		t.Fatal("ja should be a valid language")
	}
	if s, _ := msg.(string); !strings.Contains(s, locale.CrowdinURL) {
		t.Errorf("Japanese confirmation should carry the Crowdin hint: %q", s)
	}
	embed, _ = FnLanguage(sett, nil)
	if e, ok := embed.(discordgo.MessageEmbed); !ok || !strings.Contains(e.Description, locale.CrowdinURL) {
		t.Errorf("Japanese language embed should carry the Crowdin hint: %+v", embed)
	}
}
