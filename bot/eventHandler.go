package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/amongus"
	"github.com/automuteus/automuteus/v8/pkg/discord"
	"github.com/automuteus/automuteus/v8/pkg/game"
	"github.com/automuteus/automuteus/v8/pkg/premium"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

type EndGameMessage struct {
	reason  server.EndReason // empty means "delete the old game only", as /new does when replacing one
	message string
	done    chan error
}

func (bot *Bot) SubscribeToGameByConnectCode(guildID, connectCode string, endGameChannel chan EndGameMessage) {
	notify := task.Subscribe(ctx, bot.RedisInterface.client, connectCode)

	timer := time.NewTimer(time.Second * time.Duration(bot.captureTimeout))
	defer timer.Stop()
	defer notify.Close()

	dgsRequest := GameStateRequest{
		GuildID:     guildID,
		ConnectCode: connectCode,
	}
	gl := bot.gameLog(dgsRequest)
	gl.Info("subscribed to capture events")
	bot.trackGame(dgsRequest)
	defer bot.untrackGame(connectCode)

	// this process consumes the game's events only while it holds the lease, one burst at a time
	lease := bot.newConsumerLease(connectCode)
	defer lease.release(false)

	var endRequest *EndGameMessage
	var endErr error
	defer func() {
		bot.ChannelsMapLock.Lock()
		if bot.EndGameChannels[connectCode] == endGameChannel {
			delete(bot.EndGameChannels, connectCode)
		}
		bot.ChannelsMapLock.Unlock()
		if endRequest != nil && endRequest.done != nil {
			endRequest.done <- endErr
		}
	}()
	// finish ends the game: everyone is unmuted and its state deleted. That must not overlap a burst on another
	// process, which could still be applying a mute, so it waits for the lease, however long that takes. Releasing
	// with wake lets the other subscribers see the game is gone and close.
	finish := func(end EndGameMessage) {
		endRequest = &end
		lease.acquireBlocking("ending game")
		defer lease.release(true)
		if end.reason == "" {
			// The existing /new replacement path only requests deletion of the old game.
			bot.forceEndGame(dgsRequest)
			bot.metrics.RecordGameEnded(server.EndReasonReplaced)
		} else {
			endErr = bot.completeGame(dgsRequest, end.reason, end.message)
		}
	}

	// a standby polls for work it was not woken for: a lapsed lease with a backlog behind it
	standby := time.NewTicker(ConsumerLeaseTTL / 3)
	defer standby.Stop()

	// indicate to the broker that we're online and ready to start processing messages
	task.Ack(ctx, bot.RedisInterface.client, connectCode)

	for {
		select {
		case message := <-notify.Channel():
			timer.Reset(time.Second * time.Duration(bot.captureTimeout))
			if message == nil {
				break
			}
			if bot.consumeQueue(gl, guildID, dgsRequest, lease, endGameChannel, finish) {
				return
			}

		case <-standby.C:
			if bot.Draining() {
				break
			}
			if bot.consumeQueue(gl, guildID, dgsRequest, lease, endGameChannel, finish) {
				return
			}

		case <-timer.C:
			// The timer only requests a check. Missed notifications must not
			// make a standby end a game another consumer is still processing.
			if bot.consumeQueue(gl, guildID, dgsRequest, lease, endGameChannel, finish) {
				return
			}
			if bot.endGameIfInactive(dgsRequest, lease) {
				return
			}
			timer.Reset(ConsumerLeaseTTL / 3)
		case end := <-endGameChannel:
			gl.Info("end-game signal received; closing capture subscription")
			err := notify.Close()
			if err != nil {
				gl.Error("failed to close capture subscription", "err", err)
			}
			finish(end)
			return
		}
	}
}

// consumeQueue applies one burst of the game's queued capture events, in order, under the consumer lease. It
// returns true when the subscription is over: an end request was handled, or the game was ended elsewhere.
func (bot *Bot) consumeQueue(gl *slog.Logger, guildID string, dgsRequest GameStateRequest, lease *consumerLease, endGameChannel chan EndGameMessage, finish func(EndGameMessage)) bool {
	// admission and in-flight accounting happen together, before any work is taken from the queue
	done := bot.beginWork()
	defer done()
	if bot.Draining() {
		return false
	}
	if !lease.acquire() {
		return false
	}
	defer lease.release(false)
	// Checked only now, under the lease: a game ended by another subscriber is gone from the store and this
	// subscription is over. Checking before acquiring could see the game just before its owner deleted it, then
	// pop a queued event and recreate it.
	dgs, err := bot.store.ReadDiscordGameState(dgsRequest)
	if err != nil {
		gl.Error("failed to read game; leaving queued events for retry", "err", err)
		return false
	}
	if dgs == nil {
		gl.Info("game no longer exists; closing capture subscription")
		return true
	}

	for {
		// A queued stop takes priority over the next job, even if capture keeps publishing events.
		select {
		case end := <-endGameChannel:
			finish(end)
			return true
		default:
		}
		// A draining process hands the lease over and leaves queued events for the process that takes it.
		if bot.Draining() {
			gl.Info("draining; handing the game's consumer lease over")
			bot.metrics.RecordGameHandedOver()
			lease.release(true)
			return false
		}
		// Do not consume queued game events while settings are unavailable.
		sett, settingsErr := bot.settings.LoadGuildSettings(ctx, guildID)
		if settingsErr != nil {
			gl.Error("failed to load guild settings", "err", settingsErr)
			return false
		}
		entry, owner, err := lease.pop()
		if err != nil {
			gl.Error("failed to pop capture job", "err", err)
			return false
		}
		if !owner {
			return false
		}
		if entry == "" {
			return false
		}
		job, err := task.ParseJob(entry)
		if err != nil {
			gl.Error("malformed capture job; skipping it", "err", err)
			continue
		}
		gl.Info("capture event received", "type", job.JobType.String(), "payload", job.Payload)
		bot.refreshGameLiveness(dgsRequest.ConnectCode)
		bot.RedisInterface.RefreshActiveGame(guildID, dgsRequest.ConnectCode)

		// resolve premium once per job; every mute/deafen issued while handling it uses the same tier
		premTier := bot.premiumTier(guildID)

		correlatedUserID := bot.processJob(job, sett, premTier, dgsRequest)

		if job.JobType != task.ConnectionJob {
			gameEvent := storage.PostgresGameEvent{
				GameID:    -1,
				UserID:    nil,
				EventTime: int32(time.Now().Unix()),
				EventType: int16(job.JobType),
				Payload:   job.Payload.(string),
			}
			go bot.recordGameEvent(dgsRequest, correlatedUserID, gameEvent)
		}
	}
}

// processJob applies a single capture event to the game identified by dgsRequest, issuing whatever mutes, message
// edits, and match-history writes it implies. It returns the Discord user ID the event was correlated with, if any.
// This is the entry point for driving the bot with spoofed capture events: it has no dependency on the Redis job queue.
func (bot *Bot) processJob(job task.Job, sett *settings.GuildSettings, premTier premium.Tier, dgsRequest GameStateRequest) (correlatedUserID string) {
	payload, _ := job.Payload.(string)
	gl := bot.gameLog(dgsRequest)

	switch job.JobType {
	case task.ConnectionJob:
		lock, dgs := bot.store.GetDiscordGameStateAndLock(dgsRequest)
		for lock == nil {
			lock, dgs = bot.store.GetDiscordGameStateAndLock(dgsRequest)
		}
		dgs.Linked = payload == "true"
		dgs.ConnectCode = dgsRequest.ConnectCode
		bot.store.SetDiscordGameState(dgs, lock)

		bot.handleTrackedMembers(sett, premTier, 0, NoPriority, dgsRequest)
		bot.DispatchRefreshOrEdit(dgs, dgsRequest, sett)

	case task.LobbyJob:
		var lobby game.Lobby
		err := json.Unmarshal([]byte(payload), &lobby)
		if err != nil {
			gl.Error("malformed lobby payload", "err", err)
			break
		}

		bot.processLobby(sett, lobby, dgsRequest)
	case task.StateJob:
		num, err := strconv.ParseInt(payload, 10, 64)
		if err != nil {
			gl.Error("malformed phase payload", "err", err)
			break
		}

		bot.processTransition(game.Phase(num), dgsRequest, sett, premTier)
	case task.PlayerJob:
		var player game.Player
		err := json.Unmarshal([]byte(payload), &player)
		if err != nil {
			gl.Error("malformed player payload", "err", err)
			break
		}
		if player.Color > 17 || player.Color < 0 {
			break
		}

		shouldHandleTracked, userID, readOnlyDgs, err := bot.processPlayer(sett, player, dgsRequest, premTier)
		if shouldHandleTracked {
			bot.handleTrackedMembers(sett, premTier, 0, NoPriority, dgsRequest)
		}
		if err != nil {
			bot.discord.ChannelMessageSend(readOnlyDgs.GameStateMsg.MessageChannelID, sett.LocalizeMessage(&i18n.Message{
				ID:    "processplayer.error",
				Other: "Error in muting or deafening {{.User}}. Does the bot have permissions to mute/deafen users in {{.VoiceChannel}}?",
			},
				map[string]interface{}{
					"User":         discord.MentionByUserID(userID),
					"VoiceChannel": discord.MentionByChannelID(readOnlyDgs.VoiceChannel),
				},
			))
			bot.metrics.RecordDiscordRequests(server.MessageCreateDelete, 1)
		}
		correlatedUserID = userID
	case task.GameOverJob:
		var gameOverResult game.Gameover
		err := json.Unmarshal([]byte(payload), &gameOverResult)
		if err != nil {
			gl.Error("malformed gameover payload", "err", err)
			break
		}
		gl.Info("game over", "reason", gameOverResult.GameOverReason)

		// we only need a read-only state for making the game summary message
		dgs := bot.store.GetReadOnlyDiscordGameState(dgsRequest)
		if dgs != nil {
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
				channelID := bot.summaryChannel(dgsRequest.GuildID, sett, dgs.GameStateMsg.MessageChannelID)
				msg, err := bot.discord.ChannelMessageSendEmbed(channelID, embed)
				if delTime > 0 && err == nil {
					bot.metrics.RecordDiscordRequests(server.MessageCreateDelete, 2)
					go MessageDeleteWorker(bot.discord, msg.ChannelID, msg.ID, time.Minute*time.Duration(delTime))
				} else if err == nil {
					bot.metrics.RecordDiscordRequests(server.MessageCreateDelete, 1)
				}
			}
			go dumpGameToPostgres(gl, *dgs, bot.recorder, gameOverResult)

			// refresh the game message if the setting is marked (it is not locked, the previous dgs is
			// read-only). This means the original msg is refreshed, not the gameover message
			if sett.AutoRefresh {
				bot.RefreshGameStateMessage(dgsRequest, sett)
			}

			// now we need to fetch the state again (AFTER refreshing) to mark the game as complete/
			lock, dgs := bot.store.GetDiscordGameStateAndLock(dgsRequest)
			for lock == nil {
				lock, dgs = bot.store.GetDiscordGameStateAndLock(dgsRequest)
			}
			dgs.MatchID = -1
			dgs.MatchStartUnix = -1
			bot.store.SetDiscordGameState(dgs, lock)
		}
	}
	return correlatedUserID
}

// endInactiveGame ends a game whose capture went quiet: players are unmuted, the match is aborted, and the game is
// deleted.
func (bot *Bot) endInactiveGame(dgsRequest GameStateRequest) {
	bot.completeGame(dgsRequest, server.EndReasonInactivity, "")
}

// endGameIfInactive rechecks shared activity under the consumer lease. A local
// timer, a busy consumer, or a failed Redis read is never proof of inactivity.
// It returns true only when this subscription can close.
func (bot *Bot) endGameIfInactive(gsr GameStateRequest, lease *consumerLease) bool {
	done := bot.beginWork()
	defer done()
	if bot.Draining() || !lease.acquire() {
		return false
	}
	defer lease.release(false)
	gl := bot.gameLog(gsr)
	dgs, err := bot.store.ReadDiscordGameState(gsr)
	if err != nil {
		gl.Error("failed to read game for inactivity check", "err", err)
		return false
	}
	if dgs == nil {
		return true
	}
	queued, err := lease.client.LLen(ctx, rediskey.JobNamespace+gsr.ConnectCode).Result()
	if err != nil {
		gl.Error("failed to check queued capture events", "err", err)
		return false
	}
	if queued > 0 {
		return false
	}
	lastActivity, err := lease.client.ZScore(ctx, rediskey.ActiveGamesForGuild(gsr.GuildID), gsr.ConnectCode).Result()
	if err != nil {
		// Missing activity data is also unknown, not evidence of inactivity.
		gl.Error("failed to read shared game activity", "err", err)
		return false
	}
	if time.Since(time.Unix(int64(lastActivity), 0)) < time.Duration(bot.captureTimeout)*time.Second {
		return false
	}
	gl.Warn("ending game after confirmed capture inactivity", "timeout_seconds", bot.captureTimeout)
	if err := bot.completeGame(gsr, server.EndReasonInactivity, ""); err != nil {
		gl.Error("failed to end inactive game", "err", err)
		return false
	}
	lease.release(true)
	return true
}

// recordGameEvent stores a capture event against the active match, if there is one.
func (bot *Bot) recordGameEvent(dgsRequest GameStateRequest, userID string, ge storage.PostgresGameEvent) {
	gl := bot.gameLog(dgsRequest)
	dgs := bot.store.GetReadOnlyDiscordGameState(dgsRequest)
	if dgs == nil || dgs.MatchID <= 0 || dgs.MatchStartUnix <= 0 {
		return
	}
	ge.GameID = dgs.MatchID
	if userID != "" {
		num, err := strconv.ParseUint(userID, 10, 64)
		if err != nil {
			gl.Error("malformed user id for game event", "user", userID, "err", err)
			ge.UserID = nil
		} else {
			ge.UserID = &num
		}
	}
	gl.Debug("recording game event", "match", ge.GameID, "type", task.JobType(ge.EventType).String(), "user", userID)

	err := bot.recorder.AddEvent(&ge)
	if err != nil {
		gl.Error("failed to record game event", "err", err)
	}
}

type winnerRecord struct {
	userID string
	role   game.GameRole
}

func getWinners(dgs GameState, gameOver game.Gameover) []winnerRecord {
	var winners []winnerRecord

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

func (bot *Bot) processPlayer(sett *settings.GuildSettings, player game.Player, dgsRequest GameStateRequest, premTier premium.Tier) (bool, string, *GameState, error) {
	var err error
	gl := bot.gameLog(dgsRequest)
	if player.Name != "" {
		lock, dgs := bot.store.GetDiscordGameStateAndLock(dgsRequest)
		for lock == nil {
			lock, dgs = bot.store.GetDiscordGameStateAndLock(dgsRequest)
		}
		dgs.Linked = true

		defer bot.store.SetDiscordGameState(dgs, lock)

		if player.Disconnected || player.Action == game.LEFT {
			if player.Disconnected {
				gl.Info("player disconnected; purging player data", "player", player.Name)
				dgs.ClearPlayerDataByPlayerName(player.Name)
			}
			_, _, data := dgs.GameData.UpdatePlayer(player)

			userID := dgs.AttemptPairingByMatchingNames(data)
			// try pairing via the cached usernames
			if userID == "" {
				var uids map[string]interface{}
				uids, err = bot.store.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)
				userID = dgs.AttemptPairingByUserIDs(data, uids)
			} else {
				err = bot.applyToSingle(dgs, premTier, userID, false, false)
			}

			dgs.GameData.ClearPlayerData(player.Name)

			// only update the message if we're not in the tasks phase (info leaks)
			if dgs.GameData.GetPhase() != game.TASKS {
				bot.DispatchRefreshOrEdit(dgs, dgsRequest, sett)
			}

			return true, userID, dgs, err
		}
		updated, isAliveUpdated, data := dgs.GameData.UpdatePlayer(player)
		switch {
		case player.Action == game.JOINED:
			gl.Info("player joined", "player", player.Name, "color", player.Color)
			userID := dgs.AttemptPairingByMatchingNames(data)
			if userID == "" {
				var uids map[string]interface{}
				uids, err = bot.store.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)
				userID = dgs.AttemptPairingByUserIDs(data, uids)
			}
			bot.DispatchRefreshOrEdit(dgs, dgsRequest, sett)
			return true, userID, dgs, err
		case updated:
			userID := dgs.AttemptPairingByMatchingNames(data)
			if userID == "" {
				var uids map[string]interface{}
				uids, err = bot.store.GetUsernameOrUserIDMappings(dgs.GuildID, player.Name)
				userID = dgs.AttemptPairingByUserIDs(data, uids)
			}
			if isAliveUpdated && dgs.GameData.GetPhase() == game.TASKS {
				if sett.GetUnmuteDeadDuringTasks() || player.Action == game.EXILED {
					bot.DispatchRefreshOrEdit(dgs, dgsRequest, sett)
					return true, userID, dgs, err
				}
				gl.Debug("skipping status message update during tasks; would leak info", "player", player.Name)
				return false, userID, dgs, err
			}
			bot.DispatchRefreshOrEdit(dgs, dgsRequest, sett)
			if player.Action == game.EXILED {
				return false, userID, dgs, err // don't apply a mute to this player
			}
			return true, userID, dgs, err
		default:
			return false, "", nil, nil
		}
	}
	return false, "", nil, nil
}

func (bot *Bot) processTransition(phase game.Phase, dgsRequest GameStateRequest, sett *settings.GuildSettings, premTier premium.Tier) {
	lock, dgs := bot.store.GetDiscordGameStateAndLock(dgsRequest)
	for lock == nil {
		lock, dgs = bot.store.GetDiscordGameStateAndLock(dgsRequest)
	}

	oldPhase := dgs.GameData.UpdatePhase(phase)
	if oldPhase == phase {
		lock.Release(ctx)
		return
	}
	dgs.Linked = true
	gl := bot.gameLog(dgsRequest)
	gl.Info("phase changed", "from", game.PhaseNames[oldPhase], "to", game.PhaseNames[phase])
	// if we started a new game
	if oldPhase == game.LOBBY && phase == game.TASKS {
		matchStart := time.Now().Unix()
		dgs.MatchStartUnix = matchStart
		gameID := startGameInPostgres(gl, *dgs, bot.recorder)
		dgs.MatchID = int64(gameID)
		gl.Info("match started", "match", gameID, "start_unix", matchStart)
	}

	bot.store.SetDiscordGameState(dgs, lock)
	switch phase {
	case game.MENU:
		bot.DispatchRefreshOrEdit(dgs, dgsRequest, sett)
		err := bot.applyToAll(dgs, premTier, false, false)
		if err != nil {
			gl.Error("failed to unmute all users on return to menu", "err", err)
		}
		// on a gameover event from the capture, it's like going to the lobby; use that delay
	case game.GAMEOVER:
		phase = game.LOBBY
		fallthrough
	case game.LOBBY:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(sett, premTier, delay, NoPriority, dgsRequest)

		bot.DispatchRefreshOrEdit(dgs, dgsRequest, sett)

	case game.TASKS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		// when going from discussion to tasks, we should mute alive players FIRST
		priority := AlivePriority
		if oldPhase == game.LOBBY {
			priority = NoPriority
		}

		bot.handleTrackedMembers(sett, premTier, delay, priority, dgsRequest)
		bot.DispatchRefreshOrEdit(dgs, dgsRequest, sett)

	case game.DISCUSS:
		delay := sett.Delays.GetDelay(oldPhase, phase)
		bot.handleTrackedMembers(sett, premTier, delay, DeadPriority, dgsRequest)

		if sett.AutoRefresh {
			bot.RefreshGameStateMessage(dgsRequest, sett)
		} else {
			bot.DispatchRefreshOrEdit(dgs, dgsRequest, sett)
		}
	}
}

func (bot *Bot) processLobby(sett *settings.GuildSettings, lobby game.Lobby, dgsRequest GameStateRequest) {
	lock, dgs := bot.store.GetDiscordGameStateAndLock(dgsRequest)
	for lock == nil {
		lock, dgs = bot.store.GetDiscordGameStateAndLock(dgsRequest)
	}

	dgs.GameData.SetRoomRegionMap(lobby.LobbyCode, lobby.Region.ToString(), lobby.PlayMap)
	bot.store.SetDiscordGameState(dgs, lock)

	bot.DispatchRefreshOrEdit(dgs, dgsRequest, sett)
}

func startGameInPostgres(gl *slog.Logger, dgs GameState, psql GameRecorder) uint64 {
	if dgs.MatchStartUnix < 0 {
		return 0
	}
	gid, err := strconv.ParseUint(dgs.GuildID, 10, 64)
	if err != nil {
		gl.Error("invalid guild id", "err", err)
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
		gl.Error("failed to record match start", "err", err)
	}
	return i
}

func dumpGameToPostgres(gl *slog.Logger, dgs GameState, psql GameRecorder, gameOver game.Gameover) {
	if dgs.MatchID < 0 || dgs.MatchStartUnix < 0 {
		gl.Debug("no active match; not recording game result")
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
			inGameData, found := dgs.GameData.GetByName(v.GetPlayerName())
			if !found {
				gl.Warn("no in-game data for linked player", "player", v.GetPlayerName())
				continue
			}

			uid, err := strconv.ParseUint(v.User.UserID, 10, 64)
			if err != nil {
				gl.Error("invalid user id", "user", v.User.UserID, "err", err)
				continue
			}
			gid, err := strconv.ParseUint(dgs.GuildID, 10, 64)
			if err != nil {
				gl.Error("invalid guild id", "err", err)
				continue
			}

			puser, err := psql.EnsureUserExists(uid)
			if err != nil || puser == nil {
				gl.Error("failed to ensure user exists", "user", uid, "err", err)
				continue
			}

			// assume crewmate by default
			won := !imposterWin
			role := game.CrewmateRole
			for _, pi := range gameOver.PlayerInfos {
				// only override for the imposters
				if pi.IsImpostor {
					if strings.EqualFold(pi.Name, inGameData.Name) {
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
	err := psql.UpdateGameAndPlayers(dgs.MatchID, int16(gameOver.GameOverReason), end, userGames)
	if err != nil {
		gl.Error("failed to record match result", "match", dgs.MatchID, "err", err)
		return
	}
	gl.Info("match recorded", "match", dgs.MatchID, "players", len(userGames), "reason", gameOver.GameOverReason)
}
