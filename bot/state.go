package bot

import "github.com/bwmarrin/discordgo"

// configureStateTracking turns off the parts of discordgo's State cache that nothing in the bot reads.
// The bot only ever consults guilds, channels, roles, members, and voice states (for permission checks and
// mute/deafen decisions). These flags stop incremental updates for the rest; trimGuildCache drops the
// initial payload, which the State stores in full regardless of the flags.
func configureStateTracking(s *discordgo.Session) {
	s.State.TrackEmojis = false
	s.State.TrackThreads = false
	s.State.TrackThreadMembers = false
	s.State.TrackPresences = false
	s.AddHandler(trimGuildCache)
}

// trimGuildCache drops the parts of a freshly cached guild that the bot never reads.
// GUILD_CREATE fires for every guild on startup and again on every gateway re-identify, and discordgo
// dispatches each event to the State before user handlers, so by the time this runs the guild is already
// cached and this is the last word on its contents.
func trimGuildCache(s *discordgo.Session, m *discordgo.GuildCreate) {
	if !s.StateEnabled || m.Guild == nil {
		return
	}
	g, err := s.State.Guild(m.ID)
	if err != nil {
		return
	}

	// Threads are also indexed in the State's private channel map, so remove them through the public API
	// rather than only clearing the slice. Iterate over a copy because ChannelRemove edits g.Threads.
	for _, t := range append([]*discordgo.Channel(nil), g.Threads...) {
		_ = s.State.ChannelRemove(t)
	}

	// Handlers run concurrently with permission checks that read the same guild, so hold the State lock.
	s.State.Lock()
	g.Emojis = nil
	g.Stickers = nil
	g.Threads = nil
	g.Presences = nil
	g.StageInstances = nil
	s.State.Unlock()
}
