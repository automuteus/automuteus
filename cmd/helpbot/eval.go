package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// evalCase is one conversation from the questions file and what a good answer must do.
type evalCase struct {
	turns  []turn
	expect string
}

// parseQuestions reads blocks separated by "---" lines. Lines starting with "#" are comments, except
// "# expect:", which is kept for the reviewer. A block is one user message, unless its lines start
// with "USER:" or "BOT:", which make it a conversation.
func parseQuestions(r io.Reader) ([]evalCase, error) {
	var cases []evalCase
	var cur evalCase
	var lines []string
	flush := func() error {
		if len(lines) == 0 {
			cur = evalCase{}
			return nil
		}
		for _, l := range lines {
			switch {
			case strings.HasPrefix(l, "USER:"):
				cur.turns = append(cur.turns, turn{text: strings.TrimSpace(strings.TrimPrefix(l, "USER:"))})
			case strings.HasPrefix(l, "BOT:"):
				cur.turns = append(cur.turns, turn{assistant: true, text: assistantTurn(strings.TrimSpace(strings.TrimPrefix(l, "BOT:")))})
			case len(cur.turns) > 0:
				cur.turns[len(cur.turns)-1].text += "\n" + l
			default:
				cur.turns = append(cur.turns, turn{text: l})
			}
		}
		if cur.turns[0].assistant || cur.turns[len(cur.turns)-1].assistant {
			return fmt.Errorf("question %d must start and end with USER", len(cases)+1)
		}
		cases = append(cases, cur)
		cur, lines = evalCase{}, nil
		return nil
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), " \t")
		switch {
		case strings.TrimSpace(line) == "---":
			if err := flush(); err != nil {
				return nil, err
			}
		case strings.HasPrefix(line, "# expect:"):
			cur.expect = strings.TrimSpace(strings.TrimPrefix(line, "# expect:"))
		case strings.HasPrefix(line, "#"), strings.TrimSpace(line) == "" && len(lines) == 0:
		default:
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return cases, nil
}

type evalResult struct {
	res result
	err error
}

func runEval(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	questions := fs.String("questions", "cmd/helpbot/eval/questions.txt", "questions file")
	models := fs.String("models", "claude-sonnet-5-5,claude-haiku-5-5", "comma-separated models to compare")
	effort := fs.String("effort", "low", "effort level")
	out := fs.String("out", "helpbot-eval.md", "markdown report to write")
	parallel := fs.Int("parallel", 4, "requests in flight per model")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return errors.New("ANTHROPIC_API_KEY is required")
	}
	f, err := os.Open(*questions)
	if err != nil {
		return err
	}
	cases, err := parseQuestions(f)
	f.Close()
	if err != nil {
		return err
	}
	names := strings.Split(*models, ",")
	results := make([][]evalResult, len(names))
	for mi, model := range names {
		model = strings.TrimSpace(model)
		names[mi] = model
		a := newAnswerer(model, *effort)
		results[mi] = make([]evalResult, len(cases))
		run := func(i int) {
			res, err := a.ask(ctx, cases[i].turns)
			results[mi][i] = evalResult{res, err}
			log.Printf("eval: %s %d/%d (cache_read=%d)", model, i+1, len(cases), res.Usage.CacheRead)
		}
		// The first request writes the cache; the rest should read it.
		run(0)
		sem := make(chan struct{}, max(1, *parallel))
		var wg sync.WaitGroup
		for i := 1; i < len(cases); i++ {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				run(i)
			}()
		}
		wg.Wait()
	}
	report := evalReport(cases, names, results)
	if err := os.WriteFile(*out, []byte(report), 0o644); err != nil {
		return err
	}
	fmt.Print(evalSummary(names, results))
	fmt.Printf("Report written to %s\n", *out)
	return nil
}

func evalSummary(names []string, results [][]evalResult) string {
	var b strings.Builder
	b.WriteString("| Model | Cost | Per answer | Handoffs | Errors | Cache hits |\n|---|---|---|---|---|---|\n")
	for mi, model := range names {
		var total float64
		var handoffs, errs, hits int
		for _, r := range results[mi] {
			if r.res.Model != "" {
				total += cost(r.res.Model, r.res.Usage)
			}
			if r.err != nil {
				errs++
			} else if r.res.NeedsHuman {
				handoffs++
			}
			if r.res.Usage.CacheRead > 0 {
				hits++
			}
		}
		n := len(results[mi])
		fmt.Fprintf(&b, "| %s | $%.4f | $%.4f | %d | %d | %d/%d |\n", model, total, total/float64(max(n, 1)), handoffs, errs, hits, n)
	}
	return b.String()
}

func evalReport(cases []evalCase, names []string, results [][]evalResult) string {
	var b strings.Builder
	b.WriteString("# Help bot eval\n\n")
	b.WriteString(evalSummary(names, results))
	b.WriteString("\nCache hits should be every request but the first per model; if they're zero, something in the system prompt changes between requests.\n")
	for i, c := range cases {
		fmt.Fprintf(&b, "\n---\n\n## %d. %s\n\n", i+1, oneLine(c.turns[len(c.turns)-1].text))
		if len(c.turns) > 1 {
			b.WriteString("Conversation so far:\n\n")
			for _, t := range c.turns[:len(c.turns)-1] {
				who, text := "User", t.text
				if t.assistant {
					who, text = "Bot", decodeAssistant(t.text)
				}
				fmt.Fprintf(&b, "- **%s:** %s\n", who, oneLine(text))
			}
			b.WriteString("\n")
		}
		if c.expect != "" {
			fmt.Fprintf(&b, "**Expect:** %s\n\n", c.expect)
		}
		for mi, model := range names {
			r := results[mi][i]
			fmt.Fprintf(&b, "### %s\n\n", model)
			if r.err != nil {
				fmt.Fprintf(&b, "**Error:** %v\n\n", r.err)
				continue
			}
			handoff := "no"
			if r.res.NeedsHuman {
				handoff = "**yes** (" + r.res.Reason + ")"
			}
			fmt.Fprintf(&b, "Handoff: %s · %d chars · $%.4f · served by %s\n\n", handoff, len([]rune(r.res.Answer)), cost(r.res.Model, r.res.Usage), r.res.Model)
			for _, line := range strings.Split(r.res.Answer, "\n") {
				b.WriteString("> " + line + "\n")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
