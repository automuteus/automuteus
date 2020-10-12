package storage

import (
	"encoding/json"
)

type PersistentGuildData struct {
	GuildID   string `json:"guildID"`
	GuildName string `json:"guildName"`

	GuildStats    GuildStats
	GuildSettings GuildSettings
}

func PGDDefault(id string, name string) *PersistentGuildData {
	return &PersistentGuildData{
		GuildID:   id,
		GuildName: name,

		GuildStats:    MakeGuildStats(),
		GuildSettings: MakeGuildSettings(),
	}
}

func FromData(data map[string]interface{}) (*PersistentGuildData, error) {
	var newPgd PersistentGuildData
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &newPgd)
	if err != nil {
		return nil, err
	}
	return &newPgd, nil
}

func (pgd *PersistentGuildData) ToData() (map[string]interface{}, error) {
	var data map[string]interface{}

	jsonBytes, err := json.Marshal(pgd)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonBytes, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}
