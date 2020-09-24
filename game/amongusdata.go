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
	PlayerData map[string]*PlayerData
	//what current phase the game is in (lobby, tasks, discussion)
	GamePhase       Phase
	Room            string
	Region          string
	AmongUsDataLock sync.RWMutex

	lock sync.RWMutex
}

func NewAmongUsData() AmongUsData {
	return AmongUsData{
		PlayerData:      map[string]*PlayerData{},
		GamePhase:       LOBBY,
		Room:            "",
		Region:          "",
		AmongUsDataLock: sync.RWMutex{},
		lock:            sync.RWMutex{},
	}
}

func (auData *AmongUsData) SetRoomRegion(room, region string) {
	auData.lock.Lock()
	auData.Room = room
	auData.Region = region
	auData.lock.Unlock()
}

func (auData *AmongUsData) GetRoomRegion() (string, string) {
	auData.lock.RLock()
	defer auData.lock.RUnlock()

	return auData.Room, auData.Region
}

func (auData *AmongUsData) SetAllAlive() {
	auData.lock.Lock()
	for i, v := range auData.PlayerData {
		v.IsAlive = true
		auData.PlayerData[i] = v
	}
	auData.lock.Unlock()
}

func (auData *AmongUsData) SetPhase(phase Phase) {
	auData.lock.Lock()
	auData.GamePhase = phase
	auData.lock.Unlock()
}
func (auData *AmongUsData) GetPhase() Phase {
	auData.lock.RLock()
	defer auData.lock.RUnlock()
	return auData.GamePhase
}

func (auData *AmongUsData) ApplyPlayerUpdate(update Player) (bool, bool) {
	auData.lock.Lock()
	defer auData.lock.Unlock()

	if _, ok := auData.PlayerData[update.Name]; !ok {
		auData.PlayerData[update.Name] = &PlayerData{
			Color:   update.Color,
			Name:    update.Name,
			IsAlive: !update.IsDead,
		}
		log.Printf("Added new player instance for %s\n", update.Name)
		return true, false
	}
	guildDataTempPtr := auData.PlayerData[update.Name]
	isUpdate := guildDataTempPtr.isDifferent(update)
	isAliveUpdate := (*auData.PlayerData[update.Name]).IsAlive != !update.IsDead
	if isUpdate {
		(*auData.PlayerData[update.Name]).Color = update.Color
		(*auData.PlayerData[update.Name]).Name = update.Name
		(*auData.PlayerData[update.Name]).IsAlive = !update.IsDead

		log.Printf("Updated %s", (*auData.PlayerData[update.Name]).ToString())
	}

	return isUpdate, isAliveUpdate
}

func (auData *AmongUsData) GetByColor(text string) *PlayerData {
	text = strings.ToLower(text)
	auData.lock.RLock()
	defer auData.lock.RUnlock()

	for _, playerData := range auData.PlayerData {
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

	for _, playerData := range auData.PlayerData {
		if strings.ReplaceAll(strings.ToLower(playerData.Name), " ", "") == text {
			return playerData
		}
	}
	return nil
}
