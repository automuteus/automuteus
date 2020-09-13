package discord

import (
	"bytes"
	"fmt"
)

func helpResponse() string {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("Among Us Bot command reference:\n")
	buf.WriteString(fmt.Sprintf("`%s help` (`%s h`): Print help info and command usage.\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s list` (`%s l`): Print the currently tracked players, and their in-game status (Beta).\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s dead` (`%s d`): Mark a user as dead so they aren't unmuted during discussions. Ex: `%s d @DiscordUser1 @DiscordUser2`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s track` (`%s t`): Tell Bot to use a single voice channel for mute/unmute, and ignore other players. Ex: `%s t Voice channel name`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s bcast` (`%s b`): Tell Bot to broadcast the room code and region. Ex: `%s b ABCD asia` or `%s b ABCD na`\n", CommandPrefix, CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s add` (`%s a`): Manually add players to the tracked list (muted/unmuted throughout the game). Ex: `%s a @DiscordUser2 @DiscordUser1`\n", CommandPrefix, CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s reset` (`%s r`): Reset the tracked player list manually (mainly for debug)\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s muteall` (`%s ma`): Forcibly mute ALL users (mainly for debug).\n", CommandPrefix, CommandPrefix))
	buf.WriteString(fmt.Sprintf("`%s unmuteall` (`%s ua`): Forcibly unmute ALL users (mainly for debug).\n", CommandPrefix, CommandPrefix))
	return buf.String()
}
