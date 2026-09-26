package main

import (
	"testing"
	"time"
)

func TestConfigNeedsDatabasesButNoDiscordToken(t *testing.T) {
	env := map[string]string{"REDIS_ADDR": "redis:6379", "POSTGRES_ADDR": "postgres:5432", "POSTGRES_USER": "postgres", "POSTGRES_PASS": "test"}
	get := func(k string) string { return env[k] }
	c, err := configFromEnv(get)
	if err != nil {
		t.Fatal(err)
	}
	if c.port != "5000" || c.api.CaptureHost != "http://localhost:8123" {
		t.Fatalf("unexpected defaults: %#v", c.api)
	}
	for _, key := range []string{"REDIS_ADDR", "POSTGRES_ADDR", "POSTGRES_USER", "POSTGRES_PASS"} {
		value := env[key]
		delete(env, key)
		if _, err := configFromEnv(get); err == nil {
			t.Errorf("accepted missing %s", key)
		}
		env[key] = value
	}
	for _, tc := range []struct{ key, value string }{{"API_PORT", "bad"}, {"API_PORT", "0"}, {"API_PORT", "65536"},
		{"API_STATS_BUILD_TIMEOUT", "soon"}, {"API_STATS_BUILD_TIMEOUT", "0"}, {"API_STATS_BUILD_TIMEOUT", "-1m"}, {"API_STATS_BUILD_TIMEOUT", "300"}} {
		env[tc.key] = tc.value
		if _, err := configFromEnv(get); err == nil {
			t.Errorf("accepted %s=%s", tc.key, tc.value)
		}
		delete(env, tc.key)
	}
}

// Unset leaves the build budget to the API's default; a duration sets it.
func TestConfigStatsBuildTimeout(t *testing.T) {
	env := map[string]string{"REDIS_ADDR": "redis:6379", "POSTGRES_ADDR": "postgres:5432", "POSTGRES_USER": "postgres", "POSTGRES_PASS": "test"}
	get := func(k string) string { return env[k] }
	c, err := configFromEnv(get)
	if err != nil || c.api.StatsBuildTimeout != 0 {
		t.Fatalf("unset: %v, timeout %v; want zero for the default", err, c.api.StatsBuildTimeout)
	}
	env["API_STATS_BUILD_TIMEOUT"] = "5m"
	c, err = configFromEnv(get)
	if err != nil || c.api.StatsBuildTimeout != 5*time.Minute {
		t.Fatalf("5m: %v, timeout %v", err, c.api.StatsBuildTimeout)
	}
}
