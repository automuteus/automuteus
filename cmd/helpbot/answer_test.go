package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"
)

// fakeAPI answers every request with body and records the last request it saw.
func fakeAPI(t *testing.T, body string) (*answerer, *map[string]any) {
	t.Helper()
	var last map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &last); err != nil {
			t.Errorf("request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return newAnswerer("claude-sonnet-5-5", "low", option.WithBaseURL(srv.URL), option.WithAPIKey("test"), option.WithMaxRetries(0)), &last
}

func apiResponse(stopReason, text string) string {
	content := []map[string]any{{"type": "thinking", "thinking": "", "signature": "sig"}}
	if text != "" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	b, _ := json.Marshal(map[string]any{
		"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-sonnet-5-5",
		"content": content, "stop_reason": stopReason,
		"usage": map[string]any{"input_tokens": 50, "output_tokens": 120, "cache_read_input_tokens": 7000, "cache_creation_input_tokens": 0},
	})
	return string(b)
}

var question = []turn{{text: "Title: help\n\nnobody gets muted"}}

func TestAskParsesStructuredAnswer(t *testing.T) {
	a, last := fakeAPI(t, apiResponse("end_turn", `{"answer":"  Check the voice channel permissions.  ","needs_human":false,"reason":"none"}`))
	res, err := a.ask(context.Background(), question)
	if err != nil {
		t.Fatal(err)
	}
	if res.Answer != "Check the voice channel permissions." || res.NeedsHuman || res.Model != "claude-sonnet-5-5" || res.Usage.CacheRead != 7000 {
		t.Errorf("result = %+v", res)
	}

	req := *last
	system := req["system"].([]any)[0].(map[string]any)
	if cc := system["cache_control"].(map[string]any); cc["ttl"] != "1h" {
		t.Errorf("system cache_control = %v, want a 1h breakpoint", cc)
	}
	if !strings.Contains(system["text"].(string), "galactus.automute.us") {
		t.Error("system prompt should contain the knowledge")
	}
	oc := req["output_config"].(map[string]any)
	if oc["effort"] != "low" || oc["format"].(map[string]any)["type"] != "json_schema" {
		t.Errorf("output_config = %v", oc)
	}
	if req["fallbacks"] != "default" {
		t.Errorf("fallbacks = %v, want default for Sonnet", req["fallbacks"])
	}
	if _, ok := req["thinking"]; ok {
		t.Error("thinking should be left to the model default")
	}
}

func TestAskNormalisesReason(t *testing.T) {
	a, _ := fakeAPI(t, apiResponse("end_turn", `{"answer":"Someone will look.","needs_human":true,"reason":"none"}`))
	res, err := a.ask(context.Background(), question)
	if err != nil {
		t.Fatal(err)
	}
	if !res.NeedsHuman || res.Reason != "other" {
		t.Errorf("a handoff without a reason should count as other: %+v", res.reply)
	}
}

func TestAskUnusableResponses(t *testing.T) {
	tests := map[string]string{
		"refusal":    apiResponse("refusal", ""),
		"max tokens": apiResponse("max_tokens", `{"answer":"cut o`),
		"bad json":   apiResponse("end_turn", `not json`),
		"empty":      apiResponse("end_turn", `{"answer":"  ","needs_human":false,"reason":"none"}`),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			a, _ := fakeAPI(t, body)
			res, err := a.ask(context.Background(), question)
			if !errors.Is(err, errNoAnswer) {
				t.Errorf("err = %v, want errNoAnswer", err)
			}
			if res.Model == "" {
				t.Error("usage should still be reported, so the spend is counted")
			}
		})
	}
}

func TestAskHaikuHasNoFallback(t *testing.T) {
	a, last := fakeAPI(t, apiResponse("end_turn", `{"answer":"ok","needs_human":false,"reason":"none"}`))
	a.model = "claude-haiku-5-5"
	if _, err := a.ask(context.Background(), question); err != nil {
		t.Fatal(err)
	}
	if _, ok := (*last)["fallbacks"]; ok {
		t.Error("Haiku has no server-side fallback")
	}
}

func TestAskRejectsBadConversations(t *testing.T) {
	a := newAnswerer("claude-sonnet-5-5", "low")
	for _, turns := range [][]turn{nil, {{assistant: true, text: "x"}}, {{text: "q"}, {assistant: true, text: "a"}}} {
		if _, err := a.ask(context.Background(), turns); err == nil {
			t.Errorf("ask(%+v) should fail", turns)
		}
	}
}

func TestAssistantTurnRoundTrips(t *testing.T) {
	if got := decodeAssistant(assistantTurn(`He said "hi"`)); got != `He said "hi"` {
		t.Errorf("got %q", got)
	}
}

func TestKnowledgeIsFactsOnly(t *testing.T) {
	if strings.TrimSpace(knowledge) == "" {
		t.Fatal("knowledge.md isn't embedded")
	}
	// Behavior rules live in prompt.go; the knowledge shouldn't tell the model to tag anything.
	if strings.Contains(strings.ToLower(knowledge), "needs human") {
		t.Error(`knowledge.md mentions "Needs human"; handoff rules belong in prompt.go`)
	}
}
