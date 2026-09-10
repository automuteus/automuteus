package bot

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/automuteus/automuteus/v8/pkg/amongus"
	"github.com/automuteus/automuteus/v8/pkg/game"
)

const gameStateFixturePath = "testdata/game_state_v8.json"

// jsonShape reduces decoded JSON to its structure: objects become a map of key -> child shape, arrays become "[]",
// and scalars become ".". Comparing shapes catches renamed, added, or removed fields without caring about values.
func jsonShape(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, child := range val {
			out[k] = jsonShape(child)
		}
		return out
	case []interface{}:
		return "[]"
	default:
		return "."
	}
}

func loadGameStateFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(gameStateFixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	return data
}

// TestGameState_DecodeV8Fixture decodes a game state blob exactly the way RedisInterface.getDiscordGameState does
// (into a zero-value struct) and checks every field made it through. GameState is shared between shards via
// Redis, so a field rename here breaks games in progress across a rolling deploy.
func TestGameState_DecodeV8Fixture(t *testing.T) {
	var dgs GameState
	if err := json.Unmarshal(loadGameStateFixture(t), &dgs); err != nil {
		t.Fatalf("failed to decode fixture: %v", err)
	}

	if dgs.GuildID != "123456789012345678" {
		t.Errorf("GuildID = %q", dgs.GuildID)
	}
	if dgs.ConnectCode != "ABCDEFGH" {
		t.Errorf("ConnectCode = %q", dgs.ConnectCode)
	}
	if !dgs.Linked || !dgs.Running || !dgs.Subscribed {
		t.Errorf("flags: linked=%v running=%v subscribed=%v, want all true", dgs.Linked, dgs.Running, dgs.Subscribed)
	}
	if dgs.MatchID != 4242 || dgs.MatchStartUnix != 1700000000 {
		t.Errorf("MatchID=%d MatchStartUnix=%d", dgs.MatchID, dgs.MatchStartUnix)
	}
	if dgs.VoiceChannel != "333333333333333333" {
		t.Errorf("VoiceChannel = %q", dgs.VoiceChannel)
	}

	// user data: note the JSON keys are capitalized and InGameName is stored as "PlayerName"
	if got := len(dgs.UserData); got != 2 {
		t.Fatalf("UserData has %d entries, want 2", got)
	}
	soup, err := dgs.GetUser("111111111111111111")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if soup.GetNickName() != "Soup" || soup.GetUserName() != "soupdev" || soup.GetID() != "111111111111111111" {
		t.Errorf("Soup user = %+v", soup.User)
	}
	if !soup.ShouldBeMute || !soup.ShouldBeDeaf {
		t.Errorf("Soup should be mute+deaf, got mute=%v deaf=%v", soup.ShouldBeMute, soup.ShouldBeDeaf)
	}
	if soup.GetPlayerName() != "Soup" {
		t.Errorf("Soup InGameName = %q", soup.GetPlayerName())
	}
	bot, err := dgs.GetUser("222222222222222222")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if bot.GetPlayerName() != amongus.UnlinkedPlayerName {
		t.Errorf("music bot InGameName = %q, want unlinked", bot.GetPlayerName())
	}
	if got := dgs.GetCountLinked(); got != 1 {
		t.Errorf("GetCountLinked = %d, want 1", got)
	}

	// game state message
	if !dgs.GameStateMsg.Exists() {
		t.Error("GameStateMsg should exist")
	}
	if dgs.GameStateMsg.MessageID != "444444444444444444" || dgs.GameStateMsg.MessageChannelID != "555555555555555555" {
		t.Errorf("GameStateMsg = %+v", dgs.GameStateMsg)
	}
	if dgs.GameStateMsg.LeaderID != "111111111111111111" || dgs.GameStateMsg.CreationTimeUnix != 1700000001 {
		t.Errorf("GameStateMsg = %+v", dgs.GameStateMsg)
	}

	// among us data
	if got := dgs.GameData.GetPhase(); got != game.TASKS {
		t.Errorf("phase = %d, want TASKS", got)
	}
	room, region, playMap := dgs.GameData.GetRoomRegionMap()
	if room != "ABCDEF" || region != "North America" || playMap != game.POLUS {
		t.Errorf("room/region/map = %q/%q/%d", room, region, playMap)
	}
	if got := dgs.GameData.GetNumDetectedPlayers(); got != 2 {
		t.Fatalf("detected players = %d, want 2", got)
	}
	if p, ok := dgs.GameData.GetByName("Soup"); !ok || p.IsAlive || p.Color != game.Cyan {
		t.Errorf("Soup player data = %+v (found=%v)", p, ok)
	}
	if p, ok := dgs.GameData.GetByName("Dev"); !ok || !p.IsAlive || p.Color != game.Red {
		t.Errorf("Dev player data = %+v (found=%v)", p, ok)
	}
}

// TestGameState_JSONShapeMatchesFixture fails when a field is added to, removed from, or renamed on GameState
// (or UserData, GameStateMessage, amongus.GameData) without updating the fixture. That is deliberate: the blob
// lives in Redis and is read by every shard, so a shape change needs a compatibility plan.
func TestGameState_JSONShapeMatchesFixture(t *testing.T) {
	var fixture interface{}
	if err := json.Unmarshal(loadGameStateFixture(t), &fixture); err != nil {
		t.Fatalf("failed to decode fixture: %v", err)
	}

	// build a fresh state with one user and one player so the nested shapes are populated
	dgs := NewDiscordGameState("1")
	dgs.UserData["2"] = UserData{User: User{UserID: "2"}}
	dgs.GameData.UpdatePlayer(game.Player{Name: "p", Color: game.Red})
	encoded, err := json.Marshal(dgs)
	if err != nil {
		t.Fatalf("failed to encode game state: %v", err)
	}
	var current interface{}
	if err := json.Unmarshal(encoded, &current); err != nil {
		t.Fatalf("failed to decode encoded game state: %v", err)
	}

	// the user/player maps are keyed by ID/name, so compare one representative entry from each
	normalize := func(v interface{}) interface{} {
		shape := jsonShape(v).(map[string]interface{})
		for _, mapKey := range []string{"userData"} {
			if m, ok := shape[mapKey].(map[string]interface{}); ok {
				shape[mapKey] = firstValue(m)
			}
		}
		if au, ok := shape["amongUsData"].(map[string]interface{}); ok {
			if m, ok := au["playerData"].(map[string]interface{}); ok {
				au["playerData"] = firstValue(m)
			}
		}
		return shape
	}

	if want, got := normalize(fixture), normalize(current); !reflect.DeepEqual(want, got) {
		t.Errorf("GameState JSON shape changed.\n  fixture: %v\n  current: %v\nUpdate %s if this change is intentional, and consider games in progress during a rolling deploy.",
			want, got, gameStateFixturePath)
	}
}

func firstValue(m map[string]interface{}) interface{} {
	for _, v := range m {
		return v
	}
	return nil
}

// A decode -> encode -> decode cycle must be lossless, since the state is rewritten to Redis on every event.
func TestGameState_RoundTrip(t *testing.T) {
	var dgs GameState
	if err := json.Unmarshal(loadGameStateFixture(t), &dgs); err != nil {
		t.Fatalf("failed to decode fixture: %v", err)
	}
	encoded, err := json.Marshal(&dgs)
	if err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	var want, got map[string]interface{}
	if err := json.Unmarshal(loadGameStateFixture(t), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("round trip altered the game state.\n  want: %v\n  got:  %v", want, got)
	}
}

// Reset must clear everything except the guild ID, since the same GameState is reused across games in a guild.
func TestGameState_ResetKeepsGuildID(t *testing.T) {
	var dgs GameState
	if err := json.Unmarshal(loadGameStateFixture(t), &dgs); err != nil {
		t.Fatalf("failed to decode fixture: %v", err)
	}
	dgs.Reset()

	fresh := NewDiscordGameState("123456789012345678")
	if !reflect.DeepEqual(&dgs, fresh) {
		t.Errorf("Reset state differs from a fresh state.\n  reset: %+v\n  fresh: %+v", dgs, *fresh)
	}
}
