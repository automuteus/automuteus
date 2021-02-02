package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/automuteus/utils/pkg/capture"
	"github.com/automuteus/utils/pkg/game"
	"github.com/automuteus/utils/pkg/settings"
	"github.com/denverquane/amongusdiscord/amongus"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/denverquane/amongusdiscord/storage"
)

type EndGameMessage bool

func (bot *Bot) SubscribeToGameByConnectCode(guildID, connectCode string, endGameChannel chan EndGameMessage) {
	log.Println("Started Redis Subscription worker for " + connectCode)

	timer := time.NewTimer(time.Second * time.Duration(bot.captureTimeout))

	dgsRequest := GameStateRequest{
		GuildID:     guildID,
		ConnectCode: connectCode,
	}

	f := func(msg capture.Event) {
		timer.Reset(time.Second * time.Duration(bot.captureTimeout))

		log.Printf("Popped job of type %d w/ payload %s\n", msg.EventType, string(msg.Payload))
		bot.refreshGameLiveness(connectCode)
		bot.RedisInterface.RefreshActiveGame(guildID, connectCode)

		gameEvent := storage.PostgresGameEvent{
			GameID:    -1,
			UserID:    nil,
			EventTime: int32(time.Now().Unix()),
			EventType: int16(msg.EventType),
			Payload:   string(msg.Payload),
		}
		correlatedUserID := ""

		switch msg.EventType {
		case capture.Connection:
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
			for lock == nil {
				lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
			}
			if string(msg.Payload) == trueString {
				dgs.Linked = true
			} else {
				dgs.Linked = false
			}
			dgs.ConnectCode = connectCode
			bot.RedisInterface.SetDiscordGameState(dgs, lock)

			sett := bot.StorageInterface.GetGuildSettings(guildID)
			bot.handleTrackedMembers(bot.GalactusClient, sett, 0, NoPriority, dgsRequest)

			dgs.Edit(bot.GalactusClient, bot.gameStateResponse(dgs, sett))
		case capture.Lobby:
			var lobby game.Lobby
			err := json.Unmarshal(msg.Payload, &lobby)
			if err != nil {
				log.Println(err)
				break
			}

			sett := bot.StorageInterface.GetGuildSettings(guildID)
			bot.processLobby(sett, lobby, dgsRequest)
		case capture.State:
			num, err := strconv.ParseInt(string(msg.Payload), 10, 64)
			if err != nil {
				log.Println(err)
				break
			}

			bot.processTransition(game.Phase(num), dgsRequest)
		case capture.Player:
			var player game.Player
			err := json.Unmarshal(msg.Payload, &player)
			if err != nil {
				log.Println(err)
				break
			}

			sett := bot.StorageInterface.GetGuildSettings(guildID)
			shouldHandleTracked, userID := bot.processPlayer(sett, player, dgsRequest)
			if shouldHandleTracked {
				bot.handleTrackedMembers(bot.GalactusClient, sett, 0, NoPriority, dgsRequest)
			}
			correlatedUserID = userID
		case capture.GameOver:
			var gameOverResult game.Gameover
			// log.Println("Successfully identified game over event:")
			// log.Println(job.Payload)
			err := json.Unmarshal(msg.Payload, &gameOverResult)
			if err != nil {
				log.Println(err)
				break
			}

			sett := bot.StorageInterface.GetGuildSettings(guildID)

			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
			for dgs == nil {
				lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
			}
			//once we've locked the gamestate, we know for certain that it's current; then we can release the lock
			lock.Unlock()

			delTime := sett.GetDeleteGameSummaryMinutes()
			if delTime != 0 {
				winners := getWinners(*dgs, gameOverResult)
				buf := bytes.NewBuffer([]byte{})
				for i, v := range winners {
					roleStr := "Crewmate"
					if v.role == game.ImposterRole {
						roleStr = "Imposter"
					}
					buf.WriteString(fmt.Sprintf("<@%s>", v.userID))
					if i < len(winners)-1 {
						buf.WriteRune(',')
					} else {
						buf.WriteString(fmt.Sprintf(" won as %s", roleStr))
					}
				}
				embed := gameOverMessage(dgs, bot.StatusEmojis, sett, buf.String())
				channelID := dgs.GameStateMsg.MessageChannelID
				if sett.GetMatchSummaryChannelID() != "" {
					channelID = sett.GetMatchSummaryChannelID()
				}
				msg, err := bot.GalactusClient.SendChannelMessageEmbed(channelID, embed)
				if delTime > 0 && err == nil {
					go MessageDeleteWorker(bot.GalactusClient, msg.ChannelID, msg.ID, time.Minute*time.Duration(delTime))
				}
			}
			go dumpGameToPostgres(*dgs, bot.PostgresInterface, gameOverResult)

			// refresh the game message if the setting is marked (it is not locked, the previous dgs is
			// read-only). This means the original msg is refreshed, not the gameover message
			if sett.AutoRefresh {
				bot.RefreshGameStateMessage(dgsRequest, sett)
			}

			// now we need to fetch the state again (AFTER refreshing) to mark the game as complete/
			lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
			for lock == nil {
				lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
			}
			dgs.MatchID = -1
			dgs.MatchStartUnix = -1
			bot.RedisInterface.SetDiscordGameState(dgs, lock)
		}
		if msg.EventType != capture.Connection {
			go func(userID string, ge storage.PostgresGameEvent) {
				dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(dgsRequest)
				if dgs.MatchID > 0 && dgs.MatchStartUnix > 0 {
					ge.GameID = dgs.MatchID
					if userID != "" {
						num, err := strconv.ParseUint(userID, 10, 64)
						if err != nil {
							log.Println(err)
							ge.UserID = nil
						} else {
							ge.UserID = &num
						}
						log.Printf("Adding postgres event with user id %d\n", ge.UserID)
					}

					err := bot.PostgresInterface.AddEvent(&ge)
					if err != nil {
						log.Println(err)
					}
				}
			}(correlatedUserID, gameEvent)
		}
	}

	bot.GalactusClient.RegisterCaptureHandler(connectCode, f)

	err := bot.GalactusClient.StartCapturePolling(connectCode)
	if err != nil {
		log.Println(err)
	}

	for {
		select {
		case <-timer.C:
			timer.Stop()
			log.Printf("Killing game w/ code %s after %d seconds of inactivity!\n", connectCode, bot.captureTimeout)
			bot.GalactusClient.StopCapturePolling(connectCode)
			bot.forceEndGame(dgsRequest)
			bot.ChannelsMapLock.Lock()
			delete(bot.EndGameChannels, connectCode)
			bot.ChannelsMapLock.Unlock()

			return
		case <-endGameChannel:
			log.Println("Redis subscriber received kill signal, closing all pubsubs")
			bot.GalactusClient.StopCapturePolling(connectCode)
			bot.forceEndGame(dgsRequest)
			return
		}
	}
}

type winnerRecord struct {
	userID string
	role   game.GameRole
}

func getWinners(dgs GameState, gameOver game.Gameover) []winnerRecord {
	winners := []winnerRecord{}

	imposterWin := gameOver.GameOverReason == game.ImpostorByKill ||
		gameOver.GameOverReason == game.ImpostorByVote ||
		gameOver.GameOverReason == game.ImpostorBySabotage ||
		gameOver.GameOverReason == game.ImpostorDisconnect

	for _, player := range dgs.UserData {
		if player.GetPlayerName() != amongus.UnlinkedPlayerName {
			for _, v := range gameOver.PlayerInfos {
				// only override for the imposters
				if player.GetPlayerName() == v.Name {
					if (v.IsImpostor && imposterWin) || (!v.IsImpostor && !imposterWin) {
						role := game.CrewmateRole
						if v.IsImpostor {
							role = game.ImposterRole
						}
						winners = append(winners, winnerRecord{
							userID: player.User.UserID,
							role:   role,
						})
					}
				}
			}
		}
	}
	return winners
}

func (bot *Bot) processPlayer(sett *settings.GuildSettings, player game.Player, dgsRequest GameStateRequest) (bool, string) {
	if player.Name != "" {
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
		for lock == nil {
			lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
		}
		dgs.Linked = true

		defer bot.RedisInterface.SetDiscordGameState(dgs, lock)

		if player.Disconnected || player.Action == game.LEFT {
			if player.Disconnected {
				log.Println("I detected that " + player.Name + " disconnected, I'm purging their player data!")
				dgs.ClearPlayerDataByPlayerName(player.Name)
			}
			_, _, data := dgs.AmongUsData.UpdatePlayer(player)

			userID := dgs.AttemptPairingByMatchingNames(data)
			// try pairing via the cached usernames
			if userID == "" {
				uids := bot.RedisInterface.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)
				userID = dgs.AttemptPairingByUserIDs(data, uids)
			} else {
				bot.applyToSingle(dgs, userID, false, false)
			}

			dgs.AmongUsData.ClearPlayerData(player.Name)

			// only update the message if we're not in the tasks phase (info leaks)
			if dgs.AmongUsData.GetPhase() != game.TASKS {
				dgs.Edit(bot.GalactusClient, bot.gameStateResponse(dgs, sett))
			}

			return true, userID
		}
		updated, isAliveUpdated, data := dgs.AmongUsData.UpdatePlayer(player)
		switch {
		case player.Action == game.JOINED:
			log.Println("Detected a player joined, refreshing User data mappings")
			userID := dgs.AttemptPairingByMatchingNames(data)
			if userID == "" {
				uids := bot.RedisInterface.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)
				userID = dgs.AttemptPairingByUserIDs(data, uids)
			}
			dgs.Edit(bot.GalactusClient, bot.gameStateResponse(dgs, sett))
			return true, userID
		case updated:
			userID := dgs.AttemptPairingByMatchingNames(data)
			if userID == "" {
				uids := bot.RedisInterface.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)

				userID = dgs.AttemptPairingByUserIDs(data, uids)
			}
			if isAliveUpdated && dgs.AmongUsData.GetPhase() == game.TASKS {
				if sett.GetUnmuteDeadDuringTasks() || player.Action == game.EXILED {
					dgs.Edit(bot.GalactusClient, bot.gameStateResponse(dgs, sett))
					return true, userID
				}
				log.Println("NOT updating the discord status message; would leak info")
				return false, userID
			}
			dgs.Edit(bot.GalactusClient, bot.gameStateResponse(dgs, sett))
			if player.Action == game.EXILED {
				return false, userID // don't apply a mute to this player
			}
			return true, userID
		default:
			return false, ""
		}
	}
	return false, ""
}

func (bot *Bot) processTransition(phase game.Phase, dgsRequest GameStateRequest) {
	sett := bot.StorageInterface.GetGuildSettings(dgsRequest.GuildID)
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	for lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	}

	// on a gameover event from the capture, it's like going to the lobby; use that logic
	if phase == game.GAMEOVER {
		phase = game.LOBBY
	}

	oldPhase := dgs.AmongUsData.UpdatePhase(phase)
	if oldPhase == phase {
		lock.Unlock()
		return
	}
	dgs.Linked = true
	// if we started a new game
	if oldPhase == game.LOBBY && phase == game.TASKS {
		matchStart := time.Now().Unix()
		dgs.MatchStartUnix = matchStart
		gameID := startGameInPostgres(*dgs, bot.PostgresInterface)
		dgs.MatchID = int64(gameID)
		log.Printf("New match has begun. ID %d and starttime %d\n", gameID, matchStart)
	}

	bot.RedisInterface.SetDiscordGameState(dgs, lock)
	switch phase {
	case game.MENU:
		dgs.Edit(bot.GalactusClient, bot.gameStateResponse(dgs, sett))
		bot.applyToAll(dgs, false, false)
	case game.LOBBY:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(bot.GalactusClient, sett, delay, NoPriority, dgsRequest)

		dgs.Edit(bot.GalactusClient, bot.gameStateResponse(dgs, sett))

	case game.TASKS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		// when going from discussion to tasks, we should mute alive players FIRST
		priority := AlivePriority
		if oldPhase == game.LOBBY {
			priority = NoPriority
		}

		bot.handleTrackedMembers(bot.GalactusClient, sett, delay, priority, dgsRequest)
		dgs.Edit(bot.GalactusClient, bot.gameStateResponse(dgs, sett))

	case game.DISCUSS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(bot.GalactusClient, sett, delay, DeadPriority, dgsRequest)

		if sett.AutoRefresh {
			bot.RefreshGameStateMessage(dgsRequest, sett)
		} else {
			dgs.Edit(bot.GalactusClient, bot.gameStateResponse(dgs, sett))
		}
	}
}

func (bot *Bot) processLobby(sett *settings.GuildSettings, lobby game.Lobby, dgsRequest GameStateRequest) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	for lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	}

	dgs.AmongUsData.SetRoomRegionMap(lobby.LobbyCode, lobby.Region.ToString(), lobby.PlayMap)
	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	dgs.Edit(bot.GalactusClient, bot.gameStateResponse(dgs, sett))
}

func startGameInPostgres(dgs GameState, psql *storage.PsqlInterface) uint64 {
	if dgs.MatchStartUnix < 0 {
		return 0
	}
	gid, err := strconv.ParseUint(dgs.GuildID, 10, 64)
	if err != nil {
		log.Println(err)
		return 0
	}
	pgame := &storage.PostgresGame{
		GameID:      -1,
		GuildID:     gid,
		ConnectCode: dgs.ConnectCode,
		StartTime:   int32(dgs.MatchStartUnix),
		WinType:     -1,
		EndTime:     -1,
	}
	i, err := psql.AddInitialGame(pgame)
	if err != nil {
		log.Println(err)
	}
	return i
}

func dumpGameToPostgres(dgs GameState, psql *storage.PsqlInterface, gameOver game.Gameover) {
	if dgs.MatchID < 0 || dgs.MatchStartUnix < 0 {
		log.Println("dgs match id or start time is <0; not dumping game to Postgres")
		return
	}
	end := time.Now().Unix()

	userGames := make([]*storage.PostgresUserGame, 0)

	imposterWin := gameOver.GameOverReason == game.ImpostorByKill ||
		gameOver.GameOverReason == game.ImpostorBySabotage ||
		gameOver.GameOverReason == game.ImpostorByVote ||
		gameOver.GameOverReason == game.ImpostorDisconnect

	for _, v := range dgs.UserData {
		if v.GetPlayerName() != amongus.UnlinkedPlayerName {
			inGameData, found := dgs.AmongUsData.GetByName(v.GetPlayerName())
			if !found {
				log.Println("No game data found for that player")
				continue
			}

			uid, err := strconv.ParseUint(v.User.UserID, 10, 64)
			if err != nil {
				log.Println(err)
				continue
			}
			gid, err := strconv.ParseUint(dgs.GuildID, 10, 64)
			if err != nil {
				log.Println(err)
				continue
			}

			puser, err := psql.EnsureUserExists(uid)
			if err != nil || puser == nil {
				log.Println(err)
				continue
			}

			// assume crewmate by default
			won := !imposterWin
			role := game.CrewmateRole
			for _, pi := range gameOver.PlayerInfos {
				// only override for the imposters
				if pi.IsImpostor {
					if strings.ToLower(pi.Name) == strings.ToLower(inGameData.Name) {
						role = game.ImposterRole
						won = imposterWin
						break
					}
				}
			}

			userGames = append(userGames, &storage.PostgresUserGame{
				UserID:      puser.UserID,
				GuildID:     gid,
				GameID:      dgs.MatchID,
				PlayerName:  inGameData.Name,
				PlayerColor: int16(inGameData.Color),
				PlayerRole:  int16(role),
				PlayerWon:   won,
			})
		}
	}
	log.Printf("Game %d has been completed and recorded in postgres\n", dgs.MatchID)

	err := psql.UpdateGameAndPlayers(dgs.MatchID, int16(gameOver.GameOverReason), end, userGames)
	if err != nil {
		log.Println(err)
	}
}
