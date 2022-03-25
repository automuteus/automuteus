package discord

import (
	"encoding/json"
	"fmt"
	redis_common "github.com/automuteus/automuteus/common"
	"github.com/automuteus/automuteus/discord/command"
	"github.com/automuteus/automuteus/discord/setting"
	"github.com/automuteus/automuteus/metrics"
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/premium"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/bsm/redislock"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
	"regexp"
	"strings"
)

var MatchIDRegex = regexp.MustCompile(`^[A-Z0-9]{8}:[0-9]+$`)

func (bot *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	response := bot.slashCommandHandler(s, i)
	if response != nil {
		err := s.InteractionRespond(i.Interaction, response)
		if err != nil {
			log.Println(err)
		}
	}
}

func (bot *Bot) slashCommandHandler(s *discordgo.Session, i *discordgo.InteractionCreate) *discordgo.InteractionResponse {
	if i.Member != nil && i.Member.User != nil {
		if redis_common.IsUserBanned(bot.RedisInterface.client, i.Member.User.ID) {
			return nil
		}
	}

	// lock this particular interaction message so no other shard tries to process it
	interactionLock := bot.RedisInterface.LockSnowflake(i.ID)
	// couldn't obtain lock; bail bail bail!
	if interactionLock == nil {
		return nil
	}
	defer metrics.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, 1)
	defer interactionLock.Release(ctx)

	sett := bot.StorageInterface.GetGuildSettings(i.GuildID)

	// TODO respond properly for commands that *can* be performed in DMs. Such as minimal stats queries, help, info, etc
	// NOTE: difference between i.Member.User (Server/Guild chat) vs i.User (DMs)
	if i.GuildID == "" || i.Member == nil || i.Member.User == nil {
		return command.DmResponse(sett)
	}

	if redis_common.IsUserRateLimitedGeneral(bot.RedisInterface.client, i.Member.User.ID) {
		banned := redis_common.IncrementRateLimitExceed(bot.RedisInterface.client, i.Member.User.ID)
		if banned {
			return &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: 1 << 6, //private message
					Content: sett.LocalizeMessage(&i18n.Message{
						ID:    "softban.ignoring",
						Other: "I'm ignoring you for the next 5 minutes, stop spamming",
					}),
				},
			}
		} else {
			return &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: 1 << 6, //private message
					Content: sett.LocalizeMessage(&i18n.Message{
						ID:    "softban.warning",
						Other: "Please stop spamming commands",
					}),
				},
			}
		}
	}
	redis_common.MarkUserRateLimit(bot.RedisInterface.client, i.Member.User.ID, "", 0)

	g, err := s.State.Guild(i.GuildID)
	if err != nil {
		log.Println(err)
		return command.PrivateErrorResponse("get-guild", err, sett)
	}

	isAdmin, isPermissioned := false, false
	if g.OwnerID == i.Member.User.ID || (len(sett.AdminUserIDs) == 0 && len(sett.PermissionRoleIDs) == 0) {
		// the guild owner should always have both permissions
		// or if both permissions are still empty, everyone gets both
		isAdmin = true
		isPermissioned = true
	} else {
		// if we have no admins, then we MUST have mods as per the check above. So ensure this user is a mod
		if len(sett.AdminUserIDs) == 0 {
			isAdmin = sett.HasRolePerms(i.Member)
		} else {
			// we have admins; make sure user is one
			isAdmin = sett.HasAdminPerms(i.Member.User)
		}
		// even if we have admins, we can grant mod if the moderators role is empty; it is lesser permissions
		isPermissioned = len(sett.PermissionRoleIDs) == 0 || sett.HasRolePerms(i.Member)
	}

	// common gsr, but not necessarily used by all commands
	gsr := GameStateRequest{
		GuildID:     i.GuildID,
		TextChannel: i.ChannelID,
	}

	if i.Type == discordgo.InteractionApplicationCommand {
		switch i.ApplicationCommandData().Name {
		case command.Help.Name:
			return command.HelpResponse(sett, i.ApplicationCommandData().Options)

		case command.Info.Name:
			botInfo := bot.getInfo()
			return command.InfoResponse(botInfo, i.GuildID, sett)

		case command.Link.Name:
			if !isPermissioned {
				return command.InsufficientPermissionsResponse(sett)
			}
			userID, color := command.GetLinkParams(s, i.ApplicationCommandData().Options)

			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLockRetries(gsr, 5)
			if lock == nil {
				log.Printf("No lock could be obtained when linking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
				return command.DeadlockGameStateResponse(command.Link.Name, sett)
			}
			return bot.linkOrUnlinkAndRespond(dgs, lock, userID, color, sett)

		case command.Unlink.Name:
			if !isPermissioned {
				return command.InsufficientPermissionsResponse(sett)
			}
			userID := command.GetUnlinkParams(s, i.ApplicationCommandData().Options)

			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
			if lock == nil {
				log.Printf("No lock could be obtained when unlinking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
				return command.DeadlockGameStateResponse(command.Unlink.Name, sett)
			}
			return bot.linkOrUnlinkAndRespond(dgs, lock, userID, "", sett)

		case command.Settings.Name:
			if !isAdmin {
				return command.InsufficientPermissionsResponse(sett)
			}
			premStatus, days := bot.PostgresInterface.GetGuildPremiumStatus(i.GuildID)
			isPrem := !premium.IsExpired(premStatus, days)
			setting, args := command.GetSettingsParams(s, i.ApplicationCommandData().Options)
			msg := bot.HandleSettingsCommand(i.GuildID, sett, setting, args, isPrem)
			return command.SettingsResponse(msg)

		case command.New.Name:
			if !isPermissioned {
				return command.InsufficientPermissionsResponse(sett)
			}
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLockRetries(gsr, 5)
			if lock == nil {
				log.Printf("No lock could be obtained when making a new game for guild %s, channel %s\n", i.GuildID, i.ChannelID)
				return command.DeadlockGameStateResponse(command.New.Name, sett)
			}

			voiceChannelID := getTrackingChannel(g, i.Member.User.ID)

			status, activeGames := bot.newGame(dgs, voiceChannelID)
			if status == command.NewSuccess {
				// release the lock
				bot.RedisInterface.SetDiscordGameState(dgs, lock)

				bot.RedisInterface.RefreshActiveGame(dgs.GuildID, dgs.ConnectCode)

				killChan := make(chan EndGameMessage)

				go bot.SubscribeToGameByConnectCode(i.GuildID, dgs.ConnectCode, killChan)

				bot.ChannelsMapLock.Lock()
				bot.EndGameChannels[dgs.ConnectCode] = killChan
				bot.ChannelsMapLock.Unlock()

				hyperlink, minimalURL := formCaptureURL(bot.url, dgs.ConnectCode)

				bot.handleGameStartMessage(i.GuildID, i.ChannelID, voiceChannelID, i.Member.User.ID, sett, g, dgs.ConnectCode)

				return command.NewResponse(status, command.NewInfo{
					Hyperlink:   hyperlink,
					MinimalURL:  minimalURL,
					ConnectCode: dgs.ConnectCode,
					ActiveGames: activeGames, // not actually needed for Success messages
				}, sett)
			} else {
				// release the lock
				bot.RedisInterface.SetDiscordGameState(nil, lock)
				return command.NewResponse(status, command.NewInfo{
					ActiveGames: activeGames, // only field we need for success messages
				}, sett)
			}
		case command.Refresh.Name:
			if bot.RefreshGameStateMessage(gsr, sett) {
				return command.PrivateResponse(ThumbsUp)
			} else {
				return command.NoGameResponse(sett)
			}

		case command.Pause.Name:
			if !isPermissioned {
				return command.InsufficientPermissionsResponse(sett)
			}
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLockRetries(gsr, 5)
			if lock == nil {
				log.Printf("No lock could be obtained when pausing game for guild %s, channel %s\n", i.GuildID, i.ChannelID)
				return command.DeadlockGameStateResponse(command.Pause.Name, sett)
			}
			if !dgs.GameStateMsg.Exists() {
				bot.RedisInterface.SetDiscordGameState(nil, lock)
				return command.NoGameResponse(sett)
			}

			dgs.Running = !dgs.Running

			bot.RedisInterface.SetDiscordGameState(dgs, lock)
			// if we paused the game, unmute/undeafen all players
			if !dgs.Running {
				err = bot.applyToAll(dgs, false, false)
			}
			dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
			if err != nil {
				return command.PrivateErrorResponse(command.Pause.Name, err, sett)
			}
			return command.PrivateResponse(ThumbsUp)

		case command.End.Name:
			if !isPermissioned {
				return command.InsufficientPermissionsResponse(sett)
			}
			dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
			if dgs != nil {
				if !dgs.GameStateMsg.Exists() {
					return command.NoGameResponse(sett)
				}

				if v, ok := bot.EndGameChannels[dgs.ConnectCode]; ok {
					v <- true
				}
				delete(bot.EndGameChannels, dgs.ConnectCode)

				err = bot.applyToAll(dgs, false, false)
				if err != nil {
					return command.PrivateErrorResponse(command.End.Name, err, sett)
				}
				return command.PrivateResponse(ThumbsUp)
			}
			return command.DeadlockGameStateResponse(command.End.Name, sett)

		case command.Privacy.Name:
			privArg := command.GetPrivacyParam(i.ApplicationCommandData().Options)
			switch privArg {
			case command.PrivacyInfo:
				return command.PrivacyResponse(privArg, nil, nil, nil, sett)

			case command.PrivacyOptOut:
				err = bot.RedisInterface.DeleteLinksByUserID(i.GuildID, i.Member.User.ID)
				if err != nil {
					return command.PrivacyResponse(privArg, nil, nil, err, sett)
				}
				fallthrough
			case command.PrivacyOptIn:
				err = bot.PostgresInterface.OptUserByString(i.Member.User.ID, privArg == command.PrivacyOptIn)
				return command.PrivacyResponse(privArg, nil, nil, err, sett)

			case command.PrivacyShowMe:
				cached, _ := bot.RedisInterface.GetUsernameOrUserIDMappings(i.GuildID, i.Member.User.ID)
				user, err := bot.PostgresInterface.GetUserByString(i.Member.User.ID)
				return command.PrivacyResponse(privArg, cached, user, err, sett)
			}

		case command.Map.Name:
			mapType, detailed := command.GetMapParams(i.ApplicationCommandData().Options)
			return command.MapResponse(mapType, detailed)

		case command.Stats.Name:
			action, opType, id := command.GetStatsParams(bot.PrimarySession, i.GuildID, i.ApplicationCommandData().Options)
			prem := true
			if premium.IsExpired(bot.PostgresInterface.GetGuildPremiumStatus(i.GuildID)) {
				prem = false
			}
			if action == setting.View {
				var embed *discordgo.MessageEmbed
				switch opType {
				case command.User:
					embed = bot.UserStatsEmbed(id, i.GuildID, sett, prem)
				case command.Guild:
					embed = bot.GuildStatsEmbed(i.GuildID, sett, prem)
				case command.Match:
					if MatchIDRegex.Match([]byte(id)) {
						tokens := strings.Split(id, ":")
						embed = bot.GameStatsEmbed(i.GuildID, tokens[1], tokens[0], prem, sett)
					} else {
						err := fmt.Errorf("invalid match code provided: %s, should resemble something like `1A2B3C4D:12345`", id)
						return command.PrivateErrorResponse(command.Stats.Name+" "+command.Match, err, sett)
					}
				}
				if embed != nil {
					return &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Embeds: []*discordgo.MessageEmbed{
								embed,
							},
						},
					}
				}
			} else if action == setting.Clear {
				// id mismatch applies to user ids AND guild ID (guildId *always* != author.id, therefore, must be admin)
				if id != i.Member.User.ID && !isAdmin {
					return command.InsufficientPermissionsResponse(sett)
				}
				var content string
				switch opType {
				case command.User:
					err := bot.PostgresInterface.DeleteAllGamesForUser(id)
					if err != nil {
						content = sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.stats.user.reset.error",
							Other: "Encountered an error resetting the stats for {{.User}}: {{.Error}}",
						},
							map[string]interface{}{
								"User":  discord.MentionByUserID(id),
								"Error": err.Error(),
							})
					} else {
						content = sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.stats.user.reset.success",
							Other: "Successfully reset the stats for {{.User}}!",
						},
							map[string]interface{}{
								"User": discord.MentionByUserID(id),
							})
					}

				case command.Guild:
					err := bot.PostgresInterface.DeleteAllGamesForServer(id)
					if err != nil {
						content = sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.stats.guild.reset.error",
							Other: "Encountered an error resetting the stats for this guild: {{.Error}}",
						},
							map[string]interface{}{
								"Error": err.Error(),
							})
					} else {
						content = sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.stats.guild.reset.success",
							Other: "Successfully reset this guild's stats!",
						})
					}
				}
				return &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6, //private message
						Content: content,
					},
				}
				// TODO handle clear confirmation (message followups?)
			}

		case command.Premium.Name:
			premArg := command.GetPremiumParams(i.ApplicationCommandData().Options)
			premStatus, days := bot.PostgresInterface.GetGuildPremiumStatus(i.GuildID)
			if premium.IsExpired(premStatus, days) {
				premStatus = premium.FreeTier
			}
			return command.PremiumResponse(i.GuildID, premStatus, days, premArg, isAdmin, sett)

		case command.Debug.Name:
			action, opType, id := command.GetDebugParams(bot.PrimarySession, i.Member.User.ID, i.ApplicationCommandData().Options)
			if action == setting.View {
				if opType == command.User {
					cached, err := bot.RedisInterface.GetUsernameOrUserIDMappings(i.GuildID, id)
					log.Println("View user cache")
					return command.DebugResponse(setting.View, cached, nil, id, err, sett)
				} else if opType == command.GameState {
					state := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
					if state != nil {
						jBytes, err := json.MarshalIndent(state, "", "  ")
						return command.DebugResponse(setting.View, nil, jBytes, id, err, sett)
					} else {
						return command.DeadlockGameStateResponse(command.Debug.Name, sett)
					}
				}
			} else if action == setting.Clear {
				if opType == command.User {
					if id != i.Member.User.ID {
						if !isAdmin {
							return command.InsufficientPermissionsResponse(sett)
						}
					}
					err := bot.RedisInterface.DeleteLinksByUserID(i.GuildID, id)
					return command.DebugResponse(setting.Clear, nil, nil, id, err, sett)
				}
			} else if action == command.UnmuteAll {
				dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
				err = bot.applyToAll(dgs, false, false)
				if err != nil {
					return command.PrivateErrorResponse(command.UnmuteAll, err, sett)
				}
				return command.PrivateResponse(ThumbsUp)
			}
		}

	} else if i.Type == discordgo.InteractionMessageComponent {
		if i.MessageComponentData().CustomID == "select-color" {
			if len(i.MessageComponentData().Values) > 0 {
				value := i.MessageComponentData().Values[0]
				lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLockRetries(gsr, 5)
				if lock == nil {
					log.Printf("No lock could be obtained when linking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
					return command.DeadlockGameStateResponse(command.Link.Name, sett)
				}
				if value == UnlinkEmojiName {
					return bot.linkOrUnlinkAndRespond(dgs, lock, i.Member.User.ID, "", sett)
				} else {
					return bot.linkOrUnlinkAndRespond(dgs, lock, i.Member.User.ID, value, sett)
				}
			}
		}
	}

	// no command or handler matched somehow
	return nil
}

func (bot *Bot) linkOrUnlinkAndRespond(dgs *GameState, lock *redislock.Lock, userID, testValue string, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	if testValue != "" {
		// don't care if it's successful, just always unlink
		unlinkPlayer(dgs, userID)
		status, err := linkPlayer(bot.RedisInterface, dgs, userID, testValue)
		if err != nil {
			log.Println(err)
		}
		if status == command.LinkSuccess {
			bot.RedisInterface.SetDiscordGameState(dgs, lock)
			dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
		} else {
			// release the lock
			bot.RedisInterface.SetDiscordGameState(nil, lock)
		}
		return command.LinkResponse(status, userID, testValue, sett)
	} else {
		status := unlinkPlayer(dgs, userID)
		if status == command.UnlinkSuccess {
			bot.RedisInterface.SetDiscordGameState(dgs, lock)
			dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
		} else {
			// release the lock
			bot.RedisInterface.SetDiscordGameState(nil, lock)
		}
		return command.UnlinkResponse(status, userID, sett)
	}
}
