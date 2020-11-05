package discord

import (
	"encoding/json"
	"log"
	"time"

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
	log.Println("Started Redis Subscription worker")

	timer := time.NewTimer(time.Second * time.Duration(bot.captureTimeout))
	connection, lobby, phase, player := bot.RedisInterface.SubscribeToGame(connectCode)
	sett := bot.StorageInterface.GetGuildSettings(guildID)

	dgsRequest := GameStateRequest{
		GuildID:     guildID,
		ConnectCode: connectCode,
	}
	for {
		select {
		case gameMessage := <-connection.Channel():
			timer.Reset(time.Second * time.Duration(bot.captureTimeout))
			if gameMessage == nil {
				break
			}

			//tell the producer of the connection event that we got their message
			bot.RedisInterface.PublishConnectUpdateAck(connectCode)
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
			for lock == nil {
				lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(dgsRequest)
			}
			if gameMessage.Payload == "true" {
				dgs.Linked = true
			} else {
				dgs.Linked = false
			}
			dgs.ConnectCode = connectCode
			bot.RedisInterface.SetDiscordGameState(dgs, lock)

			bot.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, dgsRequest)

			dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett))
			break
		case gameMessage := <-lobby.Channel():
			timer.Reset(time.Second * time.Duration(bot.captureTimeout))
			if gameMessage == nil {
				break
			}
			var lobby game.Lobby
			err := json.Unmarshal([]byte(gameMessage.Payload), &lobby)
			if err != nil {
				log.Println(err)
				break
			}

			bot.processLobby(bot.SessionManager.GetPrimarySession(), sett, lobby, dgsRequest)
			break
		case gameMessage := <-phase.Channel():
			timer.Reset(time.Second * time.Duration(bot.captureTimeout))
			if gameMessage == nil {
				break
			}
			var phase game.Phase
			err := json.Unmarshal([]byte(gameMessage.Payload), &phase)
			if err != nil {
				log.Println(err)
				break
			}
			bot.processTransition(phase, dgsRequest)
			break
		case gameMessage := <-player.Channel():
			timer.Reset(time.Second * time.Duration(bot.captureTimeout))
			if gameMessage == nil {
				break
			}
			var player game.Player
			err := json.Unmarshal([]byte(gameMessage.Payload), &player)
			if err != nil {
				log.Println(err)
				break
			}

			shouldHandleTracked := bot.processPlayer(sett, player, dgsRequest)
			if shouldHandleTracked {
				bot.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, dgsRequest)
			}

			break
		case <-timer.C:
			timer.Stop()
			log.Printf("Killing game w/ code %s after %d seconds of inactivity!\n", connectCode, bot.captureTimeout)
			err := connection.Close()
			if err != nil {
				log.Println(err)
			}
			err = lobby.Close()
			if err != nil {
				log.Println(err)
			}
			err = phase.Close()
			if err != nil {
				log.Println(err)
			}
			err = player.Close()
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
			err := connection.Close()
			if err != nil {
				log.Println(err)
			}
			err = lobby.Close()
			if err != nil {
				log.Println(err)
			}
			err = phase.Close()
			if err != nil {
				log.Println(err)
			}
			err = player.Close()
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

			dgs.AmongUsData.ClearPlayerData(player.Name)
			dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett))
			return true
		} else {
			updated, isAliveUpdated, data := dgs.AmongUsData.UpdatePlayer(player)

			if player.Action == game.JOINED {
				log.Println("Detected a player joined, refreshing User data mappings")
				paired := dgs.AttemptPairingByMatchingNames(data)
				//try pairing via the cached usernames
				if !paired {
					uids := bot.RedisInterface.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)
					paired = dgs.AttemptPairingByUserIDs(data, uids)
				}

				dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett))
				return true
			} else if updated {
				paired := dgs.AttemptPairingByMatchingNames(data)
				//try pairing via the cached usernames
				if !paired {
					uids := bot.RedisInterface.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)

					paired = dgs.AttemptPairingByUserIDs(data, uids)
				}
				//log.Println("Player update received caused an update in cached state")
				if isAliveUpdated && dgs.AmongUsData.GetPhase() == game.TASKS {
					if sett.GetUnmuteDeadDuringTasks() || player.Action == game.EXILED {
						dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett))
						return true
					} else {
						log.Println("NOT updating the discord status message; would leak info")
						return false
					}
				} else {
					dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett))
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
		lock.Release()
		return
	}

	bot.RedisInterface.SetDiscordGameState(dgs, lock)
	switch phase {
	case game.MENU:
		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett))
		bot.applyToAll(dgs, false, false)
		go dgs.RemoveAllReactions(bot.SessionManager.GetPrimarySession())
		break
	case game.LOBBY:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(bot.SessionManager, sett, delay, NoPriority, dgsRequest)

		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett))
		go dgs.AddAllReactions(bot.SessionManager.GetPrimarySession(), bot.StatusEmojis[true])
		break
	case game.TASKS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		//when going from discussion to tasks, we should mute alive players FIRST
		priority := AlivePriority
		if oldPhase == game.LOBBY {
			priority = NoPriority
		}

		bot.handleTrackedMembers(bot.SessionManager, sett, delay, priority, dgsRequest)
		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett))
		break
	case game.DISCUSS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(bot.SessionManager, sett, delay, DeadPriority, dgsRequest)

		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs, sett))
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

	dgs.Edit(s, bot.gameStateResponse(dgs, sett))
}
