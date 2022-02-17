package command

import "github.com/bwmarrin/discordgo"

var All = []discordgo.ApplicationCommand{
	Help,
	Info,
	Link,
	Unlink,
}
