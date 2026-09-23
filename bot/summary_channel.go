package bot

import (
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

// summaryChannel returns the channel the match summary is posted to. The configured summary channel is used only
// when it is a text-capable channel of this guild; otherwise fallback (the game's own text channel) is used.
//
// The stored ID is client-supplied, through the slash command or the HTTP API, so it is checked against the
// guild's cached channel list rather than trusted. Without this check an administrator of one guild could point
// the bot at a channel in another guild it happens to share.
func (bot *Bot) summaryChannel(guildID string, sett *settings.GuildSettings, fallback string) string {
	configured := sett.GetMatchSummaryChannelID()
	if configured == "" {
		return fallback
	}
	if bot.guilds == nil {
		bot.log.Warn("match summary channel configured but guild state is unavailable; using game channel",
			"guild", guildID, "channel", configured)
		return fallback
	}
	guild, err := bot.guilds.Guild(guildID)
	if err != nil || guild == nil {
		bot.log.Warn("match summary channel configured but guild is not cached; using game channel",
			"guild", guildID, "channel", configured, "err", err)
		return fallback
	}
	channel := findChannel(guild, configured)
	if channel == nil {
		bot.log.Warn("match summary channel is not in this guild; using game channel",
			"guild", guildID, "channel", configured)
		return fallback
	}
	if !discord.IsTextCapableChannel(channel.Type) {
		bot.log.Warn("match summary channel is not a text channel; using game channel",
			"guild", guildID, "channel", configured, "type", channel.Type)
		return fallback
	}
	return configured
}

func findChannel(guild *discordgo.Guild, channelID string) *discordgo.Channel {
	for _, list := range [][]*discordgo.Channel{guild.Channels, guild.Threads} {
		for _, channel := range list {
			if channel != nil && channel.ID == channelID {
				return channel
			}
		}
	}
	return nil
}
