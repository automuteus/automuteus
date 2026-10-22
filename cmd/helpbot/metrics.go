package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// metrics are named automuteus_helpbot_* so the Alloy filter that keeps automuteus_* picks them up.
type metrics struct {
	registry *prometheus.Registry
	answers  *prometheus.CounterVec // model, handoff
	skips    *prometheus.CounterVec // reason
	handoffs *prometheus.CounterVec // reason
	errors   *prometheus.CounterVec // stage
	buttons  *prometheus.CounterVec // button
	tokens   *prometheus.CounterVec // model, kind
	dollars  *prometheus.CounterVec // model
}

func newMetrics() *metrics {
	counter := func(name, help string, labels ...string) *prometheus.CounterVec {
		return prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: "automuteus", Subsystem: "helpbot", Name: name, Help: help}, labels)
	}
	m := &metrics{
		registry: prometheus.NewRegistry(),
		answers:  counter("answers_total", "Answers posted.", "model", "handoff"),
		skips:    counter("skips_total", "Poster messages the bot didn't answer, by reason.", "reason"),
		handoffs: counter("handoffs_total", "Threads tagged for a human, by reason.", "reason"),
		errors:   counter("errors_total", "Failures, by stage.", "stage"),
		buttons:  counter("buttons_total", "Button presses.", "button"),
		tokens:   counter("tokens_total", "Tokens used, by kind.", "model", "kind"),
		dollars:  counter("dollars_total", "Estimated spend in US dollars.", "model"),
	}
	m.registry.MustRegister(m.answers, m.skips, m.handoffs, m.errors, m.buttons, m.tokens, m.dollars,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

func (m *metrics) recordUsage(model string, u usage) {
	m.tokens.WithLabelValues(model, "input").Add(float64(u.Input))
	m.tokens.WithLabelValues(model, "output").Add(float64(u.Output))
	m.tokens.WithLabelValues(model, "cache_read").Add(float64(u.CacheRead))
	m.tokens.WithLabelValues(model, "cache_write").Add(float64(u.CacheWrite))
	m.dollars.WithLabelValues(model).Add(cost(model, u))
}
