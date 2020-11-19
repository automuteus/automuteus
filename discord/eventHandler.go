package discord

import (
	"encoding/json"
	rediscommon "github.com/denverquane/amongusdiscord/redis-common"
	"github.com/go-redis/redis/v8"
	"log"
	"strconv"
	"time"

	"github.com/automuteus/galactus/broker"
	"github.com/bwmarrin/discordgo"
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
				log.Println("Popped job w/ payload " + job.Payload.(string))
				bot.refreshGameLiveness(connectCode)
				bot.RedisInterface.RefreshActiveGame(guildID, connectCode)

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
					bot.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, dgsRequest)

					dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett), bot.MetricsCollector, bot.RedisInterface)
					break
				case broker.Lobby:
					var lobby game.Lobby
					err := json.Unmarshal([]byte(job.Payload.(string)), &lobby)
					if err != nil {
						log.Println(err)
						break
					}

					sett := bot.StorageInterface.GetGuildSettings(guildID)
					bot.processLobby(bot.SessionManager.GetPrimarySession(), sett, lobby, dgsRequest)
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
					shouldHandleTracked := bot.processPlayer(sett, player, dgsRequest)
					if shouldHandleTracked {
						bot.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, dgsRequest)
					}

					break
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

func (bot *Bot) processPlayer(sett *storage.GuildSettings, player game.Player, dgsRequest GameStateRequest) bool {
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
				dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett), bot.MetricsCollector, bot.RedisInterface)
			}

			return true
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

				dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett), bot.MetricsCollector, bot.RedisInterface)
				return true
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
						dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett), bot.MetricsCollector, bot.RedisInterface)
						return true
					} else {
						log.Println("NOT updating the discord status message; would leak info")
						return false
					}
				} else {
					dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett), bot.MetricsCollector, bot.RedisInterface)
					if player.Action == game.EXILED {
						return false //don't apply a mute to this player
					}
					return true
				}
			} else {
				return false
				//No changes occurred; no reason to update
			}
		}
	}
	return false
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
		//if we went to lobby from anywhere else but the menu, assume the game is over
	} else if phase == game.LOBBY && oldPhase != game.MENU {
		//TODO process the game's completion and send to Postgres
		//only process games that actually receive the end-game event from the capture! Might need to start a worker
		//to listen for this

		dgs.MatchID = -1
		dgs.MatchStartUnix = -1
	}

	bot.RedisInterface.SetDiscordGameState(dgs, lock)
	switch phase {
	case game.MENU:
		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett), bot.MetricsCollector, bot.RedisInterface)
		bot.applyToAll(dgs, false, false)
		//go dgs.RemoveAllReactions(bot.SessionManager.GetPrimarySession())
		break
	case game.LOBBY:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(bot.SessionManager, sett, delay, NoPriority, dgsRequest)

		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett), bot.MetricsCollector, bot.RedisInterface)

		break
	case game.TASKS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		//when going from discussion to tasks, we should mute alive players FIRST
		priority := AlivePriority
		if oldPhase == game.LOBBY {
			priority = NoPriority
		}

		bot.handleTrackedMembers(bot.SessionManager, sett, delay, priority, dgsRequest)
		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett), bot.MetricsCollector, bot.RedisInterface)
		break
	case game.DISCUSS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(bot.SessionManager, sett, delay, DeadPriority, dgsRequest)

		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett), bot.MetricsCollector, bot.RedisInterface)
		break
	}
}

func (bot *Bot) processLobby(s *discordgo.Session, sett *storage.GuildSettings, lobby game.Lobby, dgsRequest GameStateRequest) {
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	if lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
	}
	dgs.AmongUsData.SetRoomRegion(lobby.LobbyCode, lobby.Region.ToString())
	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	dgs.Edit(s, bot.gameStateResponse(dgs, sett), bot.MetricsCollector, bot.RedisInterface)
}
