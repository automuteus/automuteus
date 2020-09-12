package discord

import (
	"bytes"
	"fmt"
)

func helpResponse() string {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("Among Us Bot command reference:\n")
	buf.WriteString(fmt.Sprintf("`%s h`: Display this help message\n", CommandPrefix))
	return buf.String()
}
