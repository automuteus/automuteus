package discord

import (
	"context"
	"fmt"
	"github.com/automuteus/galactus/broker"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/premium"
	"github.com/automuteus/utils/pkg/task"
	"github.com/bsm/redislock"
	redis_common "github.com/denverquane/amongusdiscord/common"
	"github.com/denverquane/amongusdiscord/discord/command"
	"github.com/denverquane/amongusdiscord/metrics"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/denverquane/amongusdiscord/storage"

	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const DefaultMaxActiveGames = 150

const downloadURL = "https://capture.automute.us"

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
	prefix := sett.GetCommandPrefix()

	if strings.Contains(m.Content, "<@!"+s.State.User.ID+">") {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "message_handlers.handleMessageCreate.respondPrefix",
			Other: "I respond to the prefix {{.CommandPrefix}}",
		},
			map[string]interface{}{
				"CommandPrefix": prefix,
			}))
		return
	}

	globalPrefix := os.Getenv("AUTOMUTEUS_GLOBAL_PREFIX")
	if globalPrefix != "" && strings.HasPrefix(contents, globalPrefix) {
		// if the global matches, then use that for future processing/control flow using the prefix
		prefix = globalPrefix
	}

	if strings.HasPrefix(contents, prefix) {
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

		oldLen := len(contents)
		contents = strings.Replace(contents, prefix+" ", "", 1)
		if len(contents) == oldLen { // didn't have a space
			contents = strings.Replace(contents, prefix, "", 1)
		}

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

		if len(contents) == 0 {
			if len(prefix) <= 1 {
				// prevent bot from spamming help message whenever the single character
				// prefix is sent by mistake
				return
			}
			embed := helpResponse(isAdmin, isPermissioned, prefix, command.AllCommands, sett)
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
			// delete the user's message
			deleteMessage(s, m.ChannelID, m.ID)
		} else {
			args := strings.Split(contents, " ")

			for i, v := range args {
				args[i] = strings.ToLower(v)
			}

			bot.HandleCommand(isAdmin, isPermissioned, sett, s, g, m, args)
		}
	}
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
						dgs.ClearPlayerData(m.UserID)
						go s.MessageReactionRemove(m.ChannelID, m.MessageID, "❌", m.UserID)
						idMatched = true
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

	prem, _ := bot.PostgresInterface.GetGuildPremiumStatus(m.GuildID)

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

	tracked := m.ChannelID != "" && dgs.Tracking.ChannelID == m.ChannelID

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
				Premium: prem,
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

func (bot *Bot) handleNewGameMessage(s *discordgo.Session, m *discordgo.MessageCreate, g *discordgo.Guild, sett *storage.GuildSettings) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(GameStateRequest{
		GuildID:     m.GuildID,
		TextChannel: m.ChannelID,
	})
	retries := 0
	for lock == nil {
		if retries > 10 {
			log.Println("DEADLOCK in obtaining game state lock, upon calling new")
			s.ChannelMessageSend(m.ChannelID, "I wasn't able to make a new game, maybe try in a different text channel?")
			return
		}
		retries++
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(GameStateRequest{
			GuildID:     m.GuildID,
			TextChannel: m.ChannelID,
		})
	}

	if redis_common.IsUserRateLimitedSpecific(bot.RedisInterface.client, m.Author.ID, "NewGame") {
		banned := redis_common.IncrementRateLimitExceed(bot.RedisInterface.client, m.Author.ID)
		if banned {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "message_handlers.softban",
				Other: "{{.User}} I'm ignoring your messages for the next 5 minutes, stop spamming",
			}, map[string]interface{}{
				"User": mentionByUserID(m.Author.ID),
			}))
		} else {
			s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "message_handlers.handleNewGameMessage.specificRatelimit",
				Other: "{{.User}} You're creating games too fast! Please slow down!",
			}, map[string]interface{}{
				"User": mentionByUserID(m.Author.ID),
			}))
		}
		lock.Release(context.Background())
		return
	}

	redis_common.MarkUserRateLimit(bot.RedisInterface.client, m.Author.ID, "NewGame", redis_common.NewGameRateLimitDuration)

	channels, err := s.GuildChannels(m.GuildID)
	if err != nil {
		log.Println(err)
	}

	tracking := TrackingChannel{}

	// loop over all the channels in the discord and cross-reference with the one that the .au new author is in
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildVoice {
			for _, v := range g.VoiceStates {
				// if the User who typed au new is in a voice channel
				if v.UserID == m.Author.ID {
					// once we find the voice channel
					if channel.ID == v.ChannelID {
						tracking = TrackingChannel{
							ChannelID:   channel.ID,
							ChannelName: channel.Name,
						}
						break
					}
				}
			}
		}
	}
	if tracking.ChannelID == "" {
		s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "message_handlers.handleNewGameMessage.noChannel",
			Other: "{{.User}}, please join a voice channel before starting a match!",
		}, map[string]interface{}{
			"User": mentionByUserID(m.Author.ID),
		}))
		lock.Release(context.Background())
		return
	}

	// allow people with a previous game going to be able to make new games
	if dgs.GameStateMsg.MessageID != "" {
		if v, ok := bot.EndGameChannels[dgs.ConnectCode]; ok {
			v <- true
		}
		delete(bot.EndGameChannels, dgs.ConnectCode)

		dgs.Reset()
	} else {
		premStatus, _ := bot.PostgresInterface.GetGuildPremiumStatus(m.GuildID)
		// Premium users should always be allowed to start new games; only check the free guilds
		if premStatus == premium.FreeTier {
			activeGames := broker.GetActiveGames(bot.RedisInterface.client, GameTimeoutSeconds)
			act := os.Getenv("MAX_ACTIVE_GAMES")
			num, err := strconv.ParseInt(act, 10, 64)
			if err != nil {
				num = DefaultMaxActiveGames
			}
			if activeGames > num {
				s.ChannelMessageSend(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
					ID: "message_handlers.handleNewGameMessage.lockout",
					Other: "If I start any more games, Discord will lock me out, or throttle the games I'm running! 😦\n" +
						"Please try again in a few minutes, or consider AutoMuteUs Premium (`{{.CommandPrefix}} premium`)\n" +
						"Current Games: {{.Games}}",
				}, map[string]interface{}{
					"CommandPrefix": sett.CommandPrefix,
					"Games":         fmt.Sprintf("%d/%d", activeGames, num),
				}))
				lock.Release(context.Background())
				return
			}
		}
	}

	connectCode := generateConnectCode(m.GuildID)

	dgs.ConnectCode = connectCode

	bot.RedisInterface.RefreshActiveGame(m.GuildID, connectCode)

	killChan := make(chan EndGameMessage)

	go bot.SubscribeToGameByConnectCode(m.GuildID, connectCode, killChan)

	dgs.Subscribed = true

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	bot.ChannelsMapLock.Lock()
	bot.EndGameChannels[connectCode] = killChan
	bot.ChannelsMapLock.Unlock()

	hyperlink, minimalURL := formCaptureURL(bot.url, connectCode)

	var embed = discordgo.MessageEmbed{
		URL:  "",
		Type: "",
		Title: sett.LocalizeMessage(&i18n.Message{
			ID:    "message_handlers.handleNewGameMessage.embed.Title",
			Other: "You just started a game!",
		}),
		Description: sett.LocalizeMessage(&i18n.Message{
			ID: "message_handlers.handleNewGameMessage.embed.Description",
			Other: "Click the following link to link your capture: \n <{{.hyperlink}}>\n\n" +
				"Don't have the capture installed? Latest version [here]({{.downloadURL}})\n\nTo link your capture manually:",
		},
			map[string]interface{}{
				"hyperlink":   hyperlink,
				"downloadURL": downloadURL,
			}),
		Timestamp: "",
		Color:     3066993, // GREEN
		Image:     nil,
		Thumbnail: nil,
		Video:     nil,
		Provider:  nil,
		Author:    nil,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "message_handlers.handleNewGameMessage.embed.Fields.URL",
					Other: "URL",
				}),
				Value:  minimalURL,
				Inline: true,
			},
			{
				Name: sett.LocalizeMessage(&i18n.Message{
					ID:    "message_handlers.handleNewGameMessage.embed.Fields.Code",
					Other: "Code",
				}),
				Value:  connectCode,
				Inline: true,
			},
		},
	}

	log.Println("Generated URL for connection: " + hyperlink)

	sendMessageDM(s, m.Author.ID, &embed)

	bot.handleGameStartMessage(s, m, sett, tracking, g, connectCode)
}

func (bot *Bot) handleGameStartMessage(s *discordgo.Session, m *discordgo.MessageCreate, sett *storage.GuildSettings, channel TrackingChannel, g *discordgo.Guild, connCode string) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(GameStateRequest{
		GuildID:     m.GuildID,
		TextChannel: m.ChannelID,
		ConnectCode: connCode,
	})
	if lock == nil {
		log.Println("Couldn't obtain lock for DGS on game start...")
		return
	}
	dgs.AmongUsData.SetRoomRegionMap("", "", game.EMPTYMAP)

	dgs.clearGameTracking(s)

	dgs.Running = true

	if channel.ChannelName != "" {
		dgs.Tracking = TrackingChannel{
			ChannelID:   channel.ChannelID,
			ChannelName: channel.ChannelName,
		}
		for _, v := range g.VoiceStates {
			if v.ChannelID == channel.ChannelID {
				dgs.checkCacheAndAddUser(g, s, v.UserID)
			}
		}
	}

	dgs.CreateMessage(s, bot.gameStateResponse(dgs, sett), m.ChannelID, m.Author.ID)

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	// log.Println("Added self game state message")
	// +12 emojis, 1 for X
	metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.ReactionAdd, 13)

	go dgs.AddAllReactions(bot.PrimarySession, bot.StatusEmojis[true])
}
