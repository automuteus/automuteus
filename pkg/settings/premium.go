package settings

// PremiumSnapshot captures the values of the settings that only premium guilds
// may change, so a write path can tell which of them a request actually
// altered. Take it before applying a request, then call Changed on the result.
//
// The set mirrors the /settings slash command: match summary deletion and
// channel, auto refresh, every leaderboard option, spectator muting, and room
// code display. Changing any of them on a non-premium guild must be refused;
// echoing the stored value back is not a change and is allowed.
type PremiumSnapshot struct {
	deleteGameSummaryMinutes int
	matchSummaryChannelID    string
	autoRefresh              bool
	leaderboardMention       bool
	leaderboardSize          int
	leaderboardMin           int
	muteSpectator            bool
	displayRoomCode          string
}

// PremiumSnapshot records the current premium-only values.
func (gs *GuildSettings) PremiumSnapshot() PremiumSnapshot {
	return PremiumSnapshot{
		deleteGameSummaryMinutes: gs.DeleteGameSummaryMinutes,
		matchSummaryChannelID:    gs.MatchSummaryChannelID,
		autoRefresh:              gs.AutoRefresh,
		leaderboardMention:       gs.LeaderboardMention,
		leaderboardSize:          gs.LeaderboardSize,
		leaderboardMin:           gs.LeaderboardMin,
		muteSpectator:            gs.MuteSpectator,
		displayRoomCode:          gs.DisplayRoomCode,
	}
}

// Changed returns the JSON field names of the premium-only settings whose
// value in gs differs from the snapshot, in document order. Nil means nothing
// premium-gated was touched.
func (p PremiumSnapshot) Changed(gs *GuildSettings) []string {
	var changed []string
	if gs.DeleteGameSummaryMinutes != p.deleteGameSummaryMinutes {
		changed = append(changed, "deleteGameSummary")
	}
	if gs.AutoRefresh != p.autoRefresh {
		changed = append(changed, "autoRefresh")
	}
	if gs.MatchSummaryChannelID != p.matchSummaryChannelID {
		changed = append(changed, "matchSummaryChannelID")
	}
	if gs.LeaderboardMention != p.leaderboardMention {
		changed = append(changed, "leaderboardMention")
	}
	if gs.LeaderboardSize != p.leaderboardSize {
		changed = append(changed, "leaderboardSize")
	}
	if gs.LeaderboardMin != p.leaderboardMin {
		changed = append(changed, "leaderboardMin")
	}
	if gs.MuteSpectator != p.muteSpectator {
		changed = append(changed, "muteSpectator")
	}
	if gs.DisplayRoomCode != p.displayRoomCode {
		changed = append(changed, "displayRoomCode")
	}
	return changed
}
