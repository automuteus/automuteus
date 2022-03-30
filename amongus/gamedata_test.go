package amongus

import (
	"github.com/automuteus/utils/pkg/game"
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
