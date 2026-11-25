package rediskey

// Keys for leaving servers that stopped using the bot; see internal/prune.

// GuildPruneLeaveBudget exists while the fleet must not leave another inactive server: every bot process shares it,
// so its TTL is the pace of the whole fleet.
const GuildPruneLeaveBudget = "automuteus:guilds:prune:leave-budget"

// GuildPruneDryRun is the set of server IDs a dry run would have left. Its size is the answer a dry run exists for.
const GuildPruneDryRun = "automuteus:guilds:prune:dry-run"

// GuildPruneLeftCount counts the inactive servers the fleet has left.
const GuildPruneLeftCount = "automuteus:guilds:prune:left"

// GuildPruned exists briefly after the bot leaves a server for inactivity, so the processes that then see it leave
// can tell that from being removed, and keep the server's settings for its return.
func GuildPruned(guildID string) string {
	return "automuteus:guilds:prune:pruned:" + guildID
}

// GuildRecentUse exists while a server that recently started a game with /new must not be left for inactivity. Match
// history alone would miss a server whose games never got as far as a match.
func GuildRecentUse(guildID string) string {
	return "automuteus:guilds:recent-use:" + guildID
}
