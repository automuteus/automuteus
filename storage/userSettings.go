package storage

import (
	"strings"
	"sync"
)

type UserSettings struct {
	UserID    string   `json:"userID"`
	UserName  string   `json:"username"`
	GameNames []string `json:"gameNames"`
}

type UserSettingsUpdate struct {
	UserID string
	Type   int
	Value  string
}

type UserSettingsCollection struct {
	users map[string]*UserSettings
	lock  sync.RWMutex
}

const (
	GAME_NAME = 0
)

func MakeUserSettingsCollection() *UserSettingsCollection {
	return &UserSettingsCollection{
		users: make(map[string]*UserSettings),
		lock:  sync.RWMutex{},
	}
}

func (usc *UserSettingsCollection) GetUser(userID string) (UserSettings, bool) {
	usc.lock.RLock()
	defer usc.lock.RUnlock()

	if v, ok := usc.users[userID]; ok {
		return *v, true
	}
	return UserSettings{}, false
}

func (usc *UserSettingsCollection) UpdateUser(userID string, settings UserSettings) {
	usc.lock.Lock()
	defer usc.lock.Unlock()

	usc.users[userID] = &settings
}

//TODO this is very inefficient. n^2 based on the number of users and their cached names
//probably better off to create a hashtable of the in-game names to the userIDs. This also guarantees a 1:1 mapping,
//UNLIKE this implementation!
func (usc *UserSettingsCollection) PairByName(name string) string {
	usc.lock.RLock()
	defer usc.lock.RUnlock()

	for id, s := range usc.users {
		if s.attemptPairingByMatchingNames(name) {
			return id
		}
	}
	return ""
}

func (us *UserSettings) attemptPairingByMatchingNames(name string) bool {
	name = strings.ReplaceAll(strings.ToLower(name), " ", "")
	for _, otherName := range us.GameNames {
		if strings.ReplaceAll(strings.ToLower(otherName), " ", "") == strings.ReplaceAll(strings.ToLower(name), " ", "") {
			return true
		}
	}
	return false
}
