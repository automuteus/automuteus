package discord

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
)

type PersistentGuildData struct {
	GuildID string `json:"guildID"`

	CommandPrefix         string `json:"commandPrefix"`
	DefaultTrackedChannel string `json:"defaultTrackedChannel"`

	AdminUserIDs          []string   `json:"adminIDs"`
	PermissionedRoleIDs   []string   `json:"permissionRoleIDs"`
	Delays                GameDelays `json:"delays"`
	VoiceRules            VoiceRules `json:"voiceRules"`
	ApplyNicknames        bool       `json:"applyNicknames"`
	UnmuteDeadDuringTasks bool       `json:"UnmuteDeadDuringTasks"`

	lock sync.RWMutex
}

func PGDDefault(id string) *PersistentGuildData {
	return &PersistentGuildData{
		GuildID:               id,
		CommandPrefix:         ".au",
		DefaultTrackedChannel: "",
		AdminUserIDs:          nil,
		PermissionedRoleIDs:   nil,
		Delays:                MakeDefaultDelays(),
		VoiceRules:            MakeMuteAndDeafenRules(),
		ApplyNicknames:        false,
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

func LoadPGDFromFile(filename string) (*PersistentGuildData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	jsonBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	pgd := PersistentGuildData{}
	err = json.Unmarshal(jsonBytes, &pgd)
	if err != nil {
		return nil, err
	}
	return &pgd, nil
}
