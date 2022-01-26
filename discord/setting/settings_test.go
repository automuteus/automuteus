package setting

import (
	"errors"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

func testSettingsFn(fn func(settings *settings.GuildSettings, args []string) (interface{}, bool)) error {
	_, valid := fn(nil, []string{})
	if valid {
		return errors.New("sending nil settings should never result in valid settings change")
	}

	sett := settings.MakeGuildSettings("", false)
	_, valid = fn(sett, []string{})
	if valid {
		return errors.New("sending no args should never result in valid settings change")
	}

	_, valid = fn(sett, []string{"sett"})
	if valid {
		return errors.New("sending no args should never result in valid settings change")
	}

	msg, valid := fn(sett, []string{"sett", "delays"})
	if valid {
		return errors.New("sending no args should never result in valid settings change")
	}
	// test the return type
	switch msg.(type) {
	case discordgo.MessageEmbed:
		return nil
	case *discordgo.MessageEmbed:
		return nil
	default:
		return errors.New("returned settings message does not match expected embed or *embed type")
	}
}
