package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitMessageShort(t *testing.T) {
	if got := splitMessage("hello", 2000); len(got) != 1 || got[0] != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestSplitMessagePrefersLineBreaks(t *testing.T) {
	line := strings.Repeat("あ", 90) // multi-byte, so a byte-based split would be wrong
	text := strings.TrimSpace(strings.Repeat(line+"\n", 50))
	chunks := splitMessage(text, 2000)
	if len(chunks) < 2 {
		t.Fatalf("got %d chunks, want a split", len(chunks))
	}
	for i, c := range chunks {
		if n := utf8.RuneCountInString(c); n > 2000 {
			t.Errorf("chunk %d has %d characters", i, n)
		}
		for _, l := range strings.Split(c, "\n") {
			if l != line {
				t.Errorf("chunk %d split mid-line: %q", i, l)
			}
		}
	}
	if strings.Join(chunks, "\n") != text {
		t.Error("chunks don't add back up to the text")
	}
}

func TestSplitMessageWithoutBreaks(t *testing.T) {
	chunks := splitMessage(strings.Repeat("x", 4500), 2000)
	for i, c := range chunks {
		if n := utf8.RuneCountInString(c); n > 2000 {
			t.Errorf("chunk %d has %d characters", i, n)
		}
	}
	if strings.Join(chunks, "") != strings.Repeat("x", 4500) {
		t.Error("lost characters")
	}
}

func TestSplitMessageKeepsCodeBlocksClosed(t *testing.T) {
	text := "Run this:\n```\n" + strings.Repeat("line of output\n", 200) + "```\nDone."
	for i, c := range splitMessage(text, 2000) {
		if strings.Count(c, "```")%2 != 0 {
			t.Errorf("chunk %d leaves a code block open", i)
		}
		if n := utf8.RuneCountInString(c); n > 2000 {
			t.Errorf("chunk %d has %d characters", i, n)
		}
	}
}

func TestAnswerMessages(t *testing.T) {
	sends := answerMessages(strings.Repeat("word ", 600), false, "t", "q")
	if len(sends) < 2 {
		t.Fatalf("got %d messages, want a split", len(sends))
	}
	last := sends[len(sends)-1]
	if !strings.HasSuffix(last.Content, footerAnswer) || len(last.Components) != 1 {
		t.Errorf("last message should carry the footer and buttons: %+v", last)
	}
	if r := sends[0].Reference; r == nil || r.MessageID != "q" || r.ChannelID != "t" {
		t.Errorf("first message should reply to the question: %+v", r)
	}
	for _, s := range sends[1:] {
		if s.Reference != nil {
			t.Error("only the first message is a reply")
		}
	}
	for _, s := range sends[:len(sends)-1] {
		if len(s.Components) != 0 {
			t.Error("only the last message gets buttons")
		}
	}
	for _, s := range sends {
		if s.AllowedMentions == nil || s.AllowedMentions.Parse == nil || len(s.AllowedMentions.Parse) != 0 {
			t.Error("every message must disable mentions")
		}
	}

	handoff := answerMessages("Someone will take a look.", true, "t", "")
	if len(handoff) != 1 || len(handoff[0].Components) != 0 || !strings.HasSuffix(handoff[0].Content, footerHandoff) {
		t.Errorf("handoff answer = %+v, want the handoff footer and no buttons", handoff[0])
	}
}

func TestStripFooter(t *testing.T) {
	for _, f := range []string{footerAnswer, footerHandoff} {
		if got := stripFooter("The answer.\n\n" + f); got != "The answer." {
			t.Errorf("got %q", got)
		}
	}
}
