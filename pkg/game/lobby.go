package game

type Lobby struct {
	LobbyCode string  `json:"LobbyCode"`
	Region    Region  `json:"Region"`
	PlayMap   PlayMap `json:"Map"`
}
