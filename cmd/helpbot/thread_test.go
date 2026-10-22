package main

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

const (
	threadID = "1300000000000000000"
	opID     = "111"
	botID    = "222"
	otherID  = "333"
)

func msg(id, author, content string) *discordgo.Message {
	return &discordgo.Message{ID: id, Content: content, Author: &discordgo.User{ID: author}}
}

func botReply(id, author, content, to string) *discordgo.Message {
	m := msg(id, author, content)
	m.MessageReference = &discordgo.MessageReference{MessageID: to, ChannelID: threadID}
	return m
}

func view(msgs ...*discordgo.Message) threadView {
	return threadView{id: threadID, ownerID: opID, title: "Capture won't connect", botID: botID, messages: msgs}
}

var testLimits = limits{minChars: 20, maxReplies: 5}

func TestDecideFirstPost(t *testing.T) {
	d := decide(view(msg(threadID, opID, "it says no capture linked")), threadID, nil, testLimits)
	if d.skip != "" {
		t.Fatalf("skip = %q, want an answer", d.skip)
	}
	if d.replyTo != threadID {
		t.Errorf("replyTo = %q, want the starter message", d.replyTo)
	}
	if !d.first || len(d.turns) != 1 || d.turns[0].assistant {
		t.Fatalf("decision = %+v, want one first user turn", d)
	}
	if want := "Title: Capture won't connect\n\nit says no capture linked"; d.turns[0].text != want {
		t.Errorf("text = %q, want %q", d.turns[0].text, want)
	}
}

func TestDecideSkips(t *testing.T) {
	blocked := []string{"solved", "human"}
	tests := []struct {
		name    string
		v       threadView
		trigger string
		want    string
	}{
		{"tagged", func() threadView {
			v := view(msg(threadID, opID, "a long enough question here"))
			v.tags = []string{"human"}
			return v
		}(), threadID, "tagged"},
		{"staff", func() threadView {
			v := view(msg(threadID, opID, "a long enough question here"))
			v.staffPosted = true
			return v
		}(), threadID, "staff"},
		{"already answered", view(msg(threadID, opID, "a long enough question here"), botReply("1300000000000000001", botID, "answer", threadID)), threadID, "answered"},
		{"answer covers a newer message", view(msg(threadID, opID, "a long enough question here"), msg("1300000000000000001", opID, "more detail"), botReply("1300000000000000002", botID, "answer", "1300000000000000001")), "1300000000000000001", "answered"},
		// Trigger deleted: the newest poster message left is already answered.
		{"nothing new since the answer", view(msg(threadID, opID, "a long enough question here"), botReply("1300000000000000002", botID, "answer", threadID)), "1300000000000000001", "answered"},
		{"later poster message", view(msg(threadID, opID, "a long enough question here"), msg("1300000000000000002", opID, "more")), threadID, "superseded"},
		// An image-only post: the title alone is under the minimum once it's short.
		{"too short", func() threadView { v := view(msg(threadID, opID, "")); v.title = "help"; return v }(), threadID, "too_short"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if d := decide(tt.v, tt.trigger, blocked, testLimits); d.skip != tt.want {
				t.Errorf("skip = %q, want %q", d.skip, tt.want)
			}
		})
	}
}

// The poster writes again while the first answer is being generated. That answer is posted after
// the follow-up but doesn't reply to it, so the follow-up still gets its own answer.
func TestDecideFollowUpDuringGeneration(t *testing.T) {
	followUp := "1300000000000000001"
	v := view(
		msg(threadID, opID, "the bot won't mute anyone"),
		msg(followUp, opID, "oh and I'm using the Capture Mod"),
		botReply("1300000000000000002", botID, "Check the voice channel permissions.\n\n"+footerAnswer, threadID),
	)
	d := decide(v, followUp, nil, testLimits)
	if d.skip != "" {
		t.Fatalf("skip = %q, want the follow-up answered", d.skip)
	}
	if d.replyTo != followUp || d.first {
		t.Errorf("replyTo = %q first = %t, want a reply to the follow-up that isn't a first answer", d.replyTo, d.first)
	}
	// The model sees the follow-up after the answer it already gave.
	if n := len(d.turns); n != 3 || d.turns[n-1].text != "oh and I'm using the Capture Mod" {
		t.Errorf("turns = %+v", d.turns)
	}
}

// Same, with an answer long enough to be split: only its first message is a reply.
func TestDecideFollowUpDuringSplitAnswer(t *testing.T) {
	followUp := "1300000000000000001"
	v := view(
		msg(threadID, opID, "the bot won't mute anyone"),
		msg(followUp, opID, "oh and I'm using the Capture Mod"),
		botReply("1300000000000000002", botID, "First part.", threadID),
		msg("1300000000000000003", botID, "Second part.\n\n"+footerAnswer),
	)
	d := decide(v, followUp, nil, testLimits)
	if d.skip != "" {
		t.Fatalf("skip = %q, want the follow-up answered", d.skip)
	}
	if n := len(d.turns); n != 3 || decodeAssistant(d.turns[1].text) != "First part.\nSecond part." {
		t.Errorf("turns = %+v, want the split answer as one assistant turn before the follow-up", d.turns)
	}
}

// An answer whose question was deleted is posted without the reply, and still counts as answering it.
func TestDecideAnswerWithoutReply(t *testing.T) {
	v := view(msg(threadID, opID, "the bot won't mute anyone"), msg("1300000000000000002", botID, "An answer."))
	if d := decide(v, threadID, nil, testLimits); d.skip != "answered" {
		t.Errorf("skip = %q, want answered", d.skip)
	}
}

func TestDecideCountsTextAcrossMessages(t *testing.T) {
	// An image-only first post followed by text is answered once the text arrives.
	v := view(msg(threadID, opID, ""), msg("1300000000000000005", opID, "bot won't mute anyone at all"))
	v.title = "help"
	v.messages[0].Attachments = []*discordgo.MessageAttachment{{ID: "a"}}
	d := decide(v, "1300000000000000005", nil, testLimits)
	if d.skip != "" {
		t.Fatalf("skip = %q", d.skip)
	}
	if len(d.turns) != 1 || !strings.Contains(d.turns[0].text, "attached 1 file") || !strings.Contains(d.turns[0].text, "won't mute") {
		t.Errorf("turns = %+v, want one merged user turn noting the attachment", d.turns)
	}
}

func TestDecideMaxReplies(t *testing.T) {
	msgs := []*discordgo.Message{msg(threadID, opID, "a long enough question here")}
	id := uint64(1300000000000000000)
	for range 5 {
		id++
		msgs = append(msgs, msg(itoa(id), botID, "answer\n\n"+footerAnswer))
		id++
		msgs = append(msgs, msg(itoa(id), opID, "still broken"))
	}
	if d := decide(view(msgs...), itoa(id), nil, testLimits); d.skip != "max_replies" {
		t.Errorf("skip = %q, want max_replies", d.skip)
	}
}

func TestBuildTurns(t *testing.T) {
	v := view(
		msg(threadID, opID, "it says no capture linked"),
		msg("1300000000000000001", otherID, "same for me"),
		msg("1300000000000000002", botID, "Try the latest link."),
		msg("1300000000000000003", botID, "Then run it as admin.\n\n"+footerAnswer),
		msg("1300000000000000004", opID, "did both"),
		msg("1300000000000000005", opID, "still nothing"),
	)
	turns := buildTurns(v)
	if len(turns) != 3 {
		t.Fatalf("got %d turns, want 3: %+v", len(turns), turns)
	}
	if !turns[1].assistant || decodeAssistant(turns[1].text) != "Try the latest link.\nThen run it as admin." {
		t.Errorf("assistant turn = %q; other users and the footer should be left out", turns[1].text)
	}
	if turns[2].text != "did both\n\nstill nothing" {
		t.Errorf("last turn = %q, want the two poster messages merged", turns[2].text)
	}
}

func TestBuildTurnsSkipsInteractionResponses(t *testing.T) {
	ack := msg("1300000000000000002", botID, "Marked as solved")
	ack.Interaction = &discordgo.MessageInteraction{ID: "x"}
	turns := buildTurns(view(msg(threadID, opID, "question"), ack, msg("1300000000000000003", opID, "back again")))
	if len(turns) != 1 || turns[0].assistant {
		t.Errorf("turns = %+v, want only the user turn", turns)
	}
}

func TestBuildTurnsDropsLeadingBotAnswer(t *testing.T) {
	// The starter message has scrolled out of the last 100 messages.
	turns := buildTurns(view(msg("1300000000000000002", botID, "an answer"), msg("1300000000000000003", opID, "follow-up")))
	if len(turns) != 1 || turns[0].assistant {
		t.Errorf("turns = %+v, want the conversation to start with the user", turns)
	}
}

func TestSnowflakeOrdersNumerically(t *testing.T) {
	// As strings, "999…" sorts after "1000…".
	if snowflake("99999999999999999") >= snowflake("100000000000000000") {
		t.Error("snowflakes of different lengths must compare numerically")
	}
}

func itoa(n uint64) string {
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
