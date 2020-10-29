package discord

import (
	"encoding/json"
	"github.com/bwmarrin/discordgo"
	"github.com/denverquane/amongusdiscord/game"
	"github.com/denverquane/amongusdiscord/storage"
	"log"
)

func (bot *Bot) SubscribeToGameByConnectCode(guildID, connectCode string, killChan <-chan bool) {
	connection, lobby, phase, player := bot.RedisInterface.SubscribeToGame(connectCode)
	for {
		select {
		case gameMessage := <-connection.Channel():
			log.Println(gameMessage)
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(guildID, "", "", connectCode)
			if lock == nil {
				log.Println("Couldn't obtain lock when receiving connect message!")
				break
			}

			if gameMessage.Payload == "true" {
				dgs.Linked = true
			} else {
				dgs.Linked = false
			}
			dgs.ConnectCode = connectCode
			bot.RedisInterface.SetDiscordGameState(dgs, lock)

			dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
			break
		case gameMessage := <-lobby.Channel():

			var lobby game.Lobby
			err := json.Unmarshal([]byte(gameMessage.Payload), &lobby)
			if err != nil {
				log.Println(err)
				break
			}
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(guildID, "", "", connectCode)
			if lock == nil {
				log.Println("Couldn't obtain lock when receiving lobby message!")
				break
			}

			bot.processLobby(dgs, bot.SessionManager.GetPrimarySession(), lobby)
			bot.RedisInterface.SetDiscordGameState(dgs, lock)
			break
		case gameMessage := <-phase.Channel():
			var phase game.Phase
			err := json.Unmarshal([]byte(gameMessage.Payload), &phase)
			if err != nil {
				log.Println(err)
				break
			}
			bot.processTransition(guildID, connectCode, phase)
			break
		case gameMessage := <-player.Channel():
			sett := bot.StorageInterface.GetGuildSettings(guildID)
			var player game.Player
			err := json.Unmarshal([]byte(gameMessage.Payload), &player)
			if err != nil {
				log.Println(err)
				break
			}

			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(guildID, "", "", connectCode)
			if lock == nil {
				log.Println("Couldn't obtain lock when receiving player message!")
				break
			}
			bot.processPlayer(dgs, sett, player)
			bot.RedisInterface.SetDiscordGameState(dgs, lock)
			break
		case k := <-killChan:
			if k {
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
				return
			}
		}
	}
}
func (bot *Bot) processPlayer(dgs *DiscordGameState, sett *storage.GuildSettings, player game.Player) {
	if player.Name != "" {
		if player.Disconnected || player.Action == game.LEFT {
			log.Println("I detected that " + player.Name + " disconnected or left! " +
				"I'm removing their linked game data; they will need to relink")

			dgs.ClearPlayerDataByPlayerName(player.Name)
			dgs.AmongUsData.ClearPlayerData(player.Name)
			dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
			return
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

				dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
			} else if updated {
				paired := dgs.AttemptPairingByMatchingNames(data)
				//try pairing via the cached usernames
				if !paired {
					uids := bot.RedisInterface.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)

					paired = dgs.AttemptPairingByUserIDs(data, uids)
				}
				//log.Println("Player update received caused an update in cached state")
				if isAliveUpdated && dgs.AmongUsData.GetPhase() == game.TASKS {
					if sett.GetUnmuteDeadDuringTasks() {
						// unmute players even if in tasks because unmuteDeadDuringTasks is true
						dgs.handleTrackedMembers(bot.SessionManager, sett, 0, NoPriority, game.TASKS)
						dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
					} else {
						log.Println("NOT updating the discord status message; would leak info")
					}
				} else {
					dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
				}
			} else {
				//No changes occurred; no reason to update
			}
		}
	}
}

func (bot *Bot) processTransition(guildID, connectCode string, phase game.Phase) {
	sett := bot.StorageInterface.GetGuildSettings(guildID)
	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(guildID, "", "", connectCode)
	if lock == nil {
		log.Println("Couldn't obtain lock when processing transition message!")
		return
	}

	oldPhase := dgs.AmongUsData.UpdatePhase(phase)
	if oldPhase == phase {
		lock.Release()
		return
	}

	switch phase {
	case game.MENU:
		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		dgs.RemoveAllReactions(bot.SessionManager.GetPrimarySession())
		break
	case game.LOBBY:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		dgs.handleTrackedMembers(bot.SessionManager, sett, delay, NoPriority, phase)

		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		dgs.AddAllReactions(bot.SessionManager.GetPrimarySession(), bot.StatusEmojis[true])
		break
	case game.TASKS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		//when going from discussion to tasks, we should mute alive players FIRST
		priority := AlivePriority
		if oldPhase == game.LOBBY {
			priority = NoPriority
		}

		dgs.handleTrackedMembers(bot.SessionManager, sett, delay, priority, phase)
		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		break
	case game.DISCUSS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		dgs.handleTrackedMembers(bot.SessionManager, sett, delay, DeadPriority, dgs.AmongUsData.GetPhase())

		dgs.Edit(bot.SessionManager.GetPrimarySession(), bot.gameStateResponse(dgs))
		break
	}
	bot.RedisInterface.SetDiscordGameState(dgs, lock)
}

func (bot *Bot) processLobby(dgs *DiscordGameState, s *discordgo.Session, lobby game.Lobby) {
	dgs.AmongUsData.SetRoomRegion(lobby.LobbyCode, lobby.Region.ToString())

	dgs.Edit(s, bot.gameStateResponse(dgs))
}

func (bot *Bot) updatesListener(s *discordgo.Session, guildID string, globalUpdates chan BroadcastMessage) {
	for {
		select {
		case worldUpdate := <-globalUpdates:
			bot.ChannelsMapLock.Lock()
			for i, connCode := range bot.ConnsToGames {
				if worldUpdate.Type == GRACEFUL_SHUTDOWN {

					go bot.gracefulShutdownWorker(guildID, connCode, s, worldUpdate.Data, worldUpdate.Message)
					lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(guildID, "", "", connCode)
					if lock == nil {
						log.Println("Couldn't obtain lock when processing graceful shutdown message!")
						break
					}
					dgs.Linked = false
					bot.RedisInterface.SetDiscordGameState(dgs, lock)
					delete(bot.ConnsToGames, i)
				}
			}
			bot.ChannelsMapLock.Unlock()
		}
	}
}
