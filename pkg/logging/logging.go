// Package logging configures the process-wide structured logger.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Setup installs a slog handler writing to w as the default logger, and routes the standard library log package
// through it, so that existing log.Print* calls share the same format and destination.
//
// LOG_FORMAT selects "text" (default) or "json". LOG_LEVEL selects debug, info (default), warn, or error.
func Setup(w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: LevelFromEnv()}
	var handler slog.Handler
	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "json") {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// LevelFromEnv parses LOG_LEVEL, defaulting to info.
func LevelFromEnv() slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL"))); err != nil {
		return slog.LevelInfo
	}
	return level
}
