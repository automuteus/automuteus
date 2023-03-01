package amongus

type LinkStatus int

const (
	LinkSuccess LinkStatus = iota
	LinkNoPlayer
	LinkNoGameData
)

type UnlinkStatus int

const (
	UnlinkSuccess UnlinkStatus = iota
	UnlinkNoPlayer
)
