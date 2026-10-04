package bot

import (
	"context"
	"time"

	"github.com/automuteus/automuteus/v8/bot/command"
	"github.com/automuteus/automuteus/v8/bot/tokenprovider"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// Reminders point a guild at something it should fix, such as premium worker bots that were never invited. One is
// chosen when a game starts and stored on the game, so it stays at the bottom of the status message for the whole
// game instead of coming and going between renders. A guild sees each kind at most once per ReminderCooldown; an
// admin can press the reminder's button to hide it for ReminderSnooze, but it cannot be turned off.

// ReminderKind identifies one kind of reminder; it is part of the guild's Redis key, so values must stay stable.
type ReminderKind string

const (
	// ReminderMissingWorkers tells a Silver or Gold guild that some of its priority mute bots are not in the server.
	ReminderMissingWorkers ReminderKind = "missing-workers"
)

const (
	ReminderCooldown = 24 * time.Hour
	ReminderSnooze   = 7 * 24 * time.Hour
)

const dismissReminderID = "dismiss-reminder"

// Reminder is what a game's status message reminds the guild of.
type Reminder struct {
	Kind ReminderKind `json:"kind"`
	Tier premium.Tier `json:"tier"`
	// Present and Needed count the guild's worker bots, for ReminderMissingWorkers.
	Present int `json:"present"`
	Needed  int `json:"needed"`
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
	return &Reminder{Kind: ReminderMissingWorkers, Tier: tier, Present: present, Needed: needed}
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
	dgs.Reminder = nil
	bot.store.SetDiscordGameState(dgs, lock)
	bot.DispatchRefreshOrEdit(dgs, gsr, sett)
	return command.PrivateResponse(sett.LocalizeMessage(&i18n.Message{
		ID:    "reminders.dismiss.done",
		Other: "Got it! This reminder is hidden for a week.",
	}))
}
