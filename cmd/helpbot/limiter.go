package main

import (
	"sync"
	"time"
)

// limiter holds the bot's only in-memory state: per-user thread counts and the dollars spent today
// (UTC). Both reset on restart, which is acceptable; the Anthropic workspace spend limit is the hard
// backstop.
type limiter struct {
	mu          sync.Mutex
	now         func() time.Time
	day         string
	threads     map[string]int
	spent       float64
	maxThreads  int
	dailyBudget float64
}

func newLimiter(maxThreads int, dailyBudget float64, now func() time.Time) *limiter {
	return &limiter{now: now, threads: map[string]int{}, maxThreads: maxThreads, dailyBudget: dailyBudget}
}

func (l *limiter) roll() {
	if day := l.now().UTC().Format(time.DateOnly); day != l.day {
		l.day, l.threads, l.spent = day, map[string]int{}, 0
	}
}

// check reports why a request can't be made, or "" if it can. first is whether it would start a new
// thread for the user.
func (l *limiter) check(userID string, first bool) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll()
	if l.spent >= l.dailyBudget {
		return "budget"
	}
	if first && l.threads[userID] >= l.maxThreads {
		return "user_limit"
	}
	return ""
}

// record counts a finished request. Concurrent requests can each pass check and together overshoot by
// a request or two, which is fine at this volume.
func (l *limiter) record(userID string, first bool, dollars float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll()
	l.spent += dollars
	if first {
		l.threads[userID]++
	}
}
