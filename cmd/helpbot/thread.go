package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

// threadView is everything the bot knows about a forum thread, rebuilt from Discord on every event so
// that nothing has to be stored between them.
type threadView struct {
	id, ownerID, title string
	tags               []string
	// messages are oldest first.
	messages    []*discordgo.Message
	botID       string
	staffPosted bool
}

type limits struct {
	minChars   int
	maxReplies int
}

// decision is whether to answer a trigger message, and the conversation to send if so.
type decision struct {
	skip  string // a metric label when not answering
	turns []turn
	// replyTo is the newest poster message the answer covers. The answer is posted as a Discord reply
	// to it, which is how later events tell what has been answered without storing anything.
	replyTo string
	// first is true when the bot hasn't answered this thread yet; only first answers count toward a
	// user's daily thread limit.
	first bool
}

// decide applies the per-thread rules from HELPBOT_PLAN.md. blockedTags are the "Solved" and
// "Needs human" tag IDs.
func decide(v threadView, trigger string, blockedTags []string, l limits) decision {
	for _, t := range v.tags {
		if slices.Contains(blockedTags, t) {
			return decision{skip: "tagged"}
		}
	}
	if v.staffPosted {
		return decision{skip: "staff"}
	}
	triggerID := snowflake(trigger)
	covered := coveredUpTo(v)
	var replyTo string
	opChars := utf8.RuneCountInString(strings.TrimSpace(v.title))
	for _, m := range v.messages {
		if m.Author == nil {
			continue
		}
		id := snowflake(m.ID)
		if m.Author.ID == v.ownerID {
			if id > triggerID {
				// A later message from the poster has its own handler, which answers both.
				return decision{skip: "superseded"}
			}
			replyTo = m.ID
			opChars += utf8.RuneCountInString(strings.TrimSpace(m.Content))
		}
	}
	if triggerID <= covered {
		return decision{skip: "answered"}
	}
	if opChars < l.minChars {
		return decision{skip: "too_short"}
	}
	turns := buildTurns(v)
	if len(turns) == 0 {
		return decision{skip: "too_short"}
	}
	if turns[len(turns)-1].assistant {
		// Nothing from the poster after the last answer, e.g. the trigger was deleted.
		return decision{skip: "answered"}
	}
	replies := 0
	for _, t := range turns {
		if t.assistant {
			replies++
		}
	}
	if replies >= l.maxReplies {
		return decision{skip: "max_replies"}
	}
	return decision{turns: turns, replyTo: replyTo, first: replies == 0}
}

// buildTurns turns the thread into a conversation: the poster's messages are user turns, the bot's
// answers are assistant turns, and everyone else is left out. Consecutive messages from one side merge.
// Poster messages no answer has replied to yet go last, even when an answer was posted after them: that
// answer was written before they arrived.
func buildTurns(v threadView) []turn {
	covered := coveredUpTo(v)
	var unanswered []*discordgo.Message
	var turns []turn
	push := func(assistant bool, text string) {
		if text == "" {
			return
		}
		if n := len(turns); n > 0 && turns[n-1].assistant == assistant {
			turns[n-1].text += "\n\n" + text
			return
		}
		turns = append(turns, turn{assistant: assistant, text: text})
	}
	// Assistant turns are collected raw, then re-encoded in the model's output format once complete,
	// because one answer can span several Discord messages.
	var pendingBot []string
	flushBot := func() {
		if len(pendingBot) == 0 {
			return
		}
		answer := stripFooter(strings.Join(pendingBot, "\n"))
		pendingBot = nil
		if answer != "" {
			push(true, assistantTurn(answer))
		}
	}
	for _, m := range v.messages {
		if m.Author == nil {
			continue
		}
		switch m.Author.ID {
		case v.botID:
			// Button acknowledgements and other interaction responses aren't answers.
			if m.Interaction != nil {
				continue
			}
			pendingBot = append(pendingBot, m.Content)
		case v.ownerID:
			if snowflake(m.ID) > covered {
				unanswered = append(unanswered, m)
				continue
			}
			flushBot()
			push(false, userText(v, m))
		}
	}
	flushBot()
	for _, m := range unanswered {
		push(false, userText(v, m))
	}
	// The conversation must start with the user. A thread whose starter message is gone (more than 100
	// messages, or deleted) can begin with a bot answer, which is dropped.
	for len(turns) > 0 && turns[0].assistant {
		turns = turns[1:]
	}
	return turns
}

// coveredUpTo is the newest poster message a bot answer has replied to. An answer posted after a
// message doesn't cover it unless it replies to it (or to something newer): the poster may have
// written while the answer was being generated. An answer that isn't a reply (the message it was
// for got deleted) covers everything before it; the later parts of a split answer cover nothing more
// than its first part.
func coveredUpTo(v threadView) uint64 {
	var covered, lastPoster uint64
	prevBot := false
	for _, m := range v.messages {
		if m.Author == nil {
			continue
		}
		switch {
		case m.Author.ID == v.ownerID:
			lastPoster, prevBot = snowflake(m.ID), false
		case m.Author.ID == v.botID && m.Interaction == nil:
			if m.MessageReference != nil {
				covered = max(covered, snowflake(m.MessageReference.MessageID))
			} else if !prevBot {
				covered = max(covered, lastPoster)
			}
			prevBot = true
		}
	}
	return covered
}

func userText(v threadView, m *discordgo.Message) string {
	text := strings.TrimSpace(m.Content)
	if m.ID == v.id {
		// In a forum, the starter message shares the thread's ID and the title carries half the question.
		text = strings.TrimSpace("Title: " + strings.TrimSpace(v.title) + "\n\n" + text)
	}
	if n := len(m.Attachments); n > 0 {
		text = strings.TrimSpace(text + fmt.Sprintf("\n\n[The user attached %d file(s), which you can't see.]", n))
	}
	return text
}

// snowflake parses a Discord ID for ordering; IDs of different lengths don't sort as strings.
func snowflake(id string) uint64 {
	n, _ := strconv.ParseUint(id, 10, 64)
	return n
}
