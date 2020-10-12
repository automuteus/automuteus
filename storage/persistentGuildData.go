package storage

import (
	"encoding/json"
	"github.com/denverquane/amongusdiscord/game"
	"os"
	"sync"
)

type PersistentGuildData struct {
	GuildID   string `json:"guildID"`
	GuildName string `json:"guildName"`

	CommandPrefix         string `json:"commandPrefix"`
	DefaultTrackedChannel string `json:"defaultTrackedChannel"`

	AdminUserIDs          []string        `json:"adminIDs"`
	PermissionedRoleIDs   []string        `json:"permissionRoleIDs"`
	Delays                game.GameDelays `json:"delays"`
	VoiceRules            game.VoiceRules `json:"voiceRules"`
	ApplyNicknames        bool            `json:"applyNicknames"`
	UnmuteDeadDuringTasks bool            `json:"unmuteDeadDuringTasks"`

	lock sync.RWMutex
}

func PGDDefault(id string, name string) *PersistentGuildData {
	return &PersistentGuildData{
		GuildID:               id,
		GuildName:             name,
		CommandPrefix:         ".au",
		DefaultTrackedChannel: "",
		AdminUserIDs:          []string{},
		PermissionedRoleIDs:   []string{},
		Delays:                game.MakeDefaultDelays(),
		VoiceRules:            game.MakeMuteAndDeafenRules(),
		ApplyNicknames:        false,
		UnmuteDeadDuringTasks: false,
		lock:                  sync.RWMutex{},
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

func (pgd *PersistentGuildData) ToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	jsonBytes, err := json.MarshalIndent(pgd, "", "    ")
	if err != nil {
		return err
	}

	_, err = file.Write(jsonBytes)
	return err
}
