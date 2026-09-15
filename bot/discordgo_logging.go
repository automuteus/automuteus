package bot

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// unknownEventPrefix is the start of the message discordgo logs (at LogWarning) for every gateway event it has no
// struct for. Discord keeps adding events (e.g. VOICE_CHANNEL_START_TIME_UPDATE, sent whenever a voice channel
// becomes occupied) that the bot has no use for, and each one would otherwise be logged with its full JSON payload.
const unknownEventPrefix = "unknown event:"

// droppedPrefixes are discordgo messages that dump an entire gateway event (as a Go literal with the raw JSON rendered
// byte by byte) on every reconnect. The reconnect itself is reported by other messages; these add only noise.
var droppedPrefixes = []string{
	unknownEventPrefix,
	"Expected READY/RESUMED, instead got:",
	"First Packet:",
}

// droppedMessages are discordgo's informational bookkeeping lines, emitted on every open, close, and reconnect.
// They carry no information beyond the messages around them ("connecting to gateway", "sending resume packet",
// "successfully reconnected to gateway", "trying to reconnect to gateway"), which are kept.
var droppedMessages = map[string]struct{}{
	"called":                                   {},
	"exiting":                                  {},
	"creating new VoiceConnections map":        {},
	"closing listening channel":                {},
	"sending close frame":                      {},
	"closing gateway websocket":                {},
	"emit disconnect event":                    {},
	"Op 10 Hello Packet received from Discord": {},
}

var installDiscordgoLoggerOnce sync.Once

// installDiscordgoLogger routes discordgo's logging through discordgoLogger. It is safe to call multiple times
// (once per shard); only the first call has an effect.
func installDiscordgoLogger() {
	installDiscordgoLoggerOnce.Do(func() {
		discordgo.Logger = discordgoLogger
	})
}

// discordgoLogger forwards discordgo's log messages to the default slog logger, tagged with the library and the
// source location inside it. Session log levels are still respected: discordgo only calls this for messages at or
// below the session's LogLevel.
func discordgoLogger(msgL, caller int, format string, a ...interface{}) {
	if shouldDropDiscordgoMessage(msgL, format) {
		return
	}

	// caller is relative to discordgo's msglog; we are one frame further from the message source
	pc, file, line, _ := runtime.Caller(caller + 1)

	files := strings.Split(file, "/")
	file = files[len(files)-1]

	name := runtime.FuncForPC(pc).Name()
	fns := strings.Split(name, ".")
	name = fns[len(fns)-1]

	msg := strings.TrimSpace(fmt.Sprintf(format, a...))

	slog.Default().Log(context.Background(), discordgoLevel(msgL), msg,
		"component", "discordgo",
		"source", fmt.Sprintf("%s:%d:%s()", file, line, name),
	)
}

func discordgoLevel(msgL int) slog.Level {
	switch msgL {
	case discordgo.LogError:
		return slog.LevelError
	case discordgo.LogWarning:
		return slog.LevelWarn
	case discordgo.LogInformational:
		return slog.LevelInfo
	default:
		return slog.LevelDebug
	}
}

func shouldDropDiscordgoMessage(msgL int, format string) bool {
	if _, ok := droppedMessages[format]; ok {
		return true
	}
	for _, prefix := range droppedPrefixes {
		if strings.HasPrefix(format, prefix) {
			// the unknown-event message is only dropped at its usual warning level; the event dumps are dropped at
			// any level since they appear at both warning and informational
			if prefix != unknownEventPrefix || msgL == discordgo.LogWarning {
				return true
			}
		}
	}
	return false
}
