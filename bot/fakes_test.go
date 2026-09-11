package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/lock"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	storageutils "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/bwmarrin/discordgo"
	"github.com/top-gg/go-dbl"
)

// In-memory implementations of the seams in deps.go. Together with newTestBot they let a test drive the bot with
// spoofed capture jobs and Discord events, then inspect exactly which mutes, messages, and match records resulted.

type fakeLock struct{}

func (fakeLock) Release(context.Context) error { return nil }

// memoryStore holds a single game's state. Reads hand back a JSON round-tripped copy, mirroring the Redis-backed
// store's behavior, so a caller that forgets to write its changes back will not see them on the next read.
type memoryStore struct {
	mu       sync.Mutex
	state    []byte
	mappings map[string]map[string]interface{}
}

func newMemoryStore() *memoryStore {
	return &memoryStore{mappings: map[string]map[string]interface{}{}}
}

func (m *memoryStore) put(dgs *GameState) {
	b, err := json.Marshal(dgs)
	if err != nil {
		panic(err)
	}
	m.mu.Lock()
	m.state = b
	m.mu.Unlock()
}

func (m *memoryStore) get() *GameState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		return nil
	}
	var dgs GameState
	if err := json.Unmarshal(m.state, &dgs); err != nil {
		panic(err)
	}
	return &dgs
}

func (m *memoryStore) GetDiscordGameStateAndLock(GameStateRequest) (lock.Lock, *GameState) {
	return fakeLock{}, m.get()
}

func (m *memoryStore) GetReadOnlyDiscordGameState(GameStateRequest) *GameState { return m.get() }

func (m *memoryStore) SetDiscordGameState(dgs *GameState, _ lock.Lock) {
	if dgs != nil {
		m.put(dgs)
	}
}

func (m *memoryStore) LockVoiceChanges(string, time.Duration) lock.Lock { return fakeLock{} }
func (m *memoryStore) LockSnowflake(string) lock.Lock                   { return fakeLock{} }

func (m *memoryStore) GetUsernameOrUserIDMappings(guildID, key string) (map[string]interface{}, error) {
	return m.mappings[guildID+":"+key], nil
}

type fakeVoice struct {
	mu       sync.Mutex
	requests []task.UserModifyRequest
	err      error
}

func (f *fakeVoice) ModifyUsers(_, _ string, req task.UserModifyRequest, l lock.Lock) error {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	if l != nil {
		_ = l.Release(context.Background())
	}
	return f.err
}

func (f *fakeVoice) all() []task.UserModifyRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]task.UserModifyRequest(nil), f.requests...)
}

type fakePremium struct{ tier premium.Tier }

func (f fakePremium) GetGuildOrUserPremiumStatus(bool, *dbl.Client, string, string) (premium.Tier, int, error) {
	return f.tier, premium.NoExpiryCode, nil
}

type fakeRecorder struct {
	mu      sync.Mutex
	games   []*storageutils.PostgresGame
	events  []*storageutils.PostgresGameEvent
	updates []int64
}

func (f *fakeRecorder) AddInitialGame(g *storageutils.PostgresGame) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.games = append(f.games, g)
	return uint64(len(f.games)), nil
}

func (f *fakeRecorder) EnsureUserExists(userID uint64) (*storageutils.PostgresUser, error) {
	return &storageutils.PostgresUser{UserID: userID}, nil
}

func (f *fakeRecorder) UpdateGameAndPlayers(gameID int64, _ int16, _ int64, _ []*storageutils.PostgresUserGame) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, gameID)
	return nil
}

func (f *fakeRecorder) AddEvent(e *storageutils.PostgresGameEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
	return nil
}

// fakeDiscord records outbound REST calls and fabricates message IDs for anything it "sends".
type fakeDiscord struct {
	mu      sync.Mutex
	nextID  int
	sent    []*discordgo.Message
	edits   []*discordgo.MessageEdit
	deleted []string
	members map[string]*discordgo.Member
}

func (f *fakeDiscord) newMessage(channelID string) *discordgo.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	msg := &discordgo.Message{ID: fmt.Sprintf("msg-%d", f.nextID), ChannelID: channelID}
	f.sent = append(f.sent, msg)
	return msg
}

func (f *fakeDiscord) ChannelMessageSend(channelID, content string, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	msg := f.newMessage(channelID)
	msg.Content = content
	return msg, nil
}

func (f *fakeDiscord) ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	msg := f.newMessage(channelID)
	msg.Embeds = []*discordgo.MessageEmbed{embed}
	return msg, nil
}

func (f *fakeDiscord) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	msg := f.newMessage(channelID)
	if data.Embed != nil {
		msg.Embeds = []*discordgo.MessageEmbed{data.Embed}
	}
	return msg, nil
}

func (f *fakeDiscord) ChannelMessageEditComplex(m *discordgo.MessageEdit, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.edits = append(f.edits, m)
	return &discordgo.Message{ID: m.ID, ChannelID: m.Channel}, nil
}

func (f *fakeDiscord) ChannelMessageDelete(channelID, messageID string, _ ...discordgo.RequestOption) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, channelID+"/"+messageID)
	return nil
}

func (f *fakeDiscord) GuildMember(_, userID string, _ ...discordgo.RequestOption) (*discordgo.Member, error) {
	if m, ok := f.members[userID]; ok {
		return m, nil
	}
	return nil, errors.New("member not found: " + userID)
}

type fakeSettings struct{ sett *settings.GuildSettings }

func (f fakeSettings) LoadGuildSettings(context.Context, string) (*settings.GuildSettings, error) {
	return f.sett, nil
}

type fakeMetrics struct {
	mu     sync.Mutex
	counts map[server.EventType]int64
}

func (f *fakeMetrics) RecordDiscordRequests(t server.EventType, n int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts == nil {
		f.counts = map[server.EventType]int64{}
	}
	f.counts[t] += n
}

// testDeps bundles the fakes so a test can inspect what the bot did.
type testDeps struct {
	store    *memoryStore
	voice    *fakeVoice
	recorder *fakeRecorder
	discord  *fakeDiscord
	guilds   *discordgo.State
	metrics  *fakeMetrics
	settings *settings.GuildSettings
	sleeps   []time.Duration
	logs     *bytes.Buffer
}

// newTestBot returns a Bot wired entirely to in-memory fakes. Delays requested by the mute path are recorded in
// deps.sleeps instead of actually sleeping.
func newTestBot(t *testing.T) (*Bot, *testDeps) {
	t.Helper()
	deps := &testDeps{
		store:    newMemoryStore(),
		voice:    &fakeVoice{},
		recorder: &fakeRecorder{},
		discord:  &fakeDiscord{members: map[string]*discordgo.Member{}},
		guilds:   discordgo.NewState(),
		metrics:  &fakeMetrics{},
		settings: settings.MakeGuildSettings(),
		logs:     &bytes.Buffer{},
	}
	bot := &Bot{
		StatusEmojis:    emptyStatusEmojis(),
		EndGameChannels: map[string]chan EndGameMessage{},
		captureTimeout:  GameTimeoutSeconds,
		store:           deps.store,
		settings:        fakeSettings{sett: deps.settings},
		voice:           deps.voice,
		premiumSource:   fakePremium{tier: premium.FreeTier},
		recorder:        deps.recorder,
		discord:         deps.discord,
		guilds:          deps.guilds,
		metrics:         deps.metrics,
		sleep:           func(d time.Duration) { deps.sleeps = append(deps.sleeps, d) },
		log:             slog.New(slog.NewTextHandler(deps.logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	return bot, deps
}
