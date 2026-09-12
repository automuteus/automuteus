package bot

import (
	"context"
	"log/slog"
	"time"

	"github.com/automuteus/automuteus/v8/bot/tokenprovider"
	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/lock"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	storageutils "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
	"github.com/top-gg/go-dbl"
)

// The interfaces below are the seams between the game-event logic (phase transitions, player updates, voice changes,
// status messages) and the infrastructure it talks to. Each one is the narrowest surface the game path actually uses,
// and each is satisfied by the concrete production type, so wiring is unchanged in production while tests can supply
// in-memory fakes and drive the bot with spoofed capture/Discord events.

// GameStateStore loads and persists the per-game Discord state, and hands out the locks that guard it.
type GameStateStore interface {
	GetDiscordGameStateAndLock(gsr GameStateRequest) (lock.Lock, *GameState)
	GetReadOnlyDiscordGameState(gsr GameStateRequest) *GameState
	SetDiscordGameState(dgs *GameState, l lock.Lock)
	LockVoiceChanges(connectCode string, dur time.Duration) lock.Lock
	LockSnowflake(snowflake string) lock.Lock
	GetUsernameOrUserIDMappings(guildID, key string) (map[string]any, error)
	RemoveOldGame(guildID, connectCode string)
	DeleteDiscordGameState(dgs *GameState)
}

// VoiceModifier applies mute/deafen changes to Discord users.
type VoiceModifier interface {
	ModifyUsers(guildID, connectCode string, req task.UserModifyRequest, l lock.Lock) error
}

// PremiumSource answers what premium tier a guild (or user) currently holds.
type PremiumSource interface {
	GetGuildOrUserPremiumStatus(official bool, dbl *dbl.Client, guildID, userID string) (premium.Tier, int, error)
}

// GameRecorder persists match history.
type GameRecorder interface {
	AddInitialGame(game *storageutils.PostgresGame) (uint64, error)
	EnsureUserExists(userID uint64) (*storageutils.PostgresUser, error)
	UpdateGameAndPlayers(gameID int64, winType int16, endTime int64, players []*storageutils.PostgresUserGame) error
	AddEvent(event *storageutils.PostgresGameEvent) error
	AbortGame(gameID int64, endTime int64) error
}

// DiscordClient is the slice of the Discord REST API the game path uses to send, edit, and delete messages, and to
// look up members that are not in the local state cache.
type DiscordClient interface {
	ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageEditComplex(m *discordgo.MessageEdit, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageDelete(channelID, messageID string, options ...discordgo.RequestOption) error
	GuildMember(guildID, userID string, options ...discordgo.RequestOption) (*discordgo.Member, error)
}

// GuildReader reads cached guild state (members and voice states). *discordgo.State satisfies it, and can be
// populated offline in tests.
type GuildReader interface {
	Guild(guildID string) (*discordgo.Guild, error)
}

// SettingsSource loads a guild's settings.
type SettingsSource interface {
	LoadGuildSettings(ctx context.Context, guildID string) (*settings.GuildSettings, error)
}

// RequestMetrics records how many Discord API requests the bot has issued, by type.
type RequestMetrics interface {
	RecordDiscordRequests(requestType server.EventType, num int64)
}

// Compile-time checks that the production types satisfy the seams.
var (
	_ GameStateStore = (*RedisInterface)(nil)
	_ VoiceModifier  = (*tokenprovider.TokenProvider)(nil)
	_ PremiumSource  = (*storageutils.PsqlInterface)(nil)
	_ GameRecorder   = (*storageutils.PsqlInterface)(nil)
	_ DiscordClient  = (*discordgo.Session)(nil)
	_ GuildReader    = (*discordgo.State)(nil)
	_ RequestMetrics = redisMetrics{}
	_ SettingsSource = (*storage.StorageInterface)(nil)
)

// redisMetrics adapts the package-level metrics recorder to RequestMetrics.
type redisMetrics struct {
	client *redis.Client
}

func (m redisMetrics) RecordDiscordRequests(requestType server.EventType, num int64) {
	server.RecordDiscordRequests(m.client, requestType, num)
}

// useProductionDeps points every seam at the real infrastructure.
func (bot *Bot) useProductionDeps(sess *discordgo.Session, redisInterface *RedisInterface, storageInterface *storage.StorageInterface, psql *storageutils.PsqlInterface) {
	bot.store = redisInterface
	bot.settings = storageInterface
	bot.recorder = psql
	bot.premiumSource = psql
	bot.discord = sess
	bot.guilds = sess.State
	bot.metrics = redisMetrics{client: redisInterface.client}
	bot.notices = redisNotices{client: redisInterface.client}
	bot.sleep = time.Sleep
	bot.log = slog.Default()
}

// SetTokenProvider installs the provider used to issue mute/deafen requests.
func (bot *Bot) SetTokenProvider(tp *tokenprovider.TokenProvider) {
	bot.voice = tp
}

// gameLog returns a logger carrying the identifiers of the game a request refers to, so that log lines from
// concurrent games can be told apart.
func (bot *Bot) gameLog(gsr GameStateRequest) *slog.Logger {
	l := bot.log.With("guild", gsr.GuildID)
	if gsr.ConnectCode != "" {
		l = l.With("code", gsr.ConnectCode)
	}
	if gsr.VoiceChannel != "" {
		l = l.With("voice_channel", gsr.VoiceChannel)
	}
	return l
}

// premiumTier resolves the effective premium tier for a guild, treating expired premium as free.
func (bot *Bot) premiumTier(guildID string) premium.Tier {
	tier, days, _ := bot.premiumSource.GetGuildOrUserPremiumStatus(bot.official, nil, guildID, "")
	if premium.IsExpired(tier, days) {
		return premium.FreeTier
	}
	return tier
}
