package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

// turn is one side of the conversation sent to the model.
type turn struct {
	assistant bool
	text      string
}

// reply is the model's structured answer.
type reply struct {
	Answer     string `json:"answer"`
	NeedsHuman bool   `json:"needs_human"`
	Reason     string `json:"reason"`
}

// result is a reply plus what it cost and which model produced it.
type result struct {
	reply
	Model string
	Usage usage
}

// errNoAnswer means the request succeeded but produced nothing postable: a refusal, a truncated
// answer, or JSON that doesn't match the schema. The thread should go to a human.
var errNoAnswer = errors.New("no usable answer")

type answerer struct {
	client anthropic.Client
	model  string
	effort string
	system string
}

func newAnswerer(model, effort string, opts ...option.RequestOption) *answerer {
	return &answerer{client: anthropic.NewClient(opts...), model: model, effort: effort, system: systemPrompt()}
}

// ask sends the conversation, which must start and end with a user turn.
func (a *answerer) ask(ctx context.Context, turns []turn) (result, error) {
	if len(turns) == 0 || turns[0].assistant || turns[len(turns)-1].assistant {
		return result{}, errors.New("conversation must start and end with the user")
	}
	messages := make([]anthropic.BetaMessageParam, 0, len(turns))
	for _, t := range turns {
		block := anthropic.NewBetaTextBlock(t.text)
		if t.assistant {
			messages = append(messages, anthropic.BetaMessageParam{Role: anthropic.BetaMessageParamRoleAssistant, Content: []anthropic.BetaContentBlockParamUnion{block}})
		} else {
			messages = append(messages, anthropic.NewBetaUserMessage(block))
		}
	}
	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: 8000,
		System: []anthropic.BetaTextBlockParam{{
			Text:         a.system,
			CacheControl: anthropic.BetaCacheControlEphemeralParam{TTL: anthropic.BetaCacheControlEphemeralTTLTTL1h},
		}},
		Messages: messages,
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: anthropic.BetaOutputConfigEffort(a.effort),
			Format: anthropic.BetaJSONOutputFormatParam{Schema: answerSchema()},
		},
	}
	// A refusal is re-served by a fallback model inside the same call. Haiku has no server-side fallback.
	if !strings.HasPrefix(a.model, "claude-haiku") {
		params.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
		params.Fallbacks = anthropic.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()}
	}
	msg, err := a.client.Beta.Messages.New(ctx, params)
	if err != nil {
		return result{}, err
	}
	res := result{Model: string(msg.Model), Usage: usage{
		Input:      msg.Usage.InputTokens,
		Output:     msg.Usage.OutputTokens,
		CacheRead:  msg.Usage.CacheReadInputTokens,
		CacheWrite: msg.Usage.CacheCreationInputTokens,
	}}
	if msg.StopReason != anthropic.BetaStopReasonEndTurn {
		return res, fmt.Errorf("%w: stop reason %s", errNoAnswer, msg.StopReason)
	}
	var text strings.Builder
	for _, block := range msg.Content {
		if b, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			text.WriteString(b.Text)
		}
	}
	if err := json.Unmarshal([]byte(text.String()), &res.reply); err != nil {
		return res, fmt.Errorf("%w: %v", errNoAnswer, err)
	}
	res.Answer = strings.TrimSpace(res.Answer)
	if res.Answer == "" {
		return res, fmt.Errorf("%w: empty answer", errNoAnswer)
	}
	if !slices.Contains(handoffReasons, res.Reason) {
		res.Reason = "other"
	}
	if !res.NeedsHuman {
		res.Reason = "none"
	} else if res.Reason == "none" {
		res.Reason = "other"
	}
	return res, nil
}

// assistantTurn renders an earlier bot answer the way the model produced it, so history matches the
// output format.
func assistantTurn(answer string) string {
	b, _ := json.Marshal(reply{Answer: answer, Reason: "none"})
	return string(b)
}

// decodeAssistant is assistantTurn's inverse, for display.
func decodeAssistant(text string) string {
	var r reply
	if json.Unmarshal([]byte(text), &r) != nil {
		return text
	}
	return r.Answer
}
