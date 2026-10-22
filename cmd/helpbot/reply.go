package main

import (
	"strings"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

const (
	// Discord's limit on message content, in characters.
	messageLimit = 2000

	footerAnswer  = "-# AI answer, may be wrong. Press 🙋 for a human."
	footerHandoff = "-# AI answer, may be wrong. This thread is tagged for the AutoMuteUs team."

	buttonSolved = "helpbot:solved"
	buttonHuman  = "helpbot:human"
)

// answerMessages renders an answer as the messages to post: split to Discord's limit, the first one a
// reply to replyTo, the footer at the end, and on ordinary answers the two buttons on the last message.
func answerMessages(answer string, handoff bool, threadID, replyTo string) []*discordgo.MessageSend {
	footer := footerAnswer
	if handoff {
		footer = footerHandoff
	}
	chunks := splitMessage(strings.TrimSpace(answer)+"\n\n"+footer, messageLimit)
	sends := make([]*discordgo.MessageSend, len(chunks))
	for i, c := range chunks {
		sends[i] = &discordgo.MessageSend{Content: c, AllowedMentions: noMentions()}
	}
	sends[0].Reference = replyRef(threadID, replyTo)
	if !handoff {
		sends[len(sends)-1].Components = answerButtons()
	}
	return sends
}

func answerButtons() []discordgo.MessageComponent {
	return []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{Label: "Solved", Emoji: discordgo.ComponentEmoji{Name: "✅"}, Style: discordgo.SuccessButton, CustomID: buttonSolved},
		discordgo.Button{Label: "Need a human", Emoji: discordgo.ComponentEmoji{Name: "🙋"}, Style: discordgo.SecondaryButton, CustomID: buttonHuman},
	}}}
}

func replyRef(threadID, replyTo string) *discordgo.MessageReference {
	if replyTo == "" {
		return nil
	}
	return &discordgo.MessageReference{MessageID: replyTo, ChannelID: threadID}
}

// noMentions stops every message from pinging anyone, whatever the text contains.
func noMentions() *discordgo.MessageAllowedMentions {
	return &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}
}

// stripFooter removes either footer from a rebuilt bot answer.
func stripFooter(text string) string {
	text = strings.TrimSpace(text)
	for _, f := range []string{footerAnswer, footerHandoff} {
		text = strings.TrimSpace(strings.TrimSuffix(text, f))
	}
	return text
}

// splitMessage cuts text into chunks of at most limit characters, preferring line breaks, then spaces.
// A chunk that ends inside a code block closes it, and the next chunk reopens it.
func splitMessage(text string, limit int) []string {
	const fence = "```"
	var chunks []string
	for utf8.RuneCountInString(text) > limit {
		// Room for a closing fence if this chunk needs one.
		window := []rune(text)[:limit-len(fence)-1]
		cut := len(string(window))
		if i := strings.LastIndex(string(window), "\n"); i > 0 {
			cut = i
		} else if i := strings.LastIndex(string(window), " "); i > 0 {
			cut = i
		}
		chunk, rest := strings.TrimRight(text[:cut], " \n"), strings.TrimLeft(text[cut:], " \n")
		if strings.Count(chunk, fence)%2 == 1 {
			chunk += "\n" + fence
			rest = fence + "\n" + rest
		}
		chunks = append(chunks, chunk)
		text = rest
	}
	return append(chunks, text)
}
