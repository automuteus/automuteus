package discord

import (
	"fmt"
	"github.com/automuteus/utils/pkg/settings"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	redis_common "github.com/automuteus/automuteus/common"
	"github.com/automuteus/automuteus/metrics"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/premium"
	"github.com/automuteus/utils/pkg/task"
	"github.com/bsm/redislock"

	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func (bot *Bot) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// IgnoreSpectator all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	if redis_common.IsUserBanned(bot.RedisInterface.client, m.Author.ID) {
		return
	}

	lock := bot.RedisInterface.LockSnowflake(m.ID)
	// couldn't obtain lock; bail bail bail!
	if lock == nil {
		return
	}
	defer lock.Release(ctx)

	g, err := s.State.Guild(m.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	contents := m.Content
	sett := bot.StorageInterface.GetGuildSettings(m.GuildID)

	// can be a guild's old prefix setting, or @AutoMuteUs
	prefix := sett.GetCommandPrefix()

	globalPrefix := os.Getenv("AUTOMUTEUS_GLOBAL_PREFIX")
	if globalPrefix != "" && strings.HasPrefix(contents, globalPrefix) {
		// if the global matches, then use that for future processing/control flow using the prefix
		prefix = globalPrefix
	}

	// TODO regex
	// have to check the actual mention format, not the explicit string "@AutoMuteUs"
	mention := "<@!" + s.State.User.ID + ">"
	altMention := "<@" + s.State.User.ID + ">"
	if strings.HasPrefix(contents, prefix) || strings.HasPrefix(contents, mention) || strings.HasPrefix(contents, altMention) {
		if redis_common.IsUserRateLimitedGeneral(bot.RedisInterface.client, m.Author.ID) {
			banned := redis_common.IncrementRateLimitExceed(bot.RedisInterface.client, m.Author.ID)
			if banned {
				s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
					ID:    "message_handlers.softban",
					Other: "I'm ignoring {{.User}} for the next 5 minutes, stop spamming",
				},
					map[string]interface{}{
						"User": mentionByUserID(m.Author.ID),
					}))
			} else {
				msg, err := s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
					ID:    "message_handlers.generalRatelimit",
					Other: "{{.User}}, you're issuing commands too fast! Please slow down!",
				},
					map[string]interface{}{
						"User": mentionByUserID(m.Author.ID),
					}))
				if err == nil {
					go func() {
						time.Sleep(time.Second * 3)
						s.ChannelMessageDelete(m.ChannelID, msg.ID)
					}()
				}
			}

			return
		}
		redis_common.MarkUserRateLimit(bot.RedisInterface.client, m.Author.ID, "", 0)

		contents = removePrefixOrMention(contents, prefix, mention, altMention)

		isAdmin, isPermissioned := false, false

		if g.OwnerID == m.Author.ID || (len(sett.AdminUserIDs) == 0 && len(sett.PermissionRoleIDs) == 0) {
			// the guild owner should always have both permissions
			// or if both permissions are still empty everyone get both
			isAdmin = true
			isPermissioned = true
		} else {
			// if we have no admins, then we MUST have mods as per the check above.
			if len(sett.AdminUserIDs) == 0 {
				// we have no admins, but we have mods, so make sure users fulfill that check
				isAdmin = sett.HasRolePerms(m.Member)
			} else {
				// we have admins; make sure user is one
				isAdmin = sett.HasAdminPerms(m.Author)
			}
			// even if we have admins, we can grant mod if the moderators role is empty; it is lesser permissions
			isPermissioned = len(sett.PermissionRoleIDs) == 0 || sett.HasRolePerms(m.Member)
		}

		deleteUserMessage := false
		if len(contents) == 0 {
			if len(prefix) <= 1 {
				// prevent bot from spamming help message whenever the single character
				// prefix is sent by mistake
				return
			}
			embed := helpResponse(isAdmin, isPermissioned, allCommands, sett)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
			deleteUserMessage = true
		} else {
			args := strings.Split(contents, " ")

			for i, v := range args {
				args[i] = strings.ToLower(v)
			}

			deleteUserMessage = bot.HandleCommand(isAdmin, isPermissioned, sett, s, g, m, args)
		}
		if deleteUserMessage {
			deleteMessage(s, m.ChannelID, m.ID)
			metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, 1)
		}
	}
}

// TODO refactor to use regex, could do the matching + removal easier
func removePrefixOrMention(contents, prefix, mention, altMention string) string {
	oldLen := len(contents)
	contents = strings.Replace(contents, prefix+" ", "", 1)
	contents = strings.Replace(contents, mention+" ", "", 1)
	contents = strings.Replace(contents, altMention+" ", "", 1)
	if len(contents) == oldLen { // wasn't replaced (no space)
		contents = strings.Replace(contents, prefix, "", 1)
		contents = strings.Replace(contents, mention, "", 1)
		contents = strings.Replace(contents, altMention, "", 1)
	}
	return contents
}

func (bot *Bot) handleReactionGameStartAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	// IgnoreSpectator all reactions created by the bot itself
	if m.UserID == s.State.User.ID {
		return
	}

	if redis_common.IsUserBanned(bot.RedisInterface.client, m.UserID) {
		return
	}

	lock := bot.RedisInterface.LockSnowflake(m.MessageID + m.UserID + m.Emoji.ID)
	// couldn't obtain lock; bail bail bail!
	if lock == nil {
		return
	}
	defer lock.Release(ctx)

	g, err := s.State.Guild(m.GuildID)
	if err != nil {
		log.Println(err)
		return
	}

	// TODO explicitly unmute/undeafen users that unlink. Current control flow won't do it (ala discord bots not being undeafened)

	sett := bot.StorageInterface.GetGuildSettings(m.GuildID)

	gsr := GameStateRequest{
		GuildID:     m.GuildID,
		TextChannel: m.ChannelID,
	}
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	if lock != nil && dgs != nil && dgs.Exists() {
		// verify that the User is reacting to the state/status message
		if dgs.IsReactionTo(m) {
			if redis_common.IsUserRateLimitedGeneral(bot.RedisInterface.client, m.UserID) {
				banned := redis_common.IncrementRateLimitExceed(bot.RedisInterface.client, m.UserID)
				if banned {
					s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
						ID:    "message_handlers.softban",
						Other: "I'm ignoring {{.User}} for the next 5 minutes, stop spamming",
					},
						map[string]interface{}{
							"User": mentionByUserID(m.UserID),
						}))
				} else {
					msg, err := s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
						ID:    "message_handlers.handleReactionGameStartAdd.generalRatelimit",
						Other: "{{.User}}, you're reacting too fast! Please slow down!",
					}, map[string]interface{}{
						"User": mentionByUserID(m.UserID),
					}))
					if err == nil {
						go func() {
							time.Sleep(time.Second * 3)
							s.ChannelMessageDelete(m.ChannelID, msg.ID)
						}()
					}
				}
				return
			}
			redis_common.MarkUserRateLimit(bot.RedisInterface.client, m.UserID, "Reaction", redis_common.ReactionRateLimitDuration)
			idMatched := false
			if m.Emoji.Name == "▶️" {
				metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.ReactionAdd, 14)
				go removeReaction(bot.PrimarySession, m.ChannelID, m.MessageID, m.Emoji.Name, m.UserID)
				go removeReaction(bot.PrimarySession, m.ChannelID, m.MessageID, m.Emoji.Name, "@me")
				go dgs.AddAllReactions(bot.PrimarySession, bot.StatusEmojis[true])
			} else {
				for color, e := range bot.StatusEmojis[true] {
					if e.ID == m.Emoji.ID {
						idMatched = true
						log.Print(fmt.Sprintf("Player %s reacted with color %s\n", m.UserID, game.GetColorStringForInt(color)))
						// the User doesn't exist in our userdata cache; add them
						user, added := dgs.checkCacheAndAddUser(g, s, m.UserID)
						if !added {
							log.Println("No users found in Discord for UserID " + m.UserID)
							idMatched = false
						} else {
							auData, found := dgs.AmongUsData.GetByColor(game.GetColorStringForInt(color))
							if found {
								user.Link(auData)
								dgs.UpdateUserData(m.UserID, user)
								go bot.RedisInterface.AddUsernameLink(m.GuildID, m.UserID, auData.Name)
							} else {
								log.Println("I couldn't find any player data for that color; is your capture linked?")
								idMatched = false
							}
						}

						// then remove the player's reaction if we matched, or if we didn't
						go s.MessageReactionRemove(m.ChannelID, m.MessageID, e.FormatForReaction(), m.UserID)
						break
					}
				}
				if !idMatched {
					// log.Println(m.Emoji.Name)
					if m.Emoji.Name == "❌" {
						log.Println("Removing player " + m.UserID)
						idMatched = dgs.ClearPlayerData(m.UserID)
						go s.MessageReactionRemove(m.ChannelID, m.MessageID, "❌", m.UserID)
					}
				}
				// make sure to update any voice changes if they occurred
				if idMatched {
					bot.handleTrackedMembers(bot.PrimarySession, sett, 0, NoPriority, gsr)
					edited := dgs.Edit(s, bot.gameStateResponse(dgs, sett))
					if edited {
						metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
					}
				}
			}
		}
		bot.RedisInterface.SetDiscordGameState(dgs, lock)
	}
}

// voiceStateChange handles more edge-case behavior for users moving between voice channels, and catches when
// relevant discord api requests are fully applied successfully. Otherwise, we can issue multiple requests for
// the same mute/unmute, erroneously
func (bot *Bot) handleVoiceStateChange(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	snowFlakeLock := bot.RedisInterface.LockSnowflake(m.ChannelID + m.UserID + m.SessionID)
	// couldn't obtain lock; bail bail bail!
	if snowFlakeLock == nil {
		return
	}
	defer snowFlakeLock.Release(ctx)

	prem, days := bot.PostgresInterface.GetGuildPremiumStatus(m.GuildID)
	premTier := premium.FreeTier
	if !premium.IsExpired(prem, days) {
		premTier = prem
	}

	sett := bot.StorageInterface.GetGuildSettings(m.GuildID)
	gsr := GameStateRequest{
		GuildID:      m.GuildID,
		VoiceChannel: m.ChannelID,
	}

	stateLock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	if stateLock == nil {
		return
	}
	defer stateLock.Release(ctx)

	var voiceLock *redislock.Lock
	if dgs.ConnectCode != "" {
		voiceLock = bot.RedisInterface.LockVoiceChanges(dgs.ConnectCode, time.Second)
		if voiceLock == nil {
			return
		}
	}

	g, err := s.State.Guild(dgs.GuildID)

	if err != nil || g == nil {
		return
	}

	// fetch the userData from our userData data cache
	userData, err := dgs.GetUser(m.UserID)
	if err != nil {
		// the User doesn't exist in our userdata cache; add them
		userData, _ = dgs.checkCacheAndAddUser(g, s, m.UserID)
	}

	tracked := m.ChannelID != "" && dgs.VoiceChannel == m.ChannelID

	auData, found := dgs.AmongUsData.GetByName(userData.InGameName)

	var isAlive bool

	// only actually tracked if we're in a tracked channel AND linked to a player
	if !sett.GetMuteSpectator() {
		tracked = tracked && found
		isAlive = auData.IsAlive
	} else {
		if !found {
			// we just assume the spectator is dead
			isAlive = false
		} else {
			isAlive = auData.IsAlive
		}
	}
	mute, deaf := sett.GetVoiceState(isAlive, tracked, dgs.AmongUsData.GetPhase())
	// check the userdata is linked here to not accidentally undeafen music bots, for example
	if found && (userData.ShouldBeDeaf != deaf || userData.ShouldBeMute != mute) && (mute != m.Mute || deaf != m.Deaf) {
		userData.SetShouldBeMuteDeaf(mute, deaf)

		dgs.UpdateUserData(m.UserID, userData)

		if dgs.Running {
			uid, _ := strconv.ParseUint(m.UserID, 10, 64)
			req := task.UserModifyRequest{
				Premium: premTier,
				Users: []task.UserModify{
					{
						UserID: uid,
						Mute:   mute,
						Deaf:   deaf,
					},
				},
			}
			mdsc := bot.GalactusClient.ModifyUsers(m.GuildID, dgs.ConnectCode, req, voiceLock)
			if mdsc == nil {
				log.Println("Nil response from modifyUsers, probably not good...")
			} else {
				go RecordDiscordRequestsByCounts(bot.RedisInterface.client, mdsc)
			}
		}
	}
	bot.RedisInterface.SetDiscordGameState(dgs, stateLock)
}

//func (bot *Bot) handleNewGameMessage(m *discordgo.MessageCreate, g *discordgo.Guild, sett *settings.GuildSettings) (string, interface{}) {
//	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(GameStateRequest{
//		GuildID:     m.GuildID,
//		TextChannel: m.ChannelID,
//	})
//	retries := 0
//	for lock == nil {
//		if retries > 10 {
//			log.Println("DEADLOCK in obtaining game state lock, upon calling new")
//			return m.ChannelID, "I wasn't able to make a new game, maybe try in a different text channel?"
//		}
//		retries++
//		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(GameStateRequest{
//			GuildID:     m.GuildID,
//			TextChannel: m.ChannelID,
//		})
//	}
//
//	if redis_common.IsUserRateLimitedSpecific(bot.RedisInterface.client, m.Author.ID, "NewGame") {
//		defer lock.Release(context.Background())
//		banned := redis_common.IncrementRateLimitExceed(bot.RedisInterface.client, m.Author.ID)
//		if banned {
//			return m.ChannelID, sett.LocalizeMessage(&i18n.Message{
//				ID:    "message_handlers.softban",
//				Other: "{{.User}} I'm ignoring your messages for the next 5 minutes, stop spamming",
//			}, map[string]interface{}{
//				"User": mentionByUserID(m.Author.ID),
//			})
//		} else {
//			return m.ChannelID, sett.LocalizeMessage(&i18n.Message{
//				ID:    "message_handlers.handleNewGameMessage.specificRatelimit",
//				Other: "{{.User}} You're creating games too fast! Please slow down!",
//			}, map[string]interface{}{
//				"User": mentionByUserID(m.Author.ID),
//			})
//		}
//	}
//
//	redis_common.MarkUserRateLimit(bot.RedisInterface.client, m.Author.ID, "NewGame", redis_common.NewGameRateLimitDuration)
//}

func (bot *Bot) handleGameStartMessage(guildID, textChannelID, voiceChannelID, userID string, sett *settings.GuildSettings, g *discordgo.Guild, connCode string) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(GameStateRequest{
		GuildID:     guildID,
		TextChannel: textChannelID,
		ConnectCode: connCode,
	})
	if lock == nil {
		log.Println("Couldn't obtain lock for DGS on game start...")
		return
	}
	dgs.AmongUsData.SetRoomRegionMap("", "", game.EMPTYMAP)

	dgs.clearGameTracking(bot.PrimarySession)

	dgs.Running = true

	if voiceChannelID != "" {
		dgs.VoiceChannel = voiceChannelID
		for _, v := range g.VoiceStates {
			if v.ChannelID == voiceChannelID {
				dgs.checkCacheAndAddUser(g, bot.PrimarySession, v.UserID)
			}
		}
	}

	dgs.CreateMessage(bot.PrimarySession, bot.gameStateResponse(dgs, sett), textChannelID, userID)

	// release the lock
	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	// log.Println("Added self game state message")
	// +18 emojis, 1 for X
	metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.ReactionAdd, 19)

	go dgs.AddAllReactions(bot.PrimarySession, bot.StatusEmojis[true])
}
