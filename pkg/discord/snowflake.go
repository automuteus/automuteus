package discord

import (
	"errors"
	"strconv"
)

const DiscordEpoch = 1420070400000

func ValidateSnowflake(snowflake string) error {
	if snowflake == "" {
		return errors.New("empty string")
	}

	num, err := strconv.ParseUint(snowflake, 10, 64)
	if err != nil {
		return err
	}

	if num < DiscordEpoch {
		return errors.New("too small (prior to discord epoch)")
	}

	return nil
}
