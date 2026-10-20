package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type config struct {
	discordToken string
	guildID      string
	forumID      string
	staffRoleIDs []string
	solvedTag    string // tag names; IDs are resolved from the forum at startup
	humanTag     string

	model  string
	effort string

	dailyBudget    float64
	threadsPerUser int
	maxReplies     int
	minChars       int
	minMemberAge   time.Duration
	debounce       time.Duration

	port string
}

func configFromEnv(getenv func(string) string) (config, error) {
	c := config{
		discordToken: getenv("HELPBOT_DISCORD_TOKEN"),
		guildID:      getenv("HELPBOT_GUILD_ID"),
		forumID:      getenv("HELPBOT_FORUM_CHANNEL_ID"),
		solvedTag:    or(getenv("HELPBOT_SOLVED_TAG"), "Solved"),
		humanTag:     or(getenv("HELPBOT_NEEDS_HUMAN_TAG"), "Needs human"),
		model:        or(getenv("HELPBOT_MODEL"), "claude-sonnet-5-5"),
		effort:       or(getenv("HELPBOT_EFFORT"), "low"),
		port:         or(getenv("HELPBOT_PORT"), "8080"),
	}
	for _, id := range strings.Split(getenv("HELPBOT_STAFF_ROLE_IDS"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			c.staffRoleIDs = append(c.staffRoleIDs, id)
		}
	}
	for key, value := range map[string]string{
		"HELPBOT_DISCORD_TOKEN": c.discordToken, "HELPBOT_GUILD_ID": c.guildID, "HELPBOT_FORUM_CHANNEL_ID": c.forumID,
		"ANTHROPIC_API_KEY": getenv("ANTHROPIC_API_KEY"),
	} {
		if value == "" {
			return c, fmt.Errorf("%s is required", key)
		}
	}
	if len(c.staffRoleIDs) == 0 {
		return c, errors.New("HELPBOT_STAFF_ROLE_IDS is required (comma-separated role IDs whose posts silence the bot)")
	}
	var err error
	if c.dailyBudget, err = floatEnv(getenv, "HELPBOT_DAILY_BUDGET_USD", 5); err != nil {
		return c, err
	}
	ints := []struct {
		key      string
		into     *int
		fallback int
	}{
		{"HELPBOT_THREADS_PER_USER_PER_DAY", &c.threadsPerUser, 3},
		{"HELPBOT_MAX_REPLIES", &c.maxReplies, 5},
		{"HELPBOT_MIN_CHARS", &c.minChars, 20},
	}
	for _, i := range ints {
		if *i.into, err = intEnv(getenv, i.key, i.fallback); err != nil {
			return c, err
		}
	}
	if c.minMemberAge, err = durationEnv(getenv, "HELPBOT_MIN_MEMBER_AGE", 10*time.Minute); err != nil {
		return c, err
	}
	if c.debounce, err = durationEnv(getenv, "HELPBOT_DEBOUNCE", 5*time.Second); err != nil {
		return c, err
	}
	return c, nil
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func intEnv(getenv func(string) string, key string, fallback int) (int, error) {
	if v := getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("%s must be a non-negative integer", key)
		}
		return n, nil
	}
	return fallback, nil
}

func floatEnv(getenv func(string) string, key string, fallback float64) (float64, error) {
	if v := getenv(key); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("%s must be a non-negative number", key)
		}
		return n, nil
	}
	return fallback, nil
}

func durationEnv(getenv func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	if v := getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 {
			return 0, fmt.Errorf("%s must be a duration like 10m", key)
		}
		return d, nil
	}
	return fallback, nil
}
