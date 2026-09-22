package bot

import (
	"fmt"
	"sync"
	"testing"

	"github.com/automuteus/automuteus/v8/internal/server"
	"github.com/prometheus/client_golang/prometheus"
)

func sharedGameMetrics(t *testing.T, count int) ([]*Bot, func(int)) {
	t.Helper()
	registry := prometheus.NewRegistry()
	metrics := server.NewMetrics(registry)
	shards := make([]*Bot, count)
	for i := range shards {
		shards[i] = &Bot{metrics: metrics, activeGameRequests: make(map[string]GameStateRequest)}
	}
	return shards, func(want int) {
		t.Helper()
		families, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		for _, family := range families {
			if family.GetName() == "automuteus_active_games" {
				if got := family.Metric[0].GetGauge().GetValue(); got != float64(want) {
					t.Fatalf("process active games = %v, want %d", got, want)
				}
				return
			}
		}
		t.Fatal("active games gauge missing")
	}
}

func TestActiveGamesMetricsSumAcrossShards(t *testing.T) {
	shards, check := sharedGameMetrics(t, 2)
	shards[0].trackGame(GameStateRequest{ConnectCode: "a"})
	shards[0].trackGame(GameStateRequest{ConnectCode: "b"})
	shards[1].trackGame(GameStateRequest{ConnectCode: "c"})
	check(3)
}

func TestActiveGamesMetricsRemovalPreservesOtherShards(t *testing.T) {
	shards, check := sharedGameMetrics(t, 2)
	shards[0].trackGame(GameStateRequest{ConnectCode: "a"})
	shards[0].trackGame(GameStateRequest{ConnectCode: "b"})
	shards[1].trackGame(GameStateRequest{ConnectCode: "c"})
	shards[1].untrackGame("c")
	check(2)
}

func TestActiveGamesMetricsRepeatedTrackingIsIdempotent(t *testing.T) {
	shards, check := sharedGameMetrics(t, 2)
	shards[0].trackGame(GameStateRequest{ConnectCode: "a"})
	shards[0].trackGame(GameStateRequest{ConnectCode: "a"})
	check(1)
	shards[1].untrackGame("missing")
	check(1)
	shards[0].untrackGame("a")
	shards[0].untrackGame("a")
	check(0)
}

func TestActiveGamesMetricsConcurrentShards(t *testing.T) {
	shards, check := sharedGameMetrics(t, 4)
	const gamesPerShard = 32
	for _, track := range []bool{true, false} {
		var wg sync.WaitGroup
		for _, shard := range shards {
			for i := 0; i < gamesPerShard; i++ {
				wg.Add(1)
				go func(shard *Bot, code string) {
					defer wg.Done()
					if track {
						shard.trackGame(GameStateRequest{ConnectCode: code})
						shard.trackGame(GameStateRequest{ConnectCode: code})
					} else {
						shard.untrackGame(code)
						shard.untrackGame(code)
					}
				}(shard, fmt.Sprint(i))
			}
		}
		wg.Wait()
		if track {
			check(len(shards) * gamesPerShard)
		} else {
			check(0)
		}
	}
}
