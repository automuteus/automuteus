package bot

import (
	"context"
	"time"

	"github.com/automuteus/automuteus/v8/bot/command"
	"github.com/automuteus/automuteus/v8/bot/tokenprovider"
	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// Reminders point a guild at something it should fix, such as premium worker bots that were never invited. One is
// chosen when a game starts, or later if the game's mutes turn out slow, and stored on the game, so it stays at the
// bottom of the status message for the rest of the game instead of coming and going between renders. A guild sees each kind at most once per ReminderCooldown; an
// admin can press the reminder's button to hide it for ReminderSnooze, but it cannot be turned off.

// ReminderKind identifies one kind of reminder; it is part of the guild's Redis key, so values must stay stable.
type ReminderKind string

const (
	// ReminderMissingWorkers tells a Silver or Gold guild that some of its priority mute bots are not in the server.
	ReminderMissingWorkers ReminderKind = "missing-workers"
	// ReminderSlowMutes tells a Free or Bronze guild whose mutes are slow that Silver or Gold priority mute bots would
	// speed them up.
	ReminderSlowMutes ReminderKind = "slow-mutes"
)

var reminderKinds = [...]ReminderKind{ReminderMissingWorkers, ReminderSlowMutes}

// reminderKindNames lists every reminder kind, for exposing their metrics before any is shown.
func reminderKindNames() []string {
	names := make([]string, len(reminderKinds))
	for i, kind := range reminderKinds {
		names[i] = string(kind)
	}
	return names
}

const (
	ReminderCooldown = 24 * time.Hour
	ReminderSnooze   = 7 * 24 * time.Hour
)

const (
	// SlowMuteThreshold is how long a round of mutes must take to count as slow. Picked from
	// automuteus_mute_batch_duration_seconds: in early October 2026, 13% of all batches took over 5s. That histogram
	// times each batch, while a round can be two (priority players first), so this is slightly easier to reach.
	SlowMuteThreshold = 5 * time.Second
	// SlowMuteMinUsers keeps small rounds, such as one player joining late, from counting.
	SlowMuteMinUsers = 5
	// SlowMutesToRemind is how many slow rounds a game needs before the reminder appears, so one slow moment doesn't.
	SlowMutesToRemind = 2
)

const dismissReminderID = "dismiss-reminder"

// Reminder is what a game's status message reminds the guild of.
type Reminder struct {
	Kind ReminderKind `json:"kind"`
	Tier premium.Tier `json:"tier"`
	// Present and Needed count the guild's worker bots, for ReminderMissingWorkers.
	Present int `json:"present"`
	Needed  int `json:"needed"`
	// Seconds is how long the slow round of mutes took, for ReminderSlowMutes.
	Seconds int `json:"seconds,omitempty"`
}

type redisReminders struct {
	client *redis.Client
}

func (r redisReminders) ClaimReminder(ctx context.Context, guildID string, kind ReminderKind, cooldown time.Duration) (bool, error) {
	return r.client.SetNX(ctx, rediskey.Reminder(guildID, string(kind)), "", cooldown).Result()
}

func (r redisReminders) SnoozeReminder(ctx context.Context, guildID string, kind ReminderKind, d time.Duration) error {
	return r.client.Set(ctx, rediskey.Reminder(guildID, string(kind)), "", d).Err()
}

// startReminder picks the reminder for a game starting in guildID, or nil. Errors are logged and treated as nothing
// to remind, so a reminder never gets in the way of starting a game.
func (bot *Bot) startReminder(guildID string) *Reminder {
	if bot.workers == nil || bot.reminders == nil {
		return nil
	}
	// The inventory is in memory, so check it before spending a premium lookup.
	present, total, known := bot.workers.WorkersPresent(guildID)
	if !known || present >= total {
		return nil
	}
	tier := bot.premiumTier(guildID)
	if tier != premium.SilverTier && tier != premium.GoldTier {
		return nil
	}
	needed := min(tokenprovider.PremiumBotConstraints[tier], total)
	if present >= needed || !bot.canInviteWorker(tier, guildID) {
		return nil
	}
	claimed, err := bot.reminders.ClaimReminder(ctx, guildID, ReminderMissingWorkers, ReminderCooldown)
	if err != nil {
		bot.log.Error("failed to claim reminder", "guild", guildID, "kind", ReminderMissingWorkers, "err", err)
		return nil
	}
	if !claimed {
		return nil
	}
	bot.metrics.RecordReminder(string(ReminderMissingWorkers), server.ReminderShown)
	return &Reminder{Kind: ReminderMissingWorkers, Tier: tier, Present: present, Needed: needed}
}

// slowMuteReminder counts round toward the game in gsr's slow rounds of mutes, and returns a ReminderSlowMutes once
// the game has had SlowMutesToRemind of them, or nil. A round only counts if the guild has no priority mute bots and
// the bot's own token did most of the muting, since that is what Silver or Gold would speed up; a capture client doing
// the muting is a different bottleneck.
func (bot *Bot) slowMuteReminder(gsr GameStateRequest, tier premium.Tier, round task.ModifyResult, users int) *Reminder {
	if bot.reminders == nil || (tier != premium.FreeTier && tier != premium.BronzeTier) {
		return nil
	}
	if users < SlowMuteMinUsers || round.Elapsed < SlowMuteThreshold || round.Official <= round.Capture+round.Worker {
		return nil
	}
	lock, dgs := bot.store.GetDiscordGameStateAndLock(gsr)
	if lock == nil {
		return nil
	}
	if !dgs.Running || dgs.Reminder != nil {
		bot.store.SetDiscordGameState(nil, lock)
		return nil
	}
	dgs.SlowMutes++
	var reminder *Reminder
	if dgs.SlowMutes >= SlowMutesToRemind {
		claimed, err := bot.reminders.ClaimReminder(ctx, dgs.GuildID, ReminderSlowMutes, ReminderCooldown)
		if err != nil {
			bot.gameLog(gsr).Error("failed to claim reminder", "kind", ReminderSlowMutes, "err", err)
		} else if claimed {
			reminder = &Reminder{Kind: ReminderSlowMutes, Tier: tier, Seconds: int(round.Elapsed.Round(time.Second).Seconds())}
			dgs.Reminder = reminder
			bot.metrics.RecordReminder(string(ReminderSlowMutes), server.ReminderShown)
		}
	}
	bot.store.SetDiscordGameState(dgs, lock)
	return reminder
}

// canInviteWorker reports whether /premium invites offers guildID a bot it can add.
func (bot *Bot) canInviteWorker(tier premium.Tier, guildID string) bool {
	offered, _ := command.OfferedBots(tier, bot.workerStatus(guildID))
	for _, b := range offered {
		if !b.Member {
			return true
		}
	}
	return false
}

// workerStatus reports each worker bot's membership in guildID and whether it is full, for /premium invites. A bot
// this process can't see counts as neither, so it is still offered.
func (bot *Bot) workerStatus(guildID string) command.BotStatus {
	return func(botID string) (bool, bool) {
		if bot.workers == nil {
			return false, false
		}
		member, full, known := bot.workers.WorkerStatus(botID, guildID)
		return known && member, known && full
	}
}

// applyReminder adds r to the bottom of a game status embed.
func applyReminder(embed *discordgo.MessageEmbed, r *Reminder, sett *settings.GuildSettings) {
	if embed == nil || r == nil {
		return
	}
	switch r.Kind {
	case ReminderMissingWorkers:
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "reminders.missingWorkers.title",
				Other: "💡 Priority mute bots missing",
			}),
			Value: sett.LocalizeMessage(&i18n.Message{
				ID:    "reminders.missingWorkers.desc",
				Other: "This server has AutoMuteUs {{.Tier}}, which includes priority mute bots for faster muting, but {{.Missing}} of {{.Needed}} aren't in this server. Use `/premium invites` to add them!",
			}, map[string]interface{}{
				"Tier":    premium.TierStrings[r.Tier],
				"Missing": r.Needed - r.Present,
				"Needed":  r.Needed,
			}),
			Inline: false,
		})
	case ReminderSlowMutes:
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name: sett.LocalizeMessage(&i18n.Message{
				ID:    "reminders.slowMutes.title",
				Other: "🐢 Muting is slow in this server",
			}),
			Value: sett.LocalizeMessage(&i18n.Message{
				ID:    "reminders.slowMutes.desc",
				Other: "Muting is taking about {{.Seconds}} seconds per round in this game, because Discord limits how quickly a single bot can mute. AutoMuteUs Silver and Gold add priority mute bots that share the work, so mutes happen much faster. See `/premium info` to learn more!",
			}, map[string]interface{}{
				"Seconds": r.Seconds,
			}),
			Inline: false,
		})
	}
}

// reminderComponents is the button that hides a game's reminder, if it has one.
func reminderComponents(r *Reminder, sett *settings.GuildSettings) []discordgo.MessageComponent {
	if r == nil {
		return nil
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					CustomID: dismissReminderID,
					Style:    discordgo.SecondaryButton,
					Label: sett.LocalizeMessage(&i18n.Message{
						ID:    "reminders.dismiss.button",
						Other: "Hide this for a week",
					}),
				},
			},
		},
	}
}

// dismissReminder hides the reminder on the game in gsr's channel for ReminderSnooze. Only admins may call it.
func (bot *Bot) dismissReminder(gsr GameStateRequest, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	lock, dgs := bot.store.GetDiscordGameStateAndLock(gsr)
	if lock == nil {
		return command.DeadlockGameStateResponse(dismissReminderID, sett)
	}
	if dgs.Reminder == nil {
		// already dismissed, perhaps by another admin
		bot.store.SetDiscordGameState(nil, lock)
		return command.PrivateResponse(ThumbsUp)
	}
	if err := bot.reminders.SnoozeReminder(ctx, dgs.GuildID, dgs.Reminder.Kind, ReminderSnooze); err != nil {
		bot.store.SetDiscordGameState(nil, lock)
		bot.gameLog(gsr).Error("failed to snooze reminder", "kind", dgs.Reminder.Kind, "err", err)
		return command.PrivateErrorResponse(dismissReminderID, err, sett)
	}
	bot.metrics.RecordReminder(string(dgs.Reminder.Kind), server.ReminderDismissed)
	dgs.Reminder = nil
	bot.store.SetDiscordGameState(dgs, lock)
	bot.DispatchRefreshOrEdit(dgs, gsr, sett)
	return command.PrivateResponse(sett.LocalizeMessage(&i18n.Message{
		ID:    "reminders.dismiss.done",
		Other: "Got it! This reminder is hidden for a week.",
	}))
}
