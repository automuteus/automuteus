package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// DefaultCheckTimeout bounds one readiness evaluation. Every check shares it; a check that has not answered by then
// is reported as timed out and readiness fails.
const DefaultCheckTimeout = 3 * time.Second

// Check reports whether one dependency the process needs is usable right now. It must honor ctx.
type Check func(ctx context.Context) error

type namedCheck struct {
	name  string
	check Check
}

// Health answers the orchestrator's liveness and readiness probes from the process's own view of its dependencies:
// each shard's gateway session, Redis, Postgres. Readiness fails until startup completes, while any check fails, and
// from the moment shutdown begins. Liveness is unconditional unless a grace period is set, in which case it fails
// once readiness has been failing continuously for that long, so a wedged process gets restarted.
type Health struct {
	mu       sync.RWMutex
	checks   []namedCheck
	started  bool
	draining bool
	// unreadySince is when readiness last began failing after startup; zero while ready or before startup
	unreadySince time.Time

	// Timeout bounds one readiness evaluation.
	Timeout time.Duration
	// LivenessGrace is how long readiness may fail continuously before liveness also fails; zero disables that.
	LivenessGrace time.Duration

	now func() time.Time
}

func NewHealth(livenessGrace time.Duration) *Health {
	return &Health{Timeout: DefaultCheckTimeout, LivenessGrace: livenessGrace, now: time.Now}
}

// AddCheck registers a named readiness check. Checks run concurrently and are reported in registration order.
func (h *Health) AddCheck(name string, check Check) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks = append(h.checks, namedCheck{name: name, check: check})
}

// SetStarted marks startup complete; readiness is evaluated from the checks from now on.
func (h *Health) SetStarted() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.started = true
}

// SetDraining marks shutdown begun; readiness fails from now on so the orchestrator stops counting on this process.
func (h *Health) SetDraining() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.draining = true
}

// Result is one check's outcome.
type Result struct {
	Name string
	Err  error
}

// Report is the outcome of one readiness evaluation.
type Report struct {
	Ready bool
	// Reason is set when readiness was decided without running the checks (starting, draining).
	Reason  string
	Results []Result
}

func (r Report) String() string {
	var b strings.Builder
	switch {
	case r.Ready:
		b.WriteString("ready\n")
	case r.Reason != "":
		fmt.Fprintf(&b, "not ready: %s\n", r.Reason)
	default:
		b.WriteString("not ready\n")
	}
	for _, res := range r.Results {
		if res.Err != nil {
			fmt.Fprintf(&b, "FAIL %s: %v\n", res.Name, res.Err)
		} else {
			fmt.Fprintf(&b, "ok   %s\n", res.Name)
		}
	}
	return b.String()
}

// Ready evaluates readiness: every registered check must pass within the timeout.
func (h *Health) Ready(ctx context.Context) Report {
	h.mu.RLock()
	started, draining := h.started, h.draining
	checks := append([]namedCheck(nil), h.checks...)
	h.mu.RUnlock()

	if !started {
		return Report{Reason: "starting"}
	}
	if draining {
		return Report{Reason: "draining"}
	}

	ctx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()

	outcomes := make([]chan error, len(checks))
	for i, c := range checks {
		outcomes[i] = make(chan error, 1)
		go func(c namedCheck, out chan<- error) { out <- c.check(ctx) }(c, outcomes[i])
	}

	report := Report{Ready: true, Results: make([]Result, len(checks))}
	for i, c := range checks {
		var err error
		select {
		case err = <-outcomes[i]:
		default:
			// not finished yet: wait for it or the deadline. A check that finished before the deadline, while the
			// collector was waiting on an earlier one, is picked up by the non-blocking receive above.
			select {
			case err = <-outcomes[i]:
			case <-ctx.Done():
				err = fmt.Errorf("timed out after %s", h.Timeout)
			}
		}
		report.Results[i] = Result{Name: c.name, Err: err}
		if err != nil {
			report.Ready = false
		}
	}

	h.mu.Lock()
	switch {
	case report.Ready:
		h.unreadySince = time.Time{}
	case h.unreadySince.IsZero():
		h.unreadySince = h.now()
	}
	h.mu.Unlock()
	return report
}

// Live reports whether the process should keep running. It only fails when a liveness grace is configured and
// readiness (as last evaluated) has been failing for at least that long.
func (h *Health) Live() error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.LivenessGrace <= 0 || h.unreadySince.IsZero() {
		return nil
	}
	if unready := h.now().Sub(h.unreadySince); unready >= h.LivenessGrace {
		return fmt.Errorf("not ready for %s, exceeding liveness grace of %s", unready.Truncate(time.Second), h.LivenessGrace)
	}
	return nil
}

// Handler serves /live and /ready. Failures answer 503 with a plain-text report naming each check, which the
// orchestrator records on the pod's events.
func (h *Health) Handler() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/live", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if err := h.Live(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, err)
			return
		}
		fmt.Fprintln(w, "alive")
	})
	r.HandleFunc("/ready", func(w http.ResponseWriter, req *http.Request) {
		report := h.Ready(req.Context())
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if !report.Ready {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		fmt.Fprint(w, report.String())
	})
	return r
}

func StartHealthCheckServer(port string, h *Health) {
	if err := http.ListenAndServe(":"+port, h.Handler()); err != nil {
		log.Printf("Health check server stopped: %v", err)
	}
}
