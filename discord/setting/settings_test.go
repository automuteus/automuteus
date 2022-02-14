package setting

import (
	"errors"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

func testSettingsFn(fn func(settings *settings.GuildSettings, args []string) (interface{}, bool)) (*settings.GuildSettings, error) {
	_, valid := fn(nil, []string{})
	if valid {
		return nil, errors.New("sending nil settings should never result in valid settings change")
	}

	sett := settings.MakeGuildSettings("", false)
	_, valid = fn(sett, []string{})
	if valid {
		return nil, errors.New("sending no args should never result in valid settings change")
	}

	_, valid = fn(sett, []string{"sett"})
	if valid {
		return nil, errors.New("sending no args should never result in valid settings change")
	}

	msg, valid := fn(sett, []string{"sett", "<somesetting>"})
	if valid {
		return nil, errors.New("sending no args should never result in valid settings change")
	}
	// test the return type
	switch msg.(type) {
	case discordgo.MessageEmbed:
		return sett, nil
	case *discordgo.MessageEmbed:
		return sett, nil
	default:
		return nil, errors.New("returned settings message does not match expected embed or *embed type")
	}
}
