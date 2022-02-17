package discord

import (
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"strings"
	"testing"
)

const (
	TestChannelID = "SomeChannelID"
	HelpTitle     = "AutoMuteUs Bot Commands:\n"
)

func TestHelpCommand(t *testing.T) {
	args := []string{"help"}
	originMessage := discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: TestChannelID,
	}}
	sett := settings.MakeGuildSettings("", false)

	channelID, message := commandFnHelp(nil, false, false, sett, nil, &originMessage, args, nil)
	assertHelpMessageProperties(message, channelID, t)

	channelID, message = commandFnHelp(nil, false, true, sett, nil, &originMessage, args, nil)
	assertHelpMessageProperties(message, channelID, t)
	// probably should test the presence of permissioned fields

	channelID, message = commandFnHelp(nil, true, true, sett, nil, &originMessage, args, nil)
	assertHelpMessageProperties(message, channelID, t)
	// probably should test the presence of permissioned fields

	args = []string{"help", "nonexistentcommand"}
	channelID, message = commandFnHelp(nil, false, false, sett, nil, &originMessage, args, nil)
	strMsg := message.(string)
	if strMsg != "I didn't recognize that command! View `help` for all available commands!" {
		t.Error("Unexpected response from help when supplied a nonexistent command: " + strMsg)
	}
	args = []string{"help", "ascii"}
	channelID, message = commandFnHelp(nil, false, false, sett, nil, &originMessage, args, nil)
	embed := message.(*discordgo.MessageEmbed)
	if !strings.Contains(embed.Title, "Ascii") {
		t.Error(embed.Title + " doesn't contain Ascii as expected")
	}
}

func TestHelpPrefix(t *testing.T) {
	args := []string{"help"}
	originMessage := discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: TestChannelID,
	}}

	sett := settings.MakeGuildSettings("", false)
	channelID, message := commandFnHelp(nil, false, false, sett, nil, &originMessage, args, nil)
	assertHelpMessageProperties(message, channelID, t)
}

func assertHelpMessageProperties(m interface{}, channelID string, t *testing.T) *discordgo.MessageEmbed {
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
	return embed
}
