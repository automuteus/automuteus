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
	"github.com/automuteus/automuteus/v8/pkg/notice"
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

// memoryStore holds games keyed by connect code. Reads hand back a JSON round-tripped copy, mirroring the
// Redis-backed store's behavior, so a caller that forgets to write its changes back will not see them on the next
// read. Lookups resolve the way the Redis store's pointers do: by connect code, then text channel, then voice channel.
type memoryStore struct {
	mu       sync.Mutex
	states   map[string][]byte
	mappings map[string]map[string]interface{}
}

func newMemoryStore() *memoryStore {
	return &memoryStore{states: map[string][]byte{}, mappings: map[string]map[string]interface{}{}}
}

func (m *memoryStore) put(dgs *GameState) {
	b, err := json.Marshal(dgs)
	if err != nil {
		panic(err)
	}
	m.mu.Lock()
	m.states[dgs.ConnectCode] = b
	m.mu.Unlock()
}

func decodeState(b []byte) *GameState {
	var dgs GameState
	if err := json.Unmarshal(b, &dgs); err != nil {
		panic(err)
	}
	return &dgs
}

// get returns the only stored game, for tests that seed exactly one. It panics if there are several.
func (m *memoryStore) get() *GameState {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch len(m.states) {
	case 0:
		return nil
	case 1:
		for _, b := range m.states {
			return decodeState(b)
		}
	}
	panic("memoryStore.get with several games stored; use getCode")
}

func (m *memoryStore) getCode(connectCode string) *GameState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := m.states[connectCode]; ok {
		return decodeState(b)
	}
	return nil
}

func (m *memoryStore) find(gsr GameStateRequest) *GameState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := m.states[gsr.ConnectCode]; ok && gsr.ConnectCode != "" {
		return decodeState(b)
	}
	for _, b := range m.states {
		dgs := decodeState(b)
		if gsr.TextChannel != "" && dgs.GameStateMsg.MessageChannelID == gsr.TextChannel {
			return dgs
		}
		if gsr.VoiceChannel != "" && dgs.VoiceChannel == gsr.VoiceChannel {
			return dgs
		}
	}
	return nil
}

func (m *memoryStore) GetDiscordGameStateAndLock(gsr GameStateRequest) (lock.Lock, *GameState) {
	dgs := m.find(gsr)
	if dgs == nil {
		// like the Redis store, a locked fetch creates an empty state if none exists
		dgs = NewDiscordGameState(gsr.GuildID)
	}
	return fakeLock{}, dgs
}

func (m *memoryStore) RemoveOldGame(string, string) {}

func (m *memoryStore) DeleteDiscordGameState(dgs *GameState) {
	m.mu.Lock()
	delete(m.states, dgs.ConnectCode)
	m.mu.Unlock()
}

func (m *memoryStore) GetReadOnlyDiscordGameState(gsr GameStateRequest) *GameState {
	return m.find(gsr)
}

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
	aborted []int64
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

func (f *fakeRecorder) AbortGame(gameID int64, _ int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.aborted = append(f.aborted, gameID)
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

func (f *fakeDiscord) editCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.edits)
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

type fakeNotices struct {
	mu sync.Mutex
	n  *notice.Notice
}

func (f *fakeNotices) Active(context.Context) (*notice.Notice, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.n, nil
}

func (f *fakeNotices) set(n *notice.Notice) {
	f.mu.Lock()
	f.n = n
	f.mu.Unlock()
}

type fakeMetrics struct {
	mu              sync.Mutex
	counts          map[server.EventType]int64
	activeGames     int
	started         int
	ended           map[server.EndReason]int
	cleanupFailures map[server.CleanupStep]int
}

func (f *fakeMetrics) RecordDiscordRequests(t server.EventType, n int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts == nil {
		f.counts = map[server.EventType]int64{}
	}
	f.counts[t] += n
}

func (f *fakeMetrics) SetActiveGames(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.activeGames = n
}

func (f *fakeMetrics) RecordGameStarted() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started++
}

func (f *fakeMetrics) RecordGameEnded(reason server.EndReason) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ended == nil {
		f.ended = map[server.EndReason]int{}
	}
	f.ended[reason]++
}

func (f *fakeMetrics) RecordCleanupFailure(step server.CleanupStep) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cleanupFailures == nil {
		f.cleanupFailures = map[server.CleanupStep]int{}
	}
	f.cleanupFailures[step]++
}

func (f *fakeMetrics) endedBy(reason server.EndReason) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ended[reason]
}

func (f *fakeMetrics) cleanupFailed(step server.CleanupStep) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cleanupFailures[step]
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
	notices  *fakeNotices
	logs     *bytes.Buffer

	sleepMu sync.Mutex
	sleeps  []time.Duration
}

func (d *testDeps) recordSleep(dur time.Duration) {
	d.sleepMu.Lock()
	d.sleeps = append(d.sleeps, dur)
	d.sleepMu.Unlock()
}

func (d *testDeps) slept() []time.Duration {
	d.sleepMu.Lock()
	defer d.sleepMu.Unlock()
	return append([]time.Duration(nil), d.sleeps...)
}

// eventually polls cond for up to a second.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
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
		notices:  &fakeNotices{},
		logs:     &bytes.Buffer{},
	}
	bot := &Bot{
		StatusEmojis:       emptyStatusEmojis(),
		EndGameChannels:    map[string]chan EndGameMessage{},
		activeGameRequests: map[string]GameStateRequest{},
		notices:            deps.notices,
		captureTimeout:     GameTimeoutSeconds,
		store:              deps.store,
		settings:           fakeSettings{sett: deps.settings},
		voice:              deps.voice,
		premiumSource:      fakePremium{tier: premium.FreeTier},
		recorder:           deps.recorder,
		discord:            deps.discord,
		guilds:             deps.guilds,
		metrics:            deps.metrics,
		sleep:              deps.recordSleep,
		log:                slog.New(slog.NewTextHandler(deps.logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	return bot, deps
}
