package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/storage"
	"testing"
)

const (
	TestChannelID = "SomeChannelID"
	HelpTitle     = "AutoMuteUs Bot Commands:\n"
)

func TestHelpCommand(t *testing.T) {
	args := []string{"help"}
	originMessage := discordgo.MessageCreate{&discordgo.Message{
		ChannelID: TestChannelID,
	}}
	sett := storage.MakeGuildSettings()

	channelID, message := commandFnHelp(nil, false, false, sett, nil, &originMessage, args, nil)
	assertHelpMessageProperties(message, channelID, t)

	channelID, message = commandFnHelp(nil, false, true, sett, nil, &originMessage, args, nil)
	assertHelpMessageProperties(message, channelID, t)

	channelID, message = commandFnHelp(nil, true, true, sett, nil, &originMessage, args, nil)
	assertHelpMessageProperties(message, channelID, t)
}

func assertHelpMessageProperties(m interface{}, channelID string, t *testing.T) {
	if channelID != TestChannelID {
		t.Errorf("Expected help channelID to be \"%s\", but got \"%s\"", TestChannelID, channelID)
	}
	switch m.(type) {
	case *discordgo.MessageEmbed:
	default:
		t.Errorf("Expected *discordgo.MessageEmbed from .au help, but got: %T", m)
	}
	embed := m.(*discordgo.MessageEmbed)
	if embed.Title != HelpTitle {
		t.Errorf("Title of \"%s\" doesn't match the expected \"%s\"", embed.Title, HelpTitle)
	}
}
