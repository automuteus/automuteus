package discord

import (
	"errors"
	"fmt"
	"strings"
)

func MentionByUserID(userID string) string {
	return fmt.Sprintf("<@!%s>", userID)
}

func MentionByChannelID(channelID string) string {
	return fmt.Sprintf("<#%s>", channelID)
}

func ExtractChannelIDFromText(mention string) (string, error) {
	// channel is formatted <#123456>
	if strings.HasPrefix(mention, "<#") && strings.HasSuffix(mention, ">") {
		err := ValidateSnowflake(mention[2 : len(mention)-1])
		if err == nil {
			return mention[2 : len(mention)-1], nil
		}
		return "", err
	} else {
		// if they just used the ID of the channel directly
		err := ValidateSnowflake(mention)
		if err == nil {
			return mention, nil
		}
	}
	return "", errors.New("channel text does not conform to the correct format (`<#roleid>` or `channelid`)")
}

func ExtractRoleIDFromText(mention string) (string, error) {
	// role is formatted <@&123456>
	if strings.HasPrefix(mention, "<@&") && strings.HasSuffix(mention, ">") {
		err := ValidateSnowflake(mention[3 : len(mention)-1])
		if err == nil {
			return mention[3 : len(mention)-1], nil
		}
		return "", err
	} else {
		// if they just used the ID of the role directly
		err := ValidateSnowflake(mention)
		if err == nil {
			return mention, nil
		}
	}
	return "", errors.New("role text does not conform to the correct format (`<@&roleid>` or `roleid`)")
}

func ExtractUserIDFromText(mention string) (string, error) {
	// nickname format
	switch {
	case strings.HasPrefix(mention, "<@!") && strings.HasSuffix(mention, ">"):
		err := ValidateSnowflake(mention[3 : len(mention)-1])
		if err == nil {
			return mention[3 : len(mention)-1], nil
		}
		return "", err
	case strings.HasPrefix(mention, "<@") && strings.HasSuffix(mention, ">"):
		err := ValidateSnowflake(mention[2 : len(mention)-1])
		if err == nil {
			return mention[2 : len(mention)-1], nil
		}
		return "", err
	default:
		err := ValidateSnowflake(mention)
		if err == nil {
			return mention, nil
		}
		return "", errors.New("mention does not conform to the correct format")
	}
}
