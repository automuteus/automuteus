package tokenprovider

import (
	"context"
	"encoding/json"
	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/go-redis/redis/v8"
)

func RecordDiscordRequestsByCounts(client *redis.Client, counts task.MuteDeafenSuccessCounts) {
	server.RecordDiscordRequests(client, server.MuteDeafenOfficial, counts.Official)
	server.RecordDiscordRequests(client, server.MuteDeafenWorker, counts.Worker)
	server.RecordDiscordRequests(client, server.MuteDeafenCapture, counts.Capture)
	server.RecordDiscordRequests(client, server.InvalidRequest, counts.RateLimit)
}

func (tokenProvider *TokenProvider) attemptOnSecondaryTokens(guildID, userID string, tokenSubset map[string]struct{}, request task.UserModify) string {
	l := tokenProvider.logger(guildID).With("user", userID)
	if len(tokenProvider.activeSessions) > 0 {
		sess, hToken := tokenProvider.getSession(guildID, tokenSubset)
		if sess != nil {
			err := task.ApplyMuteDeaf(sess, guildID, userID, request.Mute, request.Deaf)
			if err != nil {
				l.Error("secondary bot voice change failed", "token", hToken, "err", err)

				// don't attempt this token for this guild for another 5 minutes
				err = tokenProvider.BlacklistTokenForDuration(guildID, hToken, UnresponsiveCaptureBlacklistDuration)
				if err != nil {
					l.Error("failed to blacklist token", "token", hToken, "err", err)
				}
			} else {
				l.Debug("voice change applied via secondary bot", "token", hToken, "mute", request.Mute, "deaf", request.Deaf)
				return hToken
			}
		} else {
			l.Debug("no usable secondary token; trying other methods")
		}
	} else {
		l.Debug("no secondary tokens configured; skipping")
	}
	return ""
}

func (tokenProvider *TokenProvider) attemptOnCaptureBot(guildID, connectCode string, gid uint64, request task.UserModify) bool {
	l := tokenProvider.logger(guildID).With("code", connectCode, "user", request.UserID)
	// this is cheeky, but use the connect code as part of the lock; don't issue too many requests on the capture client w/ this code
	if tokenProvider.IncrAndTestGuildTokenComboLock(guildID, connectCode) {
		// if the secondary token didn't work, then next we try the client-side capture request
		taskObj := task.NewModifyTask(gid, request.UserID, task.PatchParams{
			Deaf: request.Deaf,
			Mute: request.Mute,
		})
		jBytes, err := json.Marshal(taskObj)
		if err != nil {
			l.Error("failed to marshal capture task", "err", err)
			return false
		}
		acked := make(chan bool)
		// now we wait for an ack with respect to actually performing the mute
		pubsub := tokenProvider.client.Subscribe(context.Background(), rediskey.CompleteTask(taskObj.TaskID))
		// wait for Redis to confirm the subscription before publishing the task; otherwise a fast ack can arrive
		// before we are listening and be lost, which would blacklist a working capture client
		if _, err = pubsub.Receive(context.Background()); err != nil {
			l.Error("failed to subscribe for capture task ack", "err", err)
			_ = pubsub.Close()
			return false
		}
		err = tokenProvider.client.Publish(context.Background(), rediskey.TasksList(connectCode), jBytes).Err()
		if err != nil {
			l.Error("failed to publish capture task", "err", err)
		} else {
			go tokenProvider.waitForAck(pubsub, acked)
			res := <-acked
			if res {
				l.Debug("voice change applied via capture client")

				// hooray! we did the mute with a client token!
				return true
			}
			err = tokenProvider.BlacklistTokenForDuration(guildID, connectCode, UnresponsiveCaptureBlacklistDuration)
			if err == nil {
				l.Warn("no ack from capture client; blacklisting it", "duration", UnresponsiveCaptureBlacklistDuration)
			}
		}
	} else {
		l.Debug("capture client near rate limit; deferring to primary bot")
	}
	return false
}
