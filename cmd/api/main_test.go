package main

import "testing"

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
	for _, tc := range []struct{ key, value string }{{"API_PORT", "bad"}, {"API_PORT", "0"}, {"API_PORT", "65536"}} {
		env[tc.key] = tc.value
		if _, err := configFromEnv(get); err == nil {
			t.Errorf("accepted %s=%s", tc.key, tc.value)
		}
		delete(env, tc.key)
	}
}
