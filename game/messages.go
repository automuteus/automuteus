package game

import "encoding/json"

type GenericWSMessage struct {
	GuildID string          `json:"guildID"`
	Payload json.RawMessage `json:"payload"`
}
