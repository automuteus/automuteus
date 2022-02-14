package common

import (
	"errors"
	"strconv"
	"strings"
)

// TODO move to utils

func ExtractRoleIDFromText(mention string) (string, error) {
	// role is formatted <@&123456>
	if strings.HasPrefix(mention, "<@&") && strings.HasSuffix(mention, ">") {
		return mention[3 : len(mention)-1], nil
	} else {
		// if they just used the ID of the role directly
		// TODO snowflake validation
		_, err := strconv.ParseInt(mention, 10, 64)
		if err == nil {
			return mention, nil
		}
	}
	return "", errors.New("role text does not conform to the correct format (`<@&roleid>` or `roleid`)")
}

func ExtractUserIDFromMention(mention string) (string, error) {
	// TODO snowflake validation?
	// nickname format
	switch {
	case strings.HasPrefix(mention, "<@!") && strings.HasSuffix(mention, ">"):
		return mention[3 : len(mention)-1], nil
	case strings.HasPrefix(mention, "<@") && strings.HasSuffix(mention, ">"):
		return mention[2 : len(mention)-1], nil
	default:
		_, err := strconv.ParseInt(mention, 10, 64)
		if err == nil {
			return mention, nil
		}
		return "", errors.New("mention does not conform to the correct format")
	}
}
