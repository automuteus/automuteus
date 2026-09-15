package main

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// prompts keeps the first read error so an incomplete form is never sent.
// Each validation loop stops at EOF, including when stdin is redirected.
type prompts struct {
	scanner *bufio.Scanner
	out     io.Writer
	err     error
}

func newPrompts(in io.Reader, out io.Writer) *prompts {
	return &prompts{scanner: bufio.NewScanner(in), out: out}
}

func (p *prompts) text(label, def string) string {
	if p.err != nil {
		return ""
	}
	fmt.Fprintf(p.out, "%s [%s]: ", label, def)
	if !p.scanner.Scan() {
		p.err = p.scanner.Err()
		if p.err == nil {
			p.err = io.EOF
		}
		return ""
	}
	value := strings.TrimSpace(p.scanner.Text())
	if value == "" {
		return def
	}
	return value
}

func (p *prompts) choice(label string, def int, choices map[int]string) int {
	if p.err != nil {
		return def
	}
	keys := make([]int, 0, len(choices))
	for key := range choices {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	for _, key := range keys {
		fmt.Fprintf(p.out, "  %d %s\n", key, choices[key])
	}
	for p.err == nil {
		value := p.text(label, strconv.Itoa(def))
		if p.err != nil {
			break
		}
		n, err := strconv.Atoi(value)
		if _, ok := choices[n]; err == nil && ok {
			return n
		}
		fmt.Fprintln(p.out, "Choose one of the listed numbers.")
	}
	return def
}

func (p *prompts) boolean(label string, def bool) bool {
	for p.err == nil {
		switch strings.ToLower(p.text(label, strconv.FormatBool(def))) {
		case "true", "t", "yes", "y", "1":
			return true
		case "false", "f", "no", "n", "0":
			return false
		}
		if p.err == nil {
			fmt.Fprintln(p.out, "Enter true/false or yes/no.")
		}
	}
	return def
}

// observe asks whether a checkpoint was seen in Discord. quit reports that the
// operator wants to stop the scenario rather than answer.
func (p *prompts) observe(label string) (observed, quit bool) {
	fmt.Fprintf(p.out, "?? %s\n", label)
	for p.err == nil {
		switch strings.ToLower(p.text("Observed? y/n, or q to stop the scenario", "")) {
		case "y", "yes":
			return true, false
		case "n", "no":
			return false, false
		case "q", "quit":
			return false, true
		}
		if p.err == nil {
			fmt.Fprintln(p.out, "Enter y, n, or q.")
		}
	}
	return false, false
}
