package discord

import "github.com/bwmarrin/discordgo"

// IsTextCapableChannel reports whether the bot can post a message (such as a match summary) into a channel of this
// type: guild text and announcement channels and their threads. Voice, stage, category, forum, and DM channels are
// not destinations for guild messages.
func IsTextCapableChannel(t discordgo.ChannelType) bool {
	switch t {
	case discordgo.ChannelTypeGuildText, discordgo.ChannelTypeGuildNews,
		discordgo.ChannelTypeGuildNewsThread, discordgo.ChannelTypeGuildPublicThread, discordgo.ChannelTypeGuildPrivateThread:
		return true
	default:
		return false
	}
}
