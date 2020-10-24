package game

import (
	"log"
	"strings"
	"sync"
)

type AmongUsData struct {
	//indexed by amongusname
	PlayerData map[string]PlayerData `json:"playerData"`

	Phase  Phase  `json:"phase"`
	Room   string `json:"room"`
	Region string `json:"region"`

	lock sync.RWMutex
}

func NewAmongUsData() AmongUsData {
	return AmongUsData{
		PlayerData: map[string]PlayerData{},
		Phase:      MENU,
		Room:       "",
		Region:     "",
		lock:       sync.RWMutex{},
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

func (auData *AmongUsData) UpdatePhase(phase Phase) (old Phase) {
	auData.lock.Lock()
	old = auData.Phase
	auData.Phase = phase
	auData.lock.Unlock()

	if old != phase {
		if phase == LOBBY || (phase == TASKS && old == LOBBY) {
			auData.SetAllAlive()
		} else if phase == MENU {
			auData.SetRoomRegion("", "")
		}
	}
	return old
}

func (auData *AmongUsData) UpdatePlayer(player Player) (updated, isAliveUpdated bool) {
	auData.lock.RLock()
	phase := auData.Phase
	auData.lock.RUnlock()

	if phase == LOBBY && player.IsDead {
		player.IsDead = false
	}
	if player.Action == EXILED {
		player.IsDead = true
	}

	return auData.applyPlayerUpdate(player)
}

func (auData *AmongUsData) NumDetectedPlayers() int {
	auData.lock.RLock()
	defer auData.lock.RUnlock()
	return len(auData.PlayerData)
}

func (auData *AmongUsData) GetPhase() Phase {
	auData.lock.RLock()
	defer auData.lock.RUnlock()
	return auData.Phase
}

func (auData *AmongUsData) ClearPlayerData(name string) {
	auData.lock.Lock()
	delete(auData.PlayerData, name)
	auData.lock.Unlock()
}

func (auData *AmongUsData) ClearAllPlayerData() {
	auData.lock.Lock()
	auData.PlayerData = map[string]PlayerData{}
	auData.lock.Unlock()
}

func (auData *AmongUsData) applyPlayerUpdate(update Player) (bool, bool) {
	auData.lock.Lock()
	defer auData.lock.Unlock()

	if _, ok := auData.PlayerData[update.Name]; !ok {
		auData.PlayerData[update.Name] = PlayerData{
			Color:   update.Color,
			Name:    update.Name,
			IsAlive: !update.IsDead,
		}
		log.Printf("Added new player instance for %s\n", update.Name)
		return true, false
	}
	playerData := auData.PlayerData[update.Name]
	isUpdate := playerData.isDifferent(update)
	isAliveUpdate := auData.PlayerData[update.Name].IsAlive != !update.IsDead
	if isUpdate {
		p := PlayerData{
			Color:   update.Color,
			Name:    update.Name,
			IsAlive: !update.IsDead,
		}
		auData.PlayerData[update.Name] = p
		log.Printf("Updated %s", p.ToString())
	}

	return isUpdate, isAliveUpdate
}

func (auData *AmongUsData) NameColorMappings() map[string]int {
	ret := make(map[string]int)
	auData.lock.RLock()
	for i, v := range auData.PlayerData {
		ret[i] = v.Color
	}
	auData.lock.RUnlock()
	return ret
}
func (auData *AmongUsData) NameAliveMappings() map[string]bool {
	ret := make(map[string]bool)
	auData.lock.RLock()
	for i, v := range auData.PlayerData {
		ret[i] = v.IsAlive
	}
	auData.lock.RUnlock()
	return ret
}

func (auData *AmongUsData) GetByColor(text string) (PlayerData, bool) {
	text = strings.ToLower(text)
	auData.lock.RLock()
	defer auData.lock.RUnlock()

	for _, playerData := range auData.PlayerData {
		if GetColorStringForInt(playerData.Color) == text {
			return playerData, true
		}
	}
	return PlayerData{}, false
}

func (auData *AmongUsData) GetByName(text string) (PlayerData, bool) {
	text = strings.ToLower(text)
	auData.lock.RLock()
	defer auData.lock.RUnlock()

	for _, playerData := range auData.PlayerData {
		if strings.ReplaceAll(strings.ToLower(playerData.Name), " ", "") == strings.ReplaceAll(strings.ToLower(text), " ", "") {
			return playerData, true
		}
	}
	return PlayerData{}, false
}
