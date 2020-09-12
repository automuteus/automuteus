package discord

import (
	"bytes"
	"fmt"
)

func helpResponse() string {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("Among Us Bot command reference:\n")
	buf.WriteString(fmt.Sprintf("`%s list` (`%s l`): Print the currently tracked players, and their in-game status (Beta).\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s help` (`%s h`): Print help info and command usage.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s add` (`%s a`): Add players to the tracked list (muted/unmuted throughout the game). Ex: `%s a @DiscordUser2 @DiscordUser1`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s track` (`%s t`): Tell Bot to use a single voice channel for mute/unmute, and ignore other players. Ex: `%s t Voice channel name`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	return buf.String()
}
