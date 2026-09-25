package rediskey

const TotalGuildsSet = "automuteus:count:guilds"
const ActiveGamesZSet = "automuteus:games"

// ActiveGamesByGuildZSet scores "guildID:connectCode" members by last activity, so a bot process can discover the
// games running in the guilds it serves without scanning every guild. See bot.(*Bot).discoverGames.
const ActiveGamesByGuildZSet = "automuteus:games:byguild"
const EventsNamespace = "automuteus:capture:events"
const JobNamespace = "automuteus:jobs:"

const TotalUsers = "automuteus:users:total"

// NoticeChannel carries platform notices (see pkg/notice) to every bot shard.
const NoticeChannel = "automuteus:notices"

// ActiveNotice holds the notice currently shown to players, if any.
const ActiveNotice = "automuteus:notices:active"

// StatsChangedChannel carries announcements that a guild's recorded match history changed (see
// notice.AnnounceStatsChanged), so the API can drop its cached stats documents instead of waiting out their TTL.
const StatsChangedChannel = "automuteus:stats:changed"
const TotalGames = "automuteus:games:total"

func ActiveGamesForGuild(guildID string) string {
	return "automuteus:discord:" + guildID + ":games:set"
}

func TextChannelPtr(guildID, channelID string) string {
	return "automuteus:discord:" + guildID + ":pointer:text:" + channelID
}

func VoiceChannelPtr(guildID, channelID string) string {
	return "automuteus:discord:" + guildID + ":pointer:voice:" + channelID
}

func ConnectCodePtr(guildID, code string) string {
	return "automuteus:discord:" + guildID + ":pointer:code:" + code
}

func ConnectCodeData(guildID, connCode string) string {
	return "automuteus:discord:" + guildID + ":" + connCode
}

func GuildCacheHash(guildID string) string {
	return "automuteus:discord:" + guildID + ":cache"
}

func SnowflakeLockID(snowflake string) string {
	return "automuteus:snowflake:" + snowflake + ":lock"
}

func VoiceChangesForGameCodeLock(connectCode string) string {
	return "automuteus:voice:game:" + connectCode + ":lock"
}

// GameConsumerLease is held by the one bot process currently allowed to consume a game's capture events, so that
// events are applied strictly in order even though several processes subscribe to the game. See bot/lease.go.
func GameConsumerLease(connectCode string) string {
	return "automuteus:games:consumer:" + connectCode
}

func CompleteTask(taskID string) string {
	return "automuteus:tasks:complete:ack:" + taskID
}

func TasksList(connectCode string) string {
	return "automuteus:tasks:list:" + connectCode
}

func BotTokenIdentifyLock(token string) string {
	return "automuteus:token:lock" + token
}

// APISettingsWriteLimit counts settings writes for a guild made through the HTTP API in the current window, so
// one guild cannot be rewritten in a tight loop. The window is set by the API when the key is created.
func APISettingsWriteLimit(guildID string) string {
	return "automuteus:api:ratelimit:settings:" + string(HashGuildID(guildID))
}

// GuildTokenLock counts recent mute/deafen requests issued through a token (or the capture client, keyed by connect
// code) for a guild, so the bot can route around Discord's per-guild rate limit.
func GuildTokenLock(guildID, hToken string) string {
	return "automuteus:muterequest:lock:" + hToken + ":" + guildID
}

// MuteBlacklist marks a token (or the capture client, keyed by connect code) as unusable for mute/deafen requests in a
// guild until the key expires.
func MuteBlacklist(guildID, hToken string) string {
	return "automuteus:muterequest:blacklist:" + hToken + ":" + guildID
}

// CaptureMuteReady is set by Galactus while a capture client that can apply mutes itself is connected for a connect
// code. The bot only sends mute tasks to the capture client when this key exists.
func CaptureMuteReady(connectCode string) string {
	return "automuteus:capture:muteready:" + connectCode
}

// RoomCodesForConnCode is written by Galactus (cmd/galactus) whenever a capture client reports a lobby,
// mapping a connect code to the current Among Us room code. Forked repos/communities read it directly, and the bot
// serves it via GET /game/roomcode, so the key name must stay stable.
func RoomCodesForConnCode(connCode string) string {
	return "automuteus:roomcode:" + connCode
}

// CachedPlayerProfile holds the API's record of a user in a guild (name, nickname, avatar) as JSON, for
// the stats page.
func CachedPlayerProfile(userID, guildID string) string {
	return "automuteus:cache:profile:" + guildID + ":" + userID
}
