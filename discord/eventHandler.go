package discord

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/metrics"
	rediscommon "github.com/denverquane/amongusdiscord/redis-common"
	"github.com/go-redis/redis/v8"
	"log"
	"strconv"
	"time"

	"github.com/automuteus/galactus/broker"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
)

type EndGameType int

const (
	EndAndSave EndGameType = iota
	EndAndWipe
)

type EndGameMessage struct {
	EndGameType EndGameType
}

func (bot *Bot) SubscribeToGameByConnectCode(guildID, connectCode string, endGameChannel chan EndGameMessage) {
	log.Println("Started Redis Subscription worker for " + connectCode)

	notify := broker.Subscribe(ctx, bot.RedisInterface.client, connectCode)

	timer := time.NewTimer(time.Second * time.Duration(bot.captureTimeout))

	dgsRequest := GameStateRequest{
		GuildID:     guildID,
		ConnectCode: connectCode,
	}

	//indicate to the broker that we're online and ready to start processing messages
	broker.Ack(ctx, bot.RedisInterface.client, connectCode)

	for {
		select {
		case message := <-notify.Channel():
			timer.Reset(time.Second * time.Duration(bot.captureTimeout))
			if message == nil {
				break
			}

			//anytime we get a notification message, continue pulling messages off the list until there are no more
			for {
				job, err := broker.PopJob(ctx, bot.RedisInterface.client, connectCode)
				if err == redis.Nil {
					break
				} else if err != nil {
					log.Println(err)
					break
				}
				log.Printf("Popped job of type %d w/ payload %s\n", job.JobType, job.Payload.(string))
				bot.refreshGameLiveness(connectCode)
				bot.RedisInterface.RefreshActiveGame(guildID, connectCode)

				gameEvent := storage.PostgresGameEvent{
					GameID:       -1,
					HashedUserID: -1,
					EventTime:    int32(time.Now().Unix()),
					EventType:    int16(job.JobType),
					Payload:      job.Payload.(string),
				}
				correlatedUserID := ""

				switch job.JobType {
				case broker.Connection:
					lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
					for lock == nil {
						lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
					}
					if job.Payload == "true" {
						dgs.Linked = true
					} else {
						dgs.Linked = false
					}
					dgs.ConnectCode = connectCode
					bot.RedisInterface.SetDiscordGameState(dgs, lock)

					sett := bot.StorageInterface.GetGuildSettings(guildID)
					bot.handleTrackedMembers(bot.PrimarySession, sett, 0, NoPriority, dgsRequest)

					edited := dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
					if edited {
						bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
					}
					break
				case broker.Lobby:
					var lobby game.Lobby
					err := json.Unmarshal([]byte(job.Payload.(string)), &lobby)
					if err != nil {
						log.Println(err)
						break
					}

					sett := bot.StorageInterface.GetGuildSettings(guildID)
					bot.processLobby(sett, lobby, dgsRequest)
					break
				case broker.State:
					num, err := strconv.ParseInt(job.Payload.(string), 10, 64)
					if err != nil {
						log.Println(err)
						break
					}

					bot.processTransition(game.Phase(num), dgsRequest)
					break
				case broker.Player:
					var player game.Player
					err := json.Unmarshal([]byte(job.Payload.(string)), &player)
					if err != nil {
						log.Println(err)
						break
					}

					sett := bot.StorageInterface.GetGuildSettings(guildID)
					shouldHandleTracked, userID := bot.processPlayer(sett, player, dgsRequest)
					if shouldHandleTracked {
						bot.handleTrackedMembers(bot.PrimarySession, sett, 0, NoPriority, dgsRequest)
					}
					correlatedUserID = userID

					break
				case 4: //TODO use from Galactus2.0
					var gameOverResult game.Gameover
					log.Println("Successfully identified game over event:")
					log.Println(job.Payload)
					err := json.Unmarshal([]byte(job.Payload.(string)), &gameOverResult)
					if err != nil {
						log.Println(err)
						break
					}

					sett := bot.StorageInterface.GetGuildSettings(guildID)
					lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)

					if lock != nil && dgs != nil {
						delTime := sett.GetDeleteGameSummaryMinutes()
						if delTime != 0 {
							oldPhase := dgs.AmongUsData.UpdatePhase(game.GAMEOVER)
							embed := bot.gameStateResponse(dgs, sett)
							dgs.AmongUsData.Phase = oldPhase
							embed.Description = fmt.Sprintf("Game over. View stats using Match ID: `%d:%s`", dgs.MatchID, dgs.ConnectCode)
							if delTime > 0 {
								embed.Footer = &discordgo.MessageEmbedFooter{
									Text:         fmt.Sprintf("Deleting message %d mins from", delTime),
									IconURL:      "",
									ProxyIconURL: "",
								}
							}
							msg, err := bot.PrimarySession.ChannelMessageSendEmbed(dgs.GameStateMsg.MessageChannelID, embed)
							if delTime > 0 && err == nil {
								bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, 2)
								go MessageDeleteWorker(bot.PrimarySession, dgs.GameStateMsg.MessageChannelID, msg.ID, time.Minute*time.Duration(delTime))
							} else if err == nil {
								bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageCreateDelete, 1)
							}
						}
						go dumpGameToPostgres(*dgs, bot.PostgresInterface, gameOverResult)
						dgs.MatchID = -1
						dgs.MatchStartUnix = -1
						bot.RedisInterface.SetDiscordGameState(dgs, lock)
						bot.RefreshGameStateMessage(dgsRequest, sett)
					}
				}
				if job.JobType != broker.Connection {
					go func() {
						dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(dgsRequest)
						if dgs.MatchID > 0 && dgs.MatchStartUnix > 0 {
							gameEvent.GameID = dgs.MatchID
							if correlatedUserID != "" {
								user, err := bot.PostgresInterface.GetUserByString(correlatedUserID)
								if err == nil && user != nil && user.Opt {
									gameEvent.HashedUserID = user.HashedUserID
								}
							}

							err := bot.PostgresInterface.AddEvent(&gameEvent)
							if err != nil {
								log.Println(err)
							}
						}
					}()
				}
			}
			break

		case <-timer.C:
			timer.Stop()
			log.Printf("Killing game w/ code %s after %d seconds of inactivity!\n", connectCode, bot.captureTimeout)
			err := notify.Close()
			if err != nil {
				log.Println(err)
			}
			go bot.forceEndGame(dgsRequest)
			bot.ChannelsMapLock.Lock()
			delete(bot.EndGameChannels, connectCode)
			bot.ChannelsMapLock.Unlock()

			return
		case k := <-endGameChannel:
			log.Println("Redis subscriber received kill signal, closing all pubsubs")
			err := notify.Close()
			if err != nil {
				log.Println(err)
			}

			if k.EndGameType == EndAndSave {
				go bot.gracefulShutdownWorker(guildID, connectCode)
			} else if k.EndGameType == EndAndWipe {
				bot.forceEndGame(dgsRequest)
			}

			return
		}
	}
}

func (bot *Bot) processPlayer(sett *storage.GuildSettings, player game.Player, dgsRequest GameStateRequest) (bool, string) {
	if player.Name != "" {
		lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
		for lock == nil {
			lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
		}

		defer bot.RedisInterface.SetDiscordGameState(dgs, lock)

		if player.Disconnected || player.Action == game.LEFT {
			if player.Disconnected {
				log.Println("I detected that " + player.Name + " disconnected, I'm purging their player data!")
				dgs.ClearPlayerDataByPlayerName(player.Name)
			}
			_, _, data := dgs.AmongUsData.UpdatePlayer(player)

			userID := dgs.AttemptPairingByMatchingNames(data)
			//try pairing via the cached usernames
			if userID == "" {
				uids := bot.RedisInterface.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)
				userID = dgs.AttemptPairingByUserIDs(data, uids)
			}
			if userID != "" {
				bot.applyToSingle(dgs, userID, false, false)
			}

			dgs.AmongUsData.ClearPlayerData(player.Name)

			//only update the message if we're not in the tasks phase (info leaks)
			if dgs.AmongUsData.GetPhase() != game.TASKS {
				edited := dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
				if edited {
					bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
				}
			}

			return true, userID
		} else {
			updated, isAliveUpdated, data := dgs.AmongUsData.UpdatePlayer(player)

			if player.Action == game.JOINED {
				log.Println("Detected a player joined, refreshing User data mappings")
				userID := dgs.AttemptPairingByMatchingNames(data)
				//try pairing via the cached usernames
				if userID == "" {
					uids := bot.RedisInterface.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)
					userID = dgs.AttemptPairingByUserIDs(data, uids)
				}

				edited := dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
				if edited {
					bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
				}
				return true, userID
			} else if updated {
				userID := dgs.AttemptPairingByMatchingNames(data)
				//try pairing via the cached usernames
				if userID == "" {
					uids := bot.RedisInterface.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)

					userID = dgs.AttemptPairingByUserIDs(data, uids)
				}
				//log.Println("Player update received caused an update in cached state")
				if isAliveUpdated && dgs.AmongUsData.GetPhase() == game.TASKS {
					log.Println(sett.GetUnmuteDeadDuringTasks())
					if sett.GetUnmuteDeadDuringTasks() || player.Action == game.EXILED {
						edited := dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
						if edited {
							bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
						}
						return true, userID
					} else {
						log.Println("NOT updating the discord status message; would leak info")
						return false, userID
					}
				} else {
					edited := dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
					if edited {
						bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
					}
					if player.Action == game.EXILED {
						return false, userID //don't apply a mute to this player
					}
					return true, userID
				}
			} else {
				return false, ""
				//No changes occurred; no reason to update
			}
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

	oldPhase := dgs.AmongUsData.UpdatePhase(phase)
	if oldPhase == phase {
		lock.Release(ctx)
		return
	}
	//if we started a new game
	if oldPhase == game.LOBBY && phase == game.TASKS {
		matchID := rediscommon.GetAndIncrementMatchID(bot.RedisInterface.client)
		matchStart := time.Now().Unix()
		dgs.MatchStartUnix = matchStart
		dgs.MatchID = matchID
		log.Printf("New match has begun. ID %d and starttime %d\n", matchID, matchStart)
		go startGameInPostgres(*dgs, bot.PostgresInterface)
	}

	bot.RedisInterface.SetDiscordGameState(dgs, lock)
	switch phase {
	case game.MENU:
		edited := dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
		if edited {
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
		}
		bot.applyToAll(dgs, false, false)
		//go dgs.RemoveAllReactions(bot.PrimarySession.GetPrimarySession())
		break
	case game.LOBBY:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(bot.PrimarySession, sett, delay, NoPriority, dgsRequest)

		edited := dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
		if edited {
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
		}

		break
	case game.TASKS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		//when going from discussion to tasks, we should mute alive players FIRST
		priority := AlivePriority
		if oldPhase == game.LOBBY {
			priority = NoPriority
		}

		bot.handleTrackedMembers(bot.PrimarySession, sett, delay, priority, dgsRequest)
		edited := dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
		if edited {
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
		}
		break
	case game.DISCUSS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(bot.PrimarySession, sett, delay, DeadPriority, dgsRequest)

		edited := dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
		if edited {
			bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
		}
		break
	}
}

func (bot *Bot) processLobby(sett *storage.GuildSettings, lobby game.Lobby, dgsRequest GameStateRequest) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	if lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	}
	dgs.AmongUsData.SetRoomRegion(lobby.LobbyCode, lobby.Region.ToString())
	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	edited := dgs.Edit(bot.PrimarySession, bot.gameStateResponse(dgs, sett))
	if edited {
		bot.MetricsCollector.RecordDiscordRequests(bot.RedisInterface.client, metrics.MessageEdit, 1)
	}
}

func startGameInPostgres(dgs DiscordGameState, psql *storage.PsqlInterface) {
	if dgs.MatchID < 0 || dgs.MatchStartUnix < 0 {
		return
	}
	gid, err := strconv.ParseUint(dgs.GuildID, 10, 64)
	if err != nil {
		log.Println(err)
		return
	}
	pgame := &storage.PostgresGame{
		GameID:      dgs.MatchID,
		GuildID:     gid,
		ConnectCode: dgs.ConnectCode,
		StartTime:   int32(dgs.MatchStartUnix),
		WinType:     -1,
		EndTime:     -1,
	}
	err = psql.AddInitialGame(pgame)
	if err != nil {
		log.Println(err)
	}
}

func dumpGameToPostgres(dgs DiscordGameState, psql *storage.PsqlInterface, gameOver game.Gameover) {
	if dgs.MatchID < 0 || dgs.MatchStartUnix < 0 {
		return
	}
	end := time.Now().Unix()

	userGames := make([]*storage.PostgresUserGame, 0)

	imposterWin := gameOver.GameOverReason == game.ImpostorByKill ||
		gameOver.GameOverReason == game.ImpostorBySabotage ||
		gameOver.GameOverReason == game.ImpostorByVote ||
		gameOver.GameOverReason == game.ImpostorDisconnect

	for _, v := range dgs.UserData {
		if v.GetPlayerName() != game.UnlinkedPlayerName {
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
			if err != nil {
				log.Println(err)
				continue
			}

			won := !imposterWin
			role := game.CrewmateRole
			for _, v := range gameOver.PlayerInfos {
				//only override for the imposters
				if v.IsImpostor {
					if v.Name == inGameData.Name {
						role = game.ImposterRole
						won = imposterWin
					}
				}
			}
			log.Println(puser)
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
