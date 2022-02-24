package discord

import (
	"github.com/automuteus/automuteus/discord/command"
	"github.com/bwmarrin/discordgo"
	"log"
)

func (bot *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// lock this particular interaction message so no other shard tries to process it
	interactionLock := bot.RedisInterface.LockSnowflake(i.ID)
	// couldn't obtain lock; bail bail bail!
	if interactionLock == nil {
		return
	}
	defer interactionLock.Release(ctx)

	sett := bot.StorageInterface.GetGuildSettings(i.GuildID)
	// common gsr, but not necessarily used by all commands
	gsr := GameStateRequest{
		GuildID:     i.GuildID,
		TextChannel: i.ChannelID,
	}

	var response *discordgo.InteractionResponse

	// TODO respond properly for commands that *can* be performed in DMs. Such as minimal stats queries, help, info, etc
	// NOTE: difference between i.Member.User (Server/Guild chat) vs i.User (DMs)
	if gsr.GuildID == "" {
		response = command.DmResponse(sett)
		err := s.InteractionRespond(i.Interaction, response)
		if err != nil {
			log.Println(err)
		}
		return
	}

	switch i.ApplicationCommandData().Name {
	case "help":
		response = command.HelpResponse(sett, i.ApplicationCommandData().Options)

	case "info":
		botInfo := bot.getInfo()
		response = command.InfoResponse(botInfo, i.GuildID, sett)

	case "link":
		userID, colorOrName := command.GetLinkParams(s, i.ApplicationCommandData().Options)

		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		if lock == nil {
			log.Printf("No lock could be obtained when linking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
			// TODO more gracefully respond
			return
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
		response = command.LinkResponse(status, userID, colorOrName, sett)

	case "unlink":
		userID := command.GetUnlinkParams(s, i.ApplicationCommandData().Options)

		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		if lock == nil {
			log.Printf("No lock could be obtained when unlinking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
			return
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
		response = command.UnlinkResponse(status, userID, sett)

	case "new":
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		if lock == nil {
			// TODO use retries like original new command
			log.Printf("No lock could be obtained when making a new game for guild %s, channel %s\n", i.GuildID, i.ChannelID)
			return
		}
		g, err := s.State.Guild(i.GuildID)
		if err != nil {
			log.Println(err)
			return
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
			response = command.NewResponse(status, command.NewInfo{
				Hyperlink:   hyperlink,
				MinimalURL:  minimalURL,
				ConnectCode: dgs.ConnectCode,
				ActiveGames: activeGames, // not actually needed for Success messages
			}, sett)

			bot.handleGameStartMessage(i.GuildID, i.ChannelID, voiceChannelID, i.Member.User.ID, sett, g, dgs.ConnectCode)
		} else {
			// release the lock
			bot.RedisInterface.SetDiscordGameState(nil, lock)
			response = command.NewResponse(status, command.NewInfo{
				ActiveGames: activeGames, // only field we need for success messages
			}, sett)
		}
	case "refresh":
		bot.RefreshGameStateMessage(gsr, sett)
		// TODO inform the user of how successful this command was
		response = command.RefreshResponse(sett)

	case "pause":
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
		if lock == nil {
			// TODO use retries or report status
			log.Printf("No lock could be obtained when pausing game for guild %s, channel %s\n", i.GuildID, i.ChannelID)
			return
		}
		dgs.Running = !dgs.Running

		bot.RedisInterface.SetDiscordGameState(dgs, lock)
		// if we paused the game, unmute/undeafen all players
		if !dgs.Running {
			bot.applyToAll(dgs, false, false)
		}

		dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
		// TODO inform the user of how successful this command was
		response = command.PauseResponse(sett)

	case "end":
		dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
		if v, ok := bot.EndGameChannels[dgs.ConnectCode]; ok {
			v <- true
		}
		delete(bot.EndGameChannels, dgs.ConnectCode)

		bot.applyToAll(dgs, false, false)

		// TODO inform the user of how successful this command was
		response = command.EndResponse(sett)

	case "privacy":
		privArg := command.GetPrivacyParam(i.ApplicationCommandData().Options)
		switch privArg {
		case command.PrivacyUnknown:
			fallthrough
		case command.PrivacyInfo:
			response = command.PrivacyResponse(privArg, nil, nil, nil, sett)

		case command.PrivacyOptOut:
			err := bot.RedisInterface.DeleteLinksByUserID(i.GuildID, i.Member.User.ID)
			if err != nil {
				// we send the cache clear type here because that's exactly what we failed to do; clear the cache
				response = command.PrivacyResponse(command.PrivacyCacheClear, nil, nil, err, sett)
				// don't fall-through; exit here
				break
			}
			fallthrough
		case command.PrivacyOptIn:
			_, err := bot.PostgresInterface.OptUserByString(i.Member.User.ID, privArg == command.PrivacyOptIn)
			response = command.PrivacyResponse(privArg, nil, nil, err, sett)

		case command.PrivacyShowMe:
			cached := bot.RedisInterface.GetUsernameOrUserIDMappings(i.GuildID, i.Member.User.ID)
			user, err := bot.PostgresInterface.GetUserByString(i.Member.User.ID)
			response = command.PrivacyResponse(privArg, cached, user, err, sett)

		case command.PrivacyCacheClear:
			err := bot.RedisInterface.DeleteLinksByUserID(i.GuildID, i.Member.User.ID)
			response = command.PrivacyResponse(privArg, nil, nil, err, sett)
		}

	case "map":
		mapType, detailed := command.GetMapParams(i.ApplicationCommandData().Options)
		response = command.MapResponse(mapType, detailed, sett)

	}
	if response != nil {
		err := s.InteractionRespond(i.Interaction, response)
		if err != nil {
			log.Println(err)
		}
	}
}
