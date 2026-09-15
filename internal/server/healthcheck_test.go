package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func pass(context.Context) error { return nil }

func fail(msg string) Check {
	return func(context.Context) error { return errors.New(msg) }
}

func get(t *testing.T, h *Health, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code, rec.Body.String()
}

func TestReady_FailsWithStartingUntilStartupCompletes(t *testing.T) {
	h := NewHealth(0)
	h.AddCheck("redis", pass)

	code, body := get(t, h, "/ready")
	if code != http.StatusServiceUnavailable || !strings.Contains(body, "starting") {
		t.Fatalf("before start: %d %q", code, body)
	}

	h.SetStarted()
	code, body = get(t, h, "/ready")
	if code != http.StatusOK || body != "ready\nok   redis\n" {
		t.Fatalf("after start: %d %q", code, body)
	}
}

func TestReady_FailsWhileDraining(t *testing.T) {
	h := NewHealth(0)
	h.AddCheck("redis", pass)
	h.SetStarted()
	h.SetDraining()

	code, body := get(t, h, "/ready")
	if code != http.StatusServiceUnavailable || !strings.Contains(body, "draining") {
		t.Fatalf("draining: %d %q", code, body)
	}
}

func TestReady_ReportsEveryCheckAndNamesTheFailingOnes(t *testing.T) {
	h := NewHealth(0)
	h.AddCheck("shard-0", pass)
	h.AddCheck("shard-1", fail("gateway not connected"))
	h.AddCheck("redis", pass)
	h.AddCheck("postgres", fail("dial tcp: connection refused"))
	h.SetStarted()

	code, body := get(t, h, "/ready")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d", code)
	}
	want := "not ready\nok   shard-0\nFAIL shard-1: gateway not connected\nok   redis\nFAIL postgres: dial tcp: connection refused\n"
	if body != want {
		t.Fatalf("body = %q\nwant   %q", body, want)
	}
}

func TestReady_HungCheckIsReportedAsTimedOut(t *testing.T) {
	h := NewHealth(0)
	h.Timeout = 30 * time.Millisecond
	h.AddCheck("hung", func(ctx context.Context) error {
		<-ctx.Done() // a check that only returns when cancelled
		return ctx.Err()
	})
	h.AddCheck("ignores-ctx", func(context.Context) error {
		time.Sleep(500 * time.Millisecond)
		return nil
	})
	h.AddCheck("fast", pass)
	h.SetStarted()

	start := time.Now()
	report := h.Ready(context.Background())
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("Ready blocked for %v on a check that ignores its context", elapsed)
	}
	if report.Ready {
		t.Fatal("expected not ready")
	}
	for _, r := range report.Results {
		switch r.Name {
		case "fast":
			if r.Err != nil {
				t.Errorf("fast check failed: %v", r.Err)
			}
		default:
			if r.Err == nil {
				t.Errorf("%s should have failed", r.Name)
			}
		}
	}
	if r := report.Results[1]; r.Err == nil || !strings.Contains(r.Err.Error(), "timed out") {
		t.Fatalf("check ignoring ctx reported %v, want timed out", r.Err)
	}
}

func TestLive_AlwaysOKWithoutGrace(t *testing.T) {
	h := NewHealth(0)
	h.AddCheck("redis", fail("down"))
	h.SetStarted()
	_ = h.Ready(context.Background())
	h.now = func() time.Time { return time.Now().Add(24 * time.Hour) }

	if code, _ := get(t, h, "/live"); code != http.StatusOK {
		t.Fatalf("live = %d without a grace period", code)
	}
}

func TestLive_FailsOnceUnreadyForTheGracePeriod(t *testing.T) {
	h := NewHealth(time.Minute)
	clock := time.Unix(1_000_000, 0)
	h.now = func() time.Time { return clock }
	redisUp := true
	h.AddCheck("redis", func(context.Context) error {
		if redisUp {
			return nil
		}
		return errors.New("down")
	})

	// before startup, prolonged unreadiness is expected and never trips liveness
	_ = h.Ready(context.Background())
	clock = clock.Add(time.Hour)
	if err := h.Live(); err != nil {
		t.Fatalf("live failed before startup: %v", err)
	}

	h.SetStarted()
	redisUp = false
	_ = h.Ready(context.Background())
	clock = clock.Add(59 * time.Second)
	_ = h.Ready(context.Background())
	if err := h.Live(); err != nil {
		t.Fatalf("live failed inside the grace period: %v", err)
	}
	clock = clock.Add(time.Second)
	if code, body := get(t, h, "/live"); code != http.StatusServiceUnavailable || !strings.Contains(body, "1m0s") {
		t.Fatalf("live at the grace boundary: %d %q", code, body)
	}

	// recovery resets the clock
	redisUp = true
	_ = h.Ready(context.Background())
	if err := h.Live(); err != nil {
		t.Fatalf("live failed after recovery: %v", err)
	}
	redisUp = false
	_ = h.Ready(context.Background())
	clock = clock.Add(30 * time.Second)
	if err := h.Live(); err != nil {
		t.Fatalf("live failed 30s into a new outage: %v", err)
	}
}
