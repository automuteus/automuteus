package main

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)
	l := newLimiter(2, 1.0, func() time.Time { return now })

	for range 2 {
		if r := l.check("u", true); r != "" {
			t.Fatalf("check = %q, want allowed", r)
		}
		l.record("u", true, 0.01)
	}
	if r := l.check("u", true); r != "user_limit" {
		t.Errorf("third new thread: %q, want user_limit", r)
	}
	if r := l.check("u", false); r != "" {
		t.Errorf("follow-ups aren't limited per user: %q", r)
	}
	if r := l.check("other", true); r != "" {
		t.Errorf("another user: %q", r)
	}

	l.record("other", false, 1.0)
	if r := l.check("other", true); r != "budget" {
		t.Errorf("over budget: %q, want budget", r)
	}

	now = now.Add(2 * time.Hour) // the next UTC day
	if r := l.check("u", true); r != "" {
		t.Errorf("after midnight UTC: %q, want everything reset", r)
	}
}

func TestCost(t *testing.T) {
	u := usage{Input: 1_000_000, Output: 1_000_000, CacheRead: 1_000_000, CacheWrite: 1_000_000}
	if got := cost("claude-sonnet-5-5", u); got != 2+10+0.20+4 {
		t.Errorf("sonnet = %v", got)
	}
	if got, want := cost("claude-something-new", u), cost("claude-opus-5-5", u); got != want {
		t.Errorf("unknown models are priced as Opus: %v, want %v", got, want)
	}
}
