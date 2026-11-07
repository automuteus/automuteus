package amongus

import (
	"github.com/automuteus/automuteus/v8/pkg/game"
	"testing"
)

func TestGameData_UpdatePhase(t *testing.T) {
	gd := NewGameData()
	if old := gd.UpdatePhase(game.MENU); old != game.MENU || gd.Phase != game.MENU {
		t.Error("Expected MENU->MENU transition to be a no-op")
	}
	gd.PlayerData["name"] = PlayerData{
		Color:   game.Red,
		Name:    "name",
		IsAlive: false,
	}
	if old := gd.UpdatePhase(game.LOBBY); old != game.MENU || gd.Phase != game.LOBBY {
		t.Error("Expected MENU->LOBBY transition to change the phase")
	}

	if old := gd.UpdatePhase(game.TASKS); old != game.LOBBY || gd.Phase != game.TASKS {
		t.Error("Expected LOBBY->TASKS transition to change the phase")
	}

	gd.SetRoomRegionMap("A", "US", game.SKELD)

	if !gd.PlayerData["name"].IsAlive {
		t.Error("Player was not set as alive when transitioning from LOBBY->TASKS (game started)")
	}

	if old := gd.UpdatePhase(game.MENU); old != game.TASKS || gd.Phase != game.MENU {
		t.Error("Expected TASKS->MENU transition to change the phase")
	}
	if gd.Room != "" || gd.Region != "" || gd.Map != game.EMPTYMAP {
		t.Error("GameData was not reset properly when transitioning from TASKS->MENU")
	}
}

func TestGameData_FriendCode(t *testing.T) {
	gd := NewGameData()
	gd.UpdatePhase(game.LOBBY)
	if _, _, data := gd.UpdatePlayer(game.Player{Action: game.JOINED, Name: "alice", Color: game.Red, FriendCode: "alice#1111"}); data.FriendCode != "alice#1111" {
		t.Errorf("stored Friend Code = %q, want alice#1111", data.FriendCode)
	}
	if p, ok := gd.GetByFriendCode("alice#1111"); !ok || p.Name != "alice" {
		t.Errorf("GetByFriendCode = %+v (found %v), want alice", p, ok)
	}
	if _, ok := gd.GetByFriendCode(""); ok {
		t.Error("an empty Friend Code matched a player")
	}

	gd.PlayerData["bob"] = PlayerData{Color: game.Blue, Name: "bob", IsAlive: true}
	if updated, _, _ := gd.UpdatePlayer(game.Player{Action: game.FORCEUPDATED, Name: "bob", Color: game.Blue, FriendCode: "bob#2222"}); !updated {
		t.Error("learning a player's Friend Code wasn't an update")
	}
}
