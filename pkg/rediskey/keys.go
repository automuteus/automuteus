package rediskey

const TotalGuildsSet = "automuteus:count:guilds"
const ActiveGamesZSet = "automuteus:games"
const EventsNamespace = "automuteus:capture:events"
const JobNamespace = "automuteus:jobs:"

const TotalUsers = "automuteus:users:total"

// NoticeChannel carries platform notices (see pkg/notice) to every bot shard.
const NoticeChannel = "automuteus:notices"

// ActiveNotice holds the notice currently shown to players, if any.
const ActiveNotice = "automuteus:notices:active"
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

func RequestsByType(typeStr string) string {
	return "automuteus:requests:type:" + typeStr
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

func GuildSettings(id HashedID) string {
	return "automuteus:settings:guild:" + string(id)
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

func CachedUserInfoOnGuild(userID, guildID string) string {
	return "automuteus:cache:userinfo:" + guildID + ":" + userID
}
