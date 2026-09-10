package bot

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// unknownEventPrefix is the start of the message discordgo logs (at LogWarning) for every gateway event it has no
// struct for. Discord keeps adding events (e.g. VOICE_CHANNEL_START_TIME_UPDATE, sent whenever a voice channel
// becomes occupied) that the bot has no use for, and each one would otherwise be logged with its full JSON payload.
const unknownEventPrefix = "unknown event:"

var installDiscordgoLoggerOnce sync.Once

// installDiscordgoLogger routes discordgo's logging through discordgoLogger. It is safe to call multiple times
// (once per shard); only the first call has an effect.
func installDiscordgoLogger() {
	installDiscordgoLoggerOnce.Do(func() {
		discordgo.Logger = discordgoLogger
	})
}

// discordgoLogger mirrors discordgo's default log format exactly, except that "unknown event" warnings are dropped.
// Session log levels are still respected: discordgo only calls this for messages at or below the session's LogLevel.
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

	msg := fmt.Sprintf(format, a...)

	log.Printf("[DG%d] %s:%d:%s() %s\n", msgL, file, line, name, msg)
}

func shouldDropDiscordgoMessage(msgL int, format string) bool {
	return msgL == discordgo.LogWarning && strings.HasPrefix(format, unknownEventPrefix)
}
