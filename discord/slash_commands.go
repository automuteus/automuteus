package discord

import (
	redis_common "github.com/automuteus/automuteus/common"
	"github.com/automuteus/automuteus/discord/command"
	"github.com/automuteus/utils/pkg/discord"
	"github.com/automuteus/utils/pkg/premium"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
	"strings"
)

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
	defer interactionLock.Release(ctx)

	sett := bot.StorageInterface.GetGuildSettings(i.GuildID)
	// common gsr, but not necessarily used by all commands
	gsr := GameStateRequest{
		GuildID:     i.GuildID,
		TextChannel: i.ChannelID,
	}

	// TODO respond properly for commands that *can* be performed in DMs. Such as minimal stats queries, help, info, etc
	// NOTE: difference between i.Member.User (Server/Guild chat) vs i.User (DMs)
	if gsr.GuildID == "" || i.Member == nil || i.Member.User == nil {
		return command.DmResponse(sett)
	}

	switch i.ApplicationCommandData().Name {
	case "help":
		return command.HelpResponse(sett, i.ApplicationCommandData().Options)

	case "info":
		botInfo := bot.getInfo()
		return command.InfoResponse(botInfo, i.GuildID, sett)

	case "link":
		userID, colorOrName := command.GetLinkParams(s, i.ApplicationCommandData().Options)

		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		if lock == nil {
			log.Printf("No lock could be obtained when linking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
			// TODO more gracefully respond
			return nil
		}

		status, err := bot.linkPlayer(dgs, userID, colorOrName)
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
		return command.LinkResponse(status, userID, colorOrName, sett)

	case "unlink":
		userID := command.GetUnlinkParams(s, i.ApplicationCommandData().Options)

		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		if lock == nil {
			log.Printf("No lock could be obtained when unlinking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
			// TODO more gracefully respond
			return nil
		}

		status, err := bot.unlinkPlayer(dgs, userID)
		if err != nil {
			log.Println(err)
		}
		if status == command.UnlinkSuccess {
			bot.RedisInterface.SetDiscordGameState(dgs, lock)
			dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
		} else {
			// release the lock
			bot.RedisInterface.SetDiscordGameState(nil, lock)
		}
		return command.UnlinkResponse(status, userID, sett)

	case "new":
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		if lock == nil {
			// TODO use retries like original new command
			log.Printf("No lock could be obtained when making a new game for guild %s, channel %s\n", i.GuildID, i.ChannelID)
			// TODO more gracefully respond
			return nil
		}
		g, err := s.State.Guild(i.GuildID)
		if err != nil {
			log.Println(err)
			// TODO more gracefully respond
			return nil
		}

		voiceChannelID := bot.getTrackingChannel(g, i.Member.User.ID)

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
	case "refresh":
		bot.RefreshGameStateMessage(gsr, sett)
		// TODO inform the user of how successful this command was
		return command.RefreshResponse(sett)

	case "pause":
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		if lock == nil {
			// TODO use retries or report status
			log.Printf("No lock could be obtained when pausing game for guild %s, channel %s\n", i.GuildID, i.ChannelID)
			// TODO more gracefully respond
			return nil
		}
		dgs.Running = !dgs.Running

		bot.RedisInterface.SetDiscordGameState(dgs, lock)
		// if we paused the game, unmute/undeafen all players
		if !dgs.Running {
			bot.applyToAll(dgs, false, false)
		}

		dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
		// TODO inform the user of how successful this command was
		return command.PauseResponse(sett)

	case "end":
		dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
		if v, ok := bot.EndGameChannels[dgs.ConnectCode]; ok {
			v <- true
		}
		delete(bot.EndGameChannels, dgs.ConnectCode)

		bot.applyToAll(dgs, false, false)

		// TODO inform the user of how successful this command was
		return command.EndResponse(sett)

	case "privacy":
		privArg := command.GetPrivacyParam(i.ApplicationCommandData().Options)
		switch privArg {
		case command.PrivacyUnknown:
			fallthrough
		case command.PrivacyInfo:
			return command.PrivacyResponse(privArg, nil, nil, nil, sett)

		case command.PrivacyOptOut:
			err := bot.RedisInterface.DeleteLinksByUserID(i.GuildID, i.Member.User.ID)
			if err != nil {
				// we send the cache clear type here because that's exactly what we failed to do; clear the cache
				return command.PrivacyResponse(command.PrivacyCacheClear, nil, nil, err, sett)
			}
			fallthrough
		case command.PrivacyOptIn:
			_, err := bot.PostgresInterface.OptUserByString(i.Member.User.ID, privArg == command.PrivacyOptIn)
			return command.PrivacyResponse(privArg, nil, nil, err, sett)

		case command.PrivacyShowMe:
			cached := bot.RedisInterface.GetUsernameOrUserIDMappings(i.GuildID, i.Member.User.ID)
			user, err := bot.PostgresInterface.GetUserByString(i.Member.User.ID)
			return command.PrivacyResponse(privArg, cached, user, err, sett)

		case command.PrivacyCacheClear:
			err := bot.RedisInterface.DeleteLinksByUserID(i.GuildID, i.Member.User.ID)
			return command.PrivacyResponse(privArg, nil, nil, err, sett)
		}

	case "map":
		mapType, detailed := command.GetMapParams(i.ApplicationCommandData().Options)
		return command.MapResponse(mapType, detailed)

	case "stats":
		statsOperation, statsType, id := command.GetStatsParams(bot.PrimarySession, i.GuildID, i.ApplicationCommandData().Options)
		if statsOperation == command.View {
			var embed *discordgo.MessageEmbed
			switch statsType {
			case command.UserStats:
				// TODO substitute premium status
				embed = bot.UserStatsEmbed(id, i.GuildID, sett, true)
			case command.GuildStats:
				// TODO substitute premium status
				embed = bot.GuildStatsEmbed(i.GuildID, sett, true)
			case command.MatchStats:
				tokens := strings.Split(id, ":")
				if len(tokens) > 1 {
					embed = bot.GameStatsEmbed(i.GuildID, tokens[1], tokens[0], sett)
				} else {
					// TODO report invalid match code error to user (private slash response)
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
			// TODO restrict clear to admins/self-users only
		} else {
			var content string
			switch statsType {
			case command.UserStats:
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

			case command.GuildStats:
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

	case "premium":
		premArg := command.GetPremiumParams(i.ApplicationCommandData().Options)
		premStatus, days := bot.PostgresInterface.GetGuildPremiumStatus(i.GuildID)
		if premium.IsExpired(premStatus, days) {
			premStatus = premium.FreeTier
		}
		// TODO restrict invite viewage to Admins only
		return command.PremiumResponse(i.GuildID, premStatus, days, premArg, sett)
	}
	// no command matched somehow
	return nil
}
