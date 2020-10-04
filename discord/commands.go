package discord

import "strings"

type CommandType int

const (
	Help CommandType = iota
	Track
	Link
	Unlink
	New
	End
	Force
	Refresh
	Settings
	Null
)

var CommandTypeStringMapping = map[string]CommandType{
	"help":     Help,
	"track":    Track,
	"link":     Link,
	"unlink":   Unlink,
	"new":      New,
	"end":      End,
	"force":    Force,
	"refresh":  Refresh,
	"settings": Settings,
	"":         Null,
}

func GetCommandType(arg string) CommandType {
	for str, cmd := range CommandTypeStringMapping {
		if len(arg) == 1 && cmd != Null {
			if str[0] == arg[0] {
				return cmd
			}
		} else {
			if strings.ToLower(arg) == str {
				return cmd
			}
		}
	}
	return Null
}
