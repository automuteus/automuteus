package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// counterValues gathers one counter family and returns its values keyed by the value of its single label.
func counterValues(t *testing.T, registry *prometheus.Registry, family, label string) map[string]float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != family {
			continue
		}
		if f.GetType().String() != "COUNTER" {
			t.Fatalf("%s is a %s, want a counter", family, f.GetType())
		}
		counts := make(map[string]float64)
		for _, metric := range f.Metric {
			labels := metric.GetLabel()
			if len(labels) != 1 || labels[0].GetName() != label {
				t.Fatalf("%s must have only a %s label, got %v", family, label, labels)
			}
			counts[labels[0].GetValue()] = metric.GetCounter().GetValue()
		}
		return counts
	}
	t.Fatalf("metric family %s not registered", family)
	return nil
}

func operationCounts(t *testing.T, registry *prometheus.Registry) map[string]float64 {
	t.Helper()
	return counterValues(t, registry, "automuteus_discord_operations_total", "type")
}

func TestMetricsConcurrentRecordingAndProcessIsolation(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	otherRegistry := prometheus.NewRegistry()
	NewMetrics(otherRegistry)

	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 100; j++ {
				metrics.RecordDiscordRequests(MessageEdit, 3)
				metrics.RecordDiscordRequests(MessageCreateDelete, 1)
			}
		}()
	}
	workers.Wait()
	metrics.RecordDiscordRequests(MessageEdit, 0)
	metrics.RecordDiscordRequests(MessageEdit, -1)
	metrics.RecordDiscordRequests(MessageCreateDelete, 2)
	metrics.RecordDiscordRequests(RateLimited, 1)

	want := map[string]float64{
		"message_edit":          9600,
		"message_create_delete": 3202,
		"rate_limited":          1,
	}
	got := operationCounts(t, registry)
	if len(got) != len(want) {
		t.Fatalf("counts = %v, want exactly three operation categories", got)
	}
	for name, count := range want {
		if got[name] != count {
			t.Errorf("%s = %v, want %v", name, got[name], count)
		}
	}
	other := operationCounts(t, otherRegistry)
	for name := range want {
		if count, ok := other[name]; !ok || count != 0 {
			t.Errorf("independent process counter %s = %v (present %v), want zero", name, count, ok)
		}
	}
}

func TestMetricsEndpointExposesProcessCounters(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	metrics.RecordDiscordRequests(MessageEdit, 7)

	// Constructing handlers must not register collectors or routes in the global registry/mux again.
	for i := 0; i < 2; i++ {
		handler := metricsHandler(registry)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("scrape status = %d: %s", response.Code, response.Body.String())
		}
		body := response.Body.String()
		if !strings.Contains(body, "# TYPE automuteus_discord_operations_total counter") ||
			!strings.Contains(body, `automuteus_discord_operations_total{type="message_edit"} 7`) {
			t.Fatalf("unexpected scrape output: %s", body)
		}
		if strings.Contains(body, "nodeID") || strings.Contains(body, "discord_requests_by_node_and_type") {
			t.Fatalf("scrape still exposes legacy node-attributed counters: %s", body)
		}
	}
}

func TestMetricsGameLifecycleAndCleanupLabelsAreFixed(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)

	metrics.RecordGameStarted()
	metrics.RecordGameStarted()
	metrics.SetActiveGames(2)
	metrics.RecordGameEnded(EndReasonManual)
	metrics.RecordGameEnded(EndReasonInactivity)
	metrics.RecordGameEnded(EndReason("ended by 12345")) // free text must not become a new series
	metrics.RecordGameEnded("")
	metrics.SetActiveGames(0)
	metrics.RecordCleanupFailure(CleanupUnmute)

	if got := testutil.ToFloat64(metrics.gamesStarted); got != 2 {
		t.Errorf("games started = %v, want 2", got)
	}
	if got := testutil.ToFloat64(metrics.activeGames); got != 0 {
		t.Errorf("active games = %v, want 0 after the last SetActiveGames", got)
	}
	ended := counterValues(t, registry, "automuteus_games_ended_total", "reason")
	wantEnded := map[string]float64{"manual": 1, "inactivity": 1, "other": 2, "replaced": 0, "capture_shutdown": 0, "critical_notice": 0}
	if len(ended) != len(wantEnded) {
		t.Fatalf("ended reasons = %v, want exactly the fixed set %v", ended, wantEnded)
	}
	for reason, want := range wantEnded {
		if ended[reason] != want {
			t.Errorf("games ended %s = %v, want %v", reason, ended[reason], want)
		}
	}
	cleanup := counterValues(t, registry, "automuteus_game_cleanup_failures_total", "step")
	if cleanup["unmute"] != 1 || cleanup["record_match"] != 0 || cleanup["notify"] != 0 || len(cleanup) != 3 {
		t.Errorf("cleanup failures = %v, want unmute=1 and the other two steps present at zero", cleanup)
	}
}

func TestMetricsVoiceOutcomesAndBatchDuration(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)

	metrics.RecordVoiceChange(VoiceRouteCapture, true)
	metrics.RecordVoiceChange(VoiceRoutePrimary, true)
	metrics.RecordVoiceChange(VoiceRoutePrimary, false)
	metrics.RecordWorkerFailure()
	metrics.RecordCaptureTask(CaptureTaskThrottled)
	metrics.RecordCaptureTask(CaptureTaskUnacked)
	metrics.ObserveMuteBatch(300 * time.Millisecond)
	metrics.ObserveMuteBatch(7 * time.Second)

	want := []struct {
		route   VoiceRoute
		outcome string
		count   float64
	}{
		{VoiceRouteCapture, "applied", 1},
		{VoiceRoutePrimary, "applied", 1},
		{VoiceRoutePrimary, "failed", 1},
		{VoiceRouteWorker, "applied", 0},
		{VoiceRouteWorker, "failed", 0},
		{VoiceRouteCapture, "failed", 0},
	}
	for _, w := range want {
		if got := testutil.ToFloat64(metrics.voiceChanges.WithLabelValues(string(w.route), w.outcome)); got != w.count {
			t.Errorf("voice changes {route=%s,outcome=%s} = %v, want %v", w.route, w.outcome, got, w.count)
		}
	}
	if got := testutil.ToFloat64(metrics.workerFailures); got != 1 {
		t.Errorf("worker failures = %v, want 1", got)
	}
	capture := counterValues(t, registry, "automuteus_capture_mute_tasks_total", "result")
	if capture["throttled"] != 1 || capture["unacked"] != 1 || capture["applied"] != 0 || capture["error"] != 0 || len(capture) != 4 {
		t.Errorf("capture tasks = %v", capture)
	}
	// Capture throttling is the bot's own decision and must not be reported as a Discord rate limit.
	if got := operationCounts(t, registry)["rate_limited"]; got != 0 {
		t.Errorf("rate_limited = %v, want 0 when only capture throttling happened", got)
	}

	handler := metricsHandler(registry)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	for _, line := range []string{
		"# TYPE automuteus_mute_batch_duration_seconds histogram",
		`automuteus_mute_batch_duration_seconds_bucket{le="0.5"} 1`,
		`automuteus_mute_batch_duration_seconds_bucket{le="10"} 2`,
		"automuteus_mute_batch_duration_seconds_count 2",
		"# TYPE automuteus_active_games gauge",
	} {
		if !strings.Contains(body, line) {
			t.Errorf("scrape output missing %q:\n%s", line, body)
		}
	}
}
