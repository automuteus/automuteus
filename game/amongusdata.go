package game

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

//TODO make this private?
// AmongUserData struct
type PlayerData struct {
	Color   int
	Name    string
	IsAlive bool
}

// ToString a user
func (auData *PlayerData) ToString() string {
	return fmt.Sprintf("{ Name: %s, Color: %s, Alive: %v }\n", auData.Name, GetColorStringForInt(auData.Color), auData.IsAlive)
}

func (auData *PlayerData) isDifferent(player Player) bool {
	return auData.IsAlive != !player.IsDead || auData.Color != player.Color || auData.Name != player.Name
}

type AmongUsData struct {
	//indexed by amongusname
	playerData map[string]*PlayerData
	//what current phase the game is in (lobby, tasks, discussion)
	phase  Phase
	room   string
	region string

	lock sync.RWMutex
}

func NewAmongUsData() AmongUsData {
	return AmongUsData{
		playerData: map[string]*PlayerData{},
		phase:      MENU,
		room:       "",
		region:     "",
		lock:       sync.RWMutex{},
	}
}

func (auData *AmongUsData) SetRoomRegion(room, region string) {
	auData.lock.Lock()
	auData.room = room
	auData.region = region
	auData.lock.Unlock()
}

func (auData *AmongUsData) GetRoomRegion() (string, string) {
	auData.lock.RLock()
	defer auData.lock.RUnlock()
	return auData.room, auData.region
}

func (auData *AmongUsData) SetAllAlive() {
	auData.lock.Lock()
	for i, v := range auData.playerData {
		v.IsAlive = true
		auData.playerData[i] = v
	}
	auData.lock.Unlock()
}

func (auData *AmongUsData) SetPhase(phase Phase) {
	auData.lock.Lock()
	auData.phase = phase
	auData.lock.Unlock()
}

func (auData *AmongUsData) NumDetectedPlayers() int {
	auData.lock.RLock()
	defer auData.lock.RUnlock()
	return len(auData.playerData)
}

func (auData *AmongUsData) GetPhase() Phase {
	auData.lock.RLock()
	defer auData.lock.RUnlock()
	return auData.phase
}

func (auData *AmongUsData) ClearPlayerData(name string) {
	auData.lock.Lock()
	delete(auData.playerData, name)
	auData.lock.Unlock()
}

func (auData *AmongUsData) ClearAllPlayerData() {
	auData.lock.Lock()
	auData.playerData = map[string]*PlayerData{}
	auData.lock.Unlock()
}

func (auData *AmongUsData) ApplyPlayerUpdate(update Player) (bool, bool) {
	auData.lock.Lock()
	defer auData.lock.Unlock()

	if _, ok := auData.playerData[update.Name]; !ok {
		auData.playerData[update.Name] = &PlayerData{
			Color:   update.Color,
			Name:    update.Name,
			IsAlive: !update.IsDead,
		}
		log.Printf("Added new player instance for %s\n", update.Name)
		return true, false
	}
	guildDataTempPtr := auData.playerData[update.Name]
	isUpdate := guildDataTempPtr.isDifferent(update)
	isAliveUpdate := (*auData.playerData[update.Name]).IsAlive != !update.IsDead
	if isUpdate {
		(*auData.playerData[update.Name]).Color = update.Color
		(*auData.playerData[update.Name]).Name = update.Name
		(*auData.playerData[update.Name]).IsAlive = !update.IsDead
		log.Printf("Updated %s", (*auData.playerData[update.Name]).ToString())
	}

	return isUpdate, isAliveUpdate
}

func (auData *AmongUsData) NameColorMappings() map[string]int {
	ret := make(map[string]int)
	auData.lock.RLock()
	for i, v := range auData.playerData {
		ret[i] = v.Color
	}
	auData.lock.RUnlock()
	return ret
}
func (auData *AmongUsData) NameAliveMappings() map[string]bool {
	ret := make(map[string]bool)
	auData.lock.RLock()
	for i, v := range auData.playerData {
		ret[i] = v.IsAlive
	}
	auData.lock.RUnlock()
	return ret
}

func (auData *AmongUsData) GetByColor(text string) *PlayerData {
	text = strings.ToLower(text)
	auData.lock.RLock()
	defer auData.lock.RUnlock()

	for _, playerData := range auData.playerData {
		if GetColorStringForInt(playerData.Color) == text {
			return playerData
		}
	}
	return nil
}

func (auData *AmongUsData) GetByName(text string) *PlayerData {
	text = strings.ToLower(text)
	auData.lock.RLock()
	defer auData.lock.RUnlock()

	for _, playerData := range auData.playerData {
		if strings.ReplaceAll(strings.ToLower(playerData.Name), " ", "") == strings.ReplaceAll(strings.ToLower(text), " ", "") {
			return playerData
		}
	}
	return nil
}
