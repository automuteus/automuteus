package rediskey

const TotalGuildsSet = "automuteus:count:guilds"
const ActiveGamesZSet = "automuteus:games"
const EventsNamespace = "automuteus:capture:events"
const JobNamespace = "automuteus:jobs:"

const TotalUsers = "automuteus:users:total"
const TotalGames = "automuteus:games:total"

const Commit = "automuteus:commit"
const Version = "automuteus:version"

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

func GuildTokenLock(guildID, hToken string) string {
	return "automuteus:muterequest:lock:" + hToken + ":" + guildID
}

func RoomCodesForConnCode(connCode string) string {
	return "automuteus:roomcode:" + connCode
}

func CachedUserInfoOnGuild(userID, guildID string) string {
	return "automuteus:cache:userinfo:" + guildID + ":" + userID
}

func UserRateLimitGeneral(userID string) string {
	return "automuteus:ratelimit:user:" + userID
}

func UserRateLimitSpecific(userID, cmdType string) string {
	return "automuteus:ratelimit:user:" + cmdType + ":" + userID
}

func UserSoftban(userID string) string {
	return "automuteus:ratelimit:softban:user:" + userID
}

func UserSoftbanCount(userID string) string {
	return "automuteus:ratelimit:softban:count:user:" + userID
}
