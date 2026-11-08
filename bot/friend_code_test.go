package bot

import (
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/amongus"
	"github.com/automuteus/automuteus/v8/pkg/game"
)

// These tests cover linking by Friend Code: a player's account follows them through name changes and rejoins, while a
// different account that takes their name is not mistaken for them.

const aliceCode = "alice#1111"

// addFriendCodeUser adds a Discord user linked to an alive, red player with the given name and Friend Code.
func addFriendCodeUser(dgs *GameState, userID, discordName, playerName, friendCode string) {
	dgs.GameData.PlayerData[playerName] = amongus.PlayerData{Color: game.Red, Name: playerName, IsAlive: true, FriendCode: friendCode}
	dgs.UserData[userID] = UserData{
		User:       User{UserID: userID, UserName: discordName},
		InGameName: playerName,
		FriendCode: friendCode,
	}
}

func TestFriendCode_RenameKeepsLink(t *testing.T) {
	s := newPlayerScenario(t, game.LOBBY, func(dgs *GameState) { addFriendCodeUser(dgs, "10", "someone", "alice", aliceCode) })

	// both Captures report a rename as the old name leaving and the new one joining
	s.send(game.Player{Action: game.LEFT, Name: "alice", Color: game.Red, FriendCode: aliceCode})
	got := s.send(game.Player{Action: game.JOINED, Name: "alicia", Color: game.Red, FriendCode: aliceCode})

	if got != "10" {
		t.Errorf("correlated user = %q, want 10", got)
	}
	st := s.state()
	if u := st.UserData["10"]; u.InGameName != "alicia" || u.FriendCode != aliceCode {
		t.Errorf("user 10 linked to %q (%q), want alicia (%s)", u.InGameName, u.FriendCode, aliceCode)
	}
	if p, ok := st.LinkedPlayer(st.UserData["10"]); !ok || p.Name != "alicia" {
		t.Errorf("linked player = %+v (found %v), want alicia", p, ok)
	}
}

func TestFriendCode_RememberedCodeLinksUnderNewName(t *testing.T) {
	cases := []struct {
		name string
		seed func(dgs *GameState)
	}{
		{
			name: "unlinked user",
			seed: func(dgs *GameState) { addDiscordUser(dgs, "10", "someone") },
		},
		{
			// left under an old name before Friend Codes were known, so their link doesn't carry the code
			name: "user still linked to a name that left",
			seed: func(dgs *GameState) {
				dgs.UserData["10"] = UserData{User: User{UserID: "10", UserName: "someone"}, InGameName: "oldname"}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newPlayerScenario(t, game.LOBBY, tc.seed)
			s.deps.store.mappings[scenarioGuild+":"+amongus.FriendCodeCacheKey(aliceCode)] = map[string]interface{}{"10": ""}

			got := s.send(game.Player{Action: game.JOINED, Name: "newname", Color: game.Blue, FriendCode: aliceCode})

			if got != "10" {
				t.Errorf("correlated user = %q, want 10", got)
			}
			if u := s.state().UserData["10"]; u.InGameName != "newname" || u.FriendCode != aliceCode {
				t.Errorf("user 10 linked to %q (%q), want newname (%s)", u.InGameName, u.FriendCode, aliceCode)
			}
		})
	}
}

func TestFriendCode_RememberedCodeDoesNotStealUserFromPresentPlayer(t *testing.T) {
	s := newPlayerScenario(t, game.LOBBY, func(dgs *GameState) { addFriendCodeUser(dgs, "10", "someone", "alt", "alt#2222") })
	s.deps.store.mappings[scenarioGuild+":"+amongus.FriendCodeCacheKey(aliceCode)] = map[string]interface{}{"10": ""}

	got := s.send(game.Player{Action: game.JOINED, Name: "alice2", Color: game.Blue, FriendCode: aliceCode})

	if got != "" {
		t.Errorf("correlated user = %q, want none", got)
	}
	if u := s.state().UserData["10"]; u.InGameName != "alt" {
		t.Errorf("user 10 moved to %q, want them left on alt", u.InGameName)
	}
}

func TestFriendCode_SameNameDifferentAccountIsNotLinked(t *testing.T) {
	s := newPlayerScenario(t, game.LOBBY, func(dgs *GameState) { addFriendCodeUser(dgs, "10", "someone", "alice", aliceCode) })
	// the guild remembers user 10 playing as alice
	s.deps.store.mappings[scenarioGuild+":alice"] = map[string]interface{}{"10": ""}

	s.send(game.Player{Action: game.LEFT, Name: "alice", Color: game.Red, FriendCode: aliceCode})
	if got := s.send(game.Player{Action: game.JOINED, Name: "alice", Color: game.Blue, FriendCode: "other#3333"}); got != "" {
		t.Errorf("correlated user = %q, want none: that's a different account", got)
	}

	st := s.state()
	if p, ok := st.LinkedPlayer(st.UserData["10"]); ok {
		t.Errorf("user 10 is linked to %+v, want nobody: that's a different account", p)
	}
	if u := st.UserData["10"]; u.FriendCode != aliceCode {
		t.Errorf("user 10's Friend Code = %q, want %s kept for when they return", u.FriendCode, aliceCode)
	}

	// the other account disconnecting mustn't unlink the user who had the name before
	s.send(game.Player{Action: game.DISCONNECTED, Name: "alice", Color: game.Blue, FriendCode: "other#3333", Disconnected: true})
	if u := s.state().UserData["10"]; u.InGameName != "alice" || u.FriendCode != aliceCode {
		t.Errorf("user 10 linked to %q (%q) after another account disconnected, want alice (%s)", u.InGameName, u.FriendCode, aliceCode)
	}
}

func TestFriendCode_LinkByNameRecordsCode(t *testing.T) {
	// linked before the Capture reported Friend Codes
	s := newPlayerScenario(t, game.LOBBY, func(dgs *GameState) { addLinkedUser(dgs, "10", "alice", true, false, false) })

	got := s.send(game.Player{Action: game.FORCEUPDATED, Name: "alice", Color: game.Red, FriendCode: aliceCode})

	if got != "10" {
		t.Errorf("correlated user = %q, want 10", got)
	}
	if u := s.state().UserData["10"]; u.FriendCode != aliceCode {
		t.Errorf("user 10's Friend Code = %q, want %s", u.FriendCode, aliceCode)
	}
}

func TestFriendCode_AccountLinkWinsOverNameLink(t *testing.T) {
	s := newPlayerScenario(t, game.LOBBY, func(dgs *GameState) {
		// user 10 is linked to the account, now playing as alicia; user 11 to the name alice from before Friend Codes
		addFriendCodeUser(dgs, "10", "someone", "alicia", aliceCode)
		dgs.UserData["11"] = UserData{User: User{UserID: "11", UserName: "other"}, InGameName: "alice"}
	})

	got := s.send(game.Player{Action: game.JOINED, Name: "alice", Color: game.Red, FriendCode: aliceCode})

	if got != "10" {
		t.Errorf("correlated user = %q, want 10", got)
	}
}

func TestFriendCode_DisconnectUnlinks(t *testing.T) {
	s := newPlayerScenario(t, game.LOBBY, func(dgs *GameState) { addFriendCodeUser(dgs, "10", "someone", "alice", aliceCode) })

	s.send(game.Player{Action: game.DISCONNECTED, Name: "alice", Color: game.Red, FriendCode: aliceCode, Disconnected: true})

	if u := s.state().UserData["10"]; u.InGameName != amongus.UnlinkedPlayerName || u.FriendCode != "" {
		t.Errorf("user 10 linked to %q (%q), want unlinked", u.InGameName, u.FriendCode)
	}
}

func TestFriendCode_DiscordNameMatchKeepsUserOnTheirAccount(t *testing.T) {
	s := newPlayerScenario(t, game.LOBBY, func(dgs *GameState) { addFriendCodeUser(dgs, "10", "alice", "alice", aliceCode) })

	s.send(game.Player{Action: game.LEFT, Name: "alice", Color: game.Red, FriendCode: aliceCode})
	got := s.send(game.Player{Action: game.JOINED, Name: "alice", Color: game.Blue, FriendCode: "other#3333"})

	if got != "" {
		t.Errorf("correlated user = %q, want none", got)
	}
	if u := s.state().UserData["10"]; u.FriendCode != aliceCode {
		t.Errorf("user 10's Friend Code = %q, want %s", u.FriendCode, aliceCode)
	}
}
