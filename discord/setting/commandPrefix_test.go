package setting

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"testing"
)

func TestFnCommandPrefix(t *testing.T) {
	_, valid := FnCommandPrefix(nil, []string{})
	if valid {
		t.Error("Sending nil settings should never result in valid settings change")
	}

	sett := settings.MakeGuildSettings("", false)
	_, valid = FnCommandPrefix(sett, []string{})
	if valid {
		t.Error("Sending no args should never result in valid settings change")
	}

	_, valid = FnCommandPrefix(sett, []string{"sett"})
	if valid {
		t.Error("Sending no args should never result in valid settings change")
	}

	msg, valid := FnCommandPrefix(sett, []string{"sett", "prefix"})
	if valid {
		t.Error("Sending no args should never result in valid settings change")
	}
	// test the return type
	_ = msg.(*discordgo.MessageEmbed)

	_, valid = FnCommandPrefix(sett, []string{"sett", "somereallylongprefix"})
	if valid {
		t.Error("Long prefixes are invalid and shouldn't result in valid settings change")
	}

	_, valid = FnCommandPrefix(sett, []string{"sett", "prefix", ".ok"})
	if !valid {
		t.Error(".ok is a valid prefix and should result in valid settings change")
	}
	if sett.GetCommandPrefix() != ".ok" {
		t.Error("Expected prefix to be .ok")
	}

	_, valid = FnCommandPrefix(sett, []string{"sett", "prefix", "@AutoMuteUs"})
	if !valid {
		t.Error("@AutoMuteUs is a valid prefix and should result in valid settings change")
	}

	_, valid = FnCommandPrefix(sett, []string{"sett", "prefix", "<@!" + settings.OfficialBotMention + ">"})
	if !valid {
		t.Error("<@!AutoMuteUs> is a valid prefix and should result in valid settings change")
	}
	if sett.GetCommandPrefix() != "@AutoMuteUs" {
		t.Error("Expected @AutoMuteUs prefix when set using mention format <@!" + settings.OfficialBotMention + ">")
	}
}
