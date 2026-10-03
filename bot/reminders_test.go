package bot

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/bwmarrin/discordgo"
)

type fakeWorkers struct {
	present, total int
	known          bool
	// status maps a worker's user ID to whether it is in the guild and whether it is full; absent means unknown
	status map[string][2]bool
}

func (f fakeWorkers) WorkersPresent(string) (int, int, bool) { return f.present, f.total, f.known }

func (f fakeWorkers) WorkerStatus(botID, _ string) (member, full, known bool) {
	s, ok := f.status[botID]
	return s[0], s[1], ok
}

const (
	amu1 = "780323275624546304"
	amu2 = "769022114229125181"
	amu3 = "780323801173983262"
	amu4 = "780589033033302036"
	amu5 = "780589278195220480"
)

// fakeReminders holds back kinds per guild the way the Redis keys do, without expiry.
type fakeReminders struct {
	mu       sync.Mutex
	held     map[string]time.Duration
	claims   int
	claimErr error
}

func newFakeReminders() *fakeReminders { return &fakeReminders{held: map[string]time.Duration{}} }

func (f *fakeReminders) ClaimReminder(_ context.Context, guildID string, kind ReminderKind, cooldown time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claims++
	if f.claimErr != nil {
		return false, f.claimErr
	}
	key := guildID + ":" + string(kind)
	if _, held := f.held[key]; held {
		return false, nil
	}
	f.held[key] = cooldown
	return true, nil
}

func (f *fakeReminders) SnoozeReminder(_ context.Context, guildID string, kind ReminderKind, d time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.held[guildID+":"+string(kind)] = d
	return nil
}

func (f *fakeReminders) heldFor(guildID string, kind ReminderKind) time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.held[guildID+":"+string(kind)]
}

func TestStartReminder(t *testing.T) {
	tests := []struct {
		name      string
		tier      premium.Tier
		workers   fakeWorkers
		heldBack  bool
		claimErr  error
		want      *Reminder
		wantClaim bool
	}{
		{name: "gold missing two", tier: premium.GoldTier, workers: fakeWorkers{present: 1, total: 3, known: true},
			want: &Reminder{Kind: ReminderMissingWorkers, Tier: premium.GoldTier, Present: 1, Needed: 3}, wantClaim: true},
		{name: "silver missing its one", tier: premium.SilverTier, workers: fakeWorkers{present: 0, total: 3, known: true},
			want: &Reminder{Kind: ReminderMissingWorkers, Tier: premium.SilverTier, Present: 0, Needed: 1}, wantClaim: true},
		{name: "silver has any one worker", tier: premium.SilverTier, workers: fakeWorkers{present: 1, total: 3, known: true}},
		{name: "gold has every worker", tier: premium.GoldTier, workers: fakeWorkers{present: 3, total: 3, known: true}},
		{name: "needed capped by workers configured", tier: premium.GoldTier, workers: fakeWorkers{present: 1, total: 2, known: true},
			want: &Reminder{Kind: ReminderMissingWorkers, Tier: premium.GoldTier, Present: 1, Needed: 2}, wantClaim: true},
		{name: "inventory unknown", tier: premium.GoldTier, workers: fakeWorkers{present: 0, total: 3, known: false}},
		{name: "bronze has no workers to invite", tier: premium.BronzeTier, workers: fakeWorkers{present: 0, total: 3, known: true}},
		{name: "trial has no workers to invite", tier: premium.TrialTier, workers: fakeWorkers{present: 0, total: 3, known: true}},
		{name: "free", tier: premium.FreeTier, workers: fakeWorkers{present: 0, total: 3, known: true}},
		{name: "self-hosted", tier: premium.SelfHostTier, workers: fakeWorkers{present: 0, total: 3, known: true}},
		{name: "shown within the cooldown", tier: premium.GoldTier, workers: fakeWorkers{present: 0, total: 3, known: true}, heldBack: true, wantClaim: true},
		{name: "claim fails", tier: premium.GoldTier, workers: fakeWorkers{present: 0, total: 3, known: true}, claimErr: errors.New("redis down"), wantClaim: true},
		{name: "missing bots are all full", tier: premium.GoldTier,
			workers: fakeWorkers{present: 1, total: 3, known: true, status: map[string][2]bool{amu1: {true, false}, amu2: {false, true}, amu3: {false, true}, amu4: {false, true}, amu5: {false, true}}}},
		{name: "silver's only bot is already in", tier: premium.SilverTier,
			// another worker left in the meantime, so the count is short but there is nothing to invite
			workers: fakeWorkers{present: 0, total: 3, known: true, status: map[string][2]bool{amu1: {true, false}}}},
		{name: "only the last bot has room", tier: premium.GoldTier,
			workers: fakeWorkers{present: 1, total: 3, known: true, status: map[string][2]bool{amu1: {true, false}, amu2: {false, true}, amu3: {false, true}, amu4: {false, true}, amu5: {false, false}}},
			want:    &Reminder{Kind: ReminderMissingWorkers, Tier: premium.GoldTier, Present: 1, Needed: 3}, wantClaim: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bot, _ := newTestBot(t)
			reminders := newFakeReminders()
			reminders.claimErr = tt.claimErr
			if tt.heldBack {
				reminders.held[scenarioGuild+":"+string(ReminderMissingWorkers)] = ReminderCooldown
			}
			bot.premiumSource = fakePremium{tier: tt.tier}
			bot.workers = tt.workers
			bot.reminders = reminders

			got := bot.startReminder(scenarioGuild)
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("startReminder = %+v, want %+v", got, tt.want)
			}
			if claimed := reminders.claims > 0; claimed != tt.wantClaim {
				t.Errorf("claimed cooldown = %v, want %v", claimed, tt.wantClaim)
			}
			if tt.want != nil && reminders.heldFor(scenarioGuild, ReminderMissingWorkers) != ReminderCooldown {
				t.Errorf("held back for %v, want %v", reminders.heldFor(scenarioGuild, ReminderMissingWorkers), ReminderCooldown)
			}
		})
	}
}

func TestReminder_ShownAtStartAndDismissedForAWeek(t *testing.T) {
	bot, deps := newTestBot(t)
	reminders := newFakeReminders()
	bot.premiumSource = fakePremium{tier: premium.GoldTier}
	bot.workers = fakeWorkers{present: 1, total: 3, known: true}
	bot.reminders = reminders

	dgs := NewDiscordGameState(scenarioGuild)
	dgs.ConnectCode = scenarioConnectCode
	deps.store.put(dgs)

	bot.handleGameStartMessage(scenarioGuild, scenarioTextChannel, "", "1", deps.settings, &discordgo.Guild{ID: scenarioGuild}, scenarioConnectCode)

	if len(deps.discord.sent) != 1 {
		t.Fatalf("sent %d messages, want the status message", len(deps.discord.sent))
	}
	msg := deps.discord.sent[0]
	if !hasReminderField(msg.Embeds[0]) {
		t.Fatalf("status message has no reminder: %+v", msg.Embeds[0].Fields)
	}
	if !hasDismissButton(msg.Components) {
		t.Fatalf("status message has no dismiss button: %+v", msg.Components)
	}

	resp := bot.dismissReminder(GameStateRequest{GuildID: scenarioGuild, TextChannel: scenarioTextChannel}, deps.settings)
	if resp == nil || resp.Data == nil || resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("dismiss response = %+v, want an ephemeral confirmation", resp)
	}
	if got := reminders.heldFor(scenarioGuild, ReminderMissingWorkers); got != ReminderSnooze {
		t.Errorf("held back for %v after dismissal, want %v", got, ReminderSnooze)
	}
	if got := deps.store.getCode(scenarioConnectCode); got.Reminder != nil {
		t.Errorf("game still carries reminder %+v", got.Reminder)
	}

	eventually(t, "a status message edit", func() bool { return deps.discord.editCount() == 1 })
	deps.discord.mu.Lock()
	edit := deps.discord.edits[0]
	deps.discord.mu.Unlock()
	if hasReminderField(edit.Embeds[0]) {
		t.Error("edit after dismissal still shows the reminder")
	}
	if hasDismissButton(edit.Components) {
		t.Error("edit after dismissal still shows the dismiss button")
	}
	if len(edit.Components) != 1 {
		t.Errorf("edit has %d component rows, want just the color select", len(edit.Components))
	}

	// the next game in the same week shows nothing
	bot.handleGameStartMessage(scenarioGuild, scenarioTextChannel, "", "1", deps.settings, &discordgo.Guild{ID: scenarioGuild}, scenarioConnectCode)
	if got := deps.store.getCode(scenarioConnectCode); got.Reminder != nil {
		t.Errorf("reminder shown again while snoozed: %+v", got.Reminder)
	}
}

func hasReminderField(embed *discordgo.MessageEmbed) bool {
	for _, f := range embed.Fields {
		if strings.Contains(f.Name, "Priority mute bots missing") {
			return true
		}
	}
	return false
}

func hasDismissButton(rows []discordgo.MessageComponent) bool {
	for _, row := range rows {
		for _, c := range row.(discordgo.ActionsRow).Components {
			if b, ok := c.(discordgo.Button); ok && b.CustomID == dismissReminderID {
				return true
			}
		}
	}
	return false
}
