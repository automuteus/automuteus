package discord

import (
	"context"
	"fmt"
	galactus_client "github.com/automuteus/galactus/pkg/client"
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/premium"
	"github.com/automuteus/utils/pkg/rediskey"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/denverquane/amongusdiscord/discord/command"
	"github.com/go-redsync/redsync/v4"
	"go.uber.org/zap"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const DefaultMaxActiveGames = 150

const downloadURL = "https://capture.automute.us"

func (bot *Bot) handleMessageCreate(m discordgo.MessageCreate) {
	contents := m.Content
	sett := bot.StorageInterface.GetGuildSettings(m.GuildID)
	prefix := sett.GetCommandPrefix()

	g, err := bot.GalactusClient.GetGuild(m.GuildID)
	if err != nil {
		log.Println(err)
		return
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
		embed := helpResponse(isAdmin, isPermissioned, prefix, command.AllCommands, sett)
		bot.GalactusClient.SendChannelMessageEmbed(m.ChannelID, &embed)
		// delete the user's message
		bot.GalactusClient.DeleteChannelMessage(m.ChannelID, m.ID)
	} else {
		args := strings.Split(contents, " ")

		for i, v := range args {
			args[i] = strings.ToLower(v)
		}

		bot.HandleCommand(isAdmin, isPermissioned, sett, g, m, args)
	}
}

func (bot *Bot) handleReactionGameStartAdd(m discordgo.MessageReactionAdd) {

	g, err := bot.GalactusClient.GetGuild(m.GuildID)
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
		if dgs.IsReactionTo(&m) {
			idMatched := false
			if m.Emoji.Name == "▶️" {
				go removeReaction(bot.GalactusClient, m.ChannelID, m.MessageID, m.Emoji.Name, m.UserID)
				go removeReaction(bot.GalactusClient, m.ChannelID, m.MessageID, m.Emoji.Name, "@me")
				go dgs.AddAllReactions(bot.GalactusClient, bot.StatusEmojis[true])
			} else {
				for color, e := range bot.StatusEmojis[true] {
					if e.ID == m.Emoji.ID {
						idMatched = true
						log.Print(fmt.Sprintf("Player %s reacted with color %s\n", m.UserID, game.GetColorStringForInt(color)))
						// the User doesn't exist in our userdata cache; add them
						user, added := dgs.checkCacheAndAddUser(g, bot.GalactusClient, m.UserID)
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
						go bot.GalactusClient.RemoveReaction(m.ChannelID, m.MessageID, e.FormatForReaction(), m.UserID)
						break
					}
				}
				if !idMatched {
					// log.Println(m.Emoji.Name)
					if m.Emoji.Name == "❌" {
						log.Println("Removing player " + m.UserID)
						dgs.ClearPlayerData(m.UserID)
						// then remove the player's reaction if we matched, or if we didn't
						go bot.GalactusClient.RemoveReaction(m.ChannelID, m.MessageID, "❌", m.UserID)
						idMatched = true
					}
				}
				// make sure to update any voice changes if they occurred
				if idMatched {
					bot.handleTrackedMembers(bot.GalactusClient, sett, 0, NoPriority, gsr)
					dgs.Edit(bot.GalactusClient, bot.gameStateResponse(dgs, sett))
				}
			}
		}
		bot.RedisInterface.SetDiscordGameState(dgs, lock)
	}
}

// voiceStateChange handles more edge-case behavior for users moving between voice channels, and catches when
// relevant discord api requests are fully applied successfully. Otherwise, we can issue multiple requests for
// the same mute/unmute, erroneously
func (bot *Bot) handleVoiceStateChange(m discordgo.VoiceStateUpdate) {
	sett := bot.StorageInterface.GetGuildSettings(m.GuildID)
	gsr := GameStateRequest{
		GuildID:      m.GuildID,
		VoiceChannel: m.ChannelID,
	}

	stateLock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	if stateLock == nil {
		return
	}
	defer stateLock.Unlock()

	var voiceLock *redsync.Mutex
	var err error
	if dgs.ConnectCode != "" {
		voiceLock, err = bot.RedisInterface.LockVoiceChanges(dgs.ConnectCode)
		if voiceLock == nil {
			return
		}
		defer voiceLock.Unlock()
	}

	g, err := bot.GalactusClient.GetGuild(dgs.GuildID)

	if err != nil || g == nil {
		return
	}

	// fetch the userData from our userData data cache
	userData, err := dgs.GetUser(m.UserID)
	if err != nil {
		// the User doesn't exist in our userdata cache; add them
		userData, _ = dgs.checkCacheAndAddUser(g, bot.GalactusClient, m.UserID)
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
			premTier := premium.FreeTier
			premiumRecord, err := bot.GalactusClient.GetGuildPremium(m.GuildID)
			if err == nil && !premium.IsExpired(premiumRecord.Tier, premiumRecord.Days) {
				premTier = premiumRecord.Tier
			}

			uid, _ := strconv.ParseUint(m.UserID, 10, 64)
			req := discord.UserModifyRequest{
				Premium: premTier,
				Users: []discord.UserModify{
					{
						UserID: uid,
						Mute:   mute,
						Deaf:   deaf,
					},
				},
			}
			mdsc := bot.GalactusClient.ModifyUsers(m.GuildID, dgs.ConnectCode, req)
			if mdsc == nil {
				log.Println("Nil response from modifyUsers, probably not good...")
			}
		}
	}
	bot.RedisInterface.SetDiscordGameState(dgs, stateLock)
}

func (bot *Bot) handleNewGameMessage(galactus *galactus_client.GalactusClient, m discordgo.MessageCreate, g *discordgo.Guild, sett *settings.GuildSettings) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(GameStateRequest{
		GuildID:     m.GuildID,
		TextChannel: m.ChannelID,
	})
	retries := 0
	for lock == nil {
		if retries > 10 {
			log.Println("DEADLOCK in obtaining game state lock, upon calling new")
			galactus.SendChannelMessage(m.ChannelID, "I wasn't able to make a new game, maybe try in a different text channel?")
			return
		}
		retries++
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(GameStateRequest{
			GuildID:     m.GuildID,
			TextChannel: m.ChannelID,
		})
	}

	channels, err := galactus.GetGuildChannels(m.GuildID)
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
		go galactus.SendAndDeleteMessage(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
			ID:    "message_handlers.handleNewGameMessage.noChannel",
			Other: "{{.User}}, please join a voice channel before starting a match!",
		}, map[string]interface{}{
			"User": mentionByUserID(m.Author.ID),
		}), time.Second*5)
		return
	}

	// allow people with a previous game going to be able to make new games
	if dgs.GameStateMsg.MessageID != "" {
		bot.forceEndGameWithState(dgs)

		dgs.Reset()
	} else {
		premTier := premium.FreeTier
		premiumRecord, err := bot.GalactusClient.GetGuildPremium(m.GuildID)
		if err == nil && !premium.IsExpired(premiumRecord.Tier, premiumRecord.Days) {
			premTier = premiumRecord.Tier
		}
		// Premium users should always be allowed to start new games; only check the free guilds
		if premTier == premium.FreeTier {
			activeGames := rediskey.GetActiveGames(context.Background(), bot.RedisInterface.client, GameTimeoutSeconds)
			act := os.Getenv("MAX_ACTIVE_GAMES")
			num, err := strconv.ParseInt(act, 10, 64)
			if err != nil {
				num = DefaultMaxActiveGames
			}
			if activeGames > num {
				go galactus.SendAndDeleteMessage(m.ChannelID, sett.LocalizeMessage(&i18n.Message{
					ID: "message_handlers.handleNewGameMessage.lockout",
					Other: "If I start any more games, Discord will lock me out, or throttle the games I'm running! 😦\n" +
						"Please try again in a few minutes, or consider AutoMuteUs Premium (`{{.CommandPrefix}} premium`)\n" +
						"Current Games: {{.Games}}",
				}, map[string]interface{}{
					"CommandPrefix": sett.CommandPrefix,
					"Games":         fmt.Sprintf("%d/%d", activeGames, num),
				}), time.Second*10)
				lock.Unlock()
				return
			}
		}
	}
	lock.Extend()

	connectCode := generateConnectCode(m.GuildID)

	dgs.ConnectCode = connectCode

	bot.RedisInterface.RefreshActiveGame(m.GuildID, connectCode)

	go bot.SubscribeToGameByConnectCode(m.GuildID, connectCode)

	dgs.Subscribed = true

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

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
	bot.logger.Info("generated URL for connection",
		zap.String("URL", hyperlink),
	)

	sendMessageDM(bot.GalactusClient, m.Author.ID, &embed)

	bot.handleGameStartMessage(galactus, m, sett, tracking, g, connectCode)
}

func (bot *Bot) handleGameStartMessage(galactus *galactus_client.GalactusClient, m discordgo.MessageCreate, sett *settings.GuildSettings, channel TrackingChannel, g *discordgo.Guild, connCode string) {
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

	dgs.clearGameTracking(bot.GalactusClient)

	dgs.Running = true

	if channel.ChannelName != "" {
		dgs.Tracking = TrackingChannel{
			ChannelID:   channel.ChannelID,
			ChannelName: channel.ChannelName,
		}
		for _, v := range g.VoiceStates {
			if v.ChannelID == channel.ChannelID {
				dgs.checkCacheAndAddUser(g, galactus, v.UserID)
			}
		}
	}

	dgs.CreateMessage(galactus, bot.gameStateResponse(dgs, sett), m.ChannelID, m.Author.ID)

	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	go dgs.AddAllReactions(bot.GalactusClient, bot.StatusEmojis[true])
}
