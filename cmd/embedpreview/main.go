// embedpreview sends the bot's game status embeds, rendered from sample data, to a Discord channel so they can be
// reviewed without running the bot. It can post through a webhook (no bot needed) or with a bot token.
//
//	go run ./cmd/embedpreview -webhook https://discord.com/api/webhooks/<id>/<token>
//	go run ./cmd/embedpreview -token <bot token> -channel <channel id> -severity warning -message "Expect some lag"
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/automuteus/automuteus/v8/bot"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
)

var webhookURL = regexp.MustCompile(`^https://(?:[a-z]+\.)?discord(?:app)?\.com/api/(?:v\d+/)?webhooks/(\d+)/([\w-]+)$`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		webhook  = flag.String("webhook", "", "Discord webhook URL to post through (no bot token needed)")
		token    = flag.String("token", os.Getenv("DISCORD_BOT_TOKEN"), "bot token to post with (or DISCORD_BOT_TOKEN); requires -channel")
		channel  = flag.String("channel", "", "channel ID to post to when using -token")
		severity = flag.String("severity", "", "render every embed with an active notice of this severity: warning or critical")
		message  = flag.String("message", "This is a preview notice.", "notice text to render when -severity is set")
		only     = flag.String("only", "", "comma-separated preview names to send (default: all)")
		list     = flag.Bool("list", false, "print the preview names and exit")
	)
	flag.Parse()

	var n *notice.Notice
	if *severity != "" {
		sev := notice.Severity(strings.ToLower(*severity))
		if !sev.Valid() {
			return errors.New("severity must be warning or critical")
		}
		n = &notice.Notice{Severity: sev, Message: *message}
	}

	previews := bot.PreviewEmbeds(settings.MakeGuildSettings(), n)
	if *list {
		for _, p := range previews {
			fmt.Println(p.Name)
		}
		return nil
	}
	wanted := map[string]bool{}
	for _, name := range strings.Split(*only, ",") {
		if name = strings.TrimSpace(name); name != "" {
			wanted[name] = true
		}
	}

	send, err := sender(*webhook, *token, *channel)
	if err != nil {
		return err
	}
	for _, p := range previews {
		if len(wanted) > 0 && !wanted[p.Name] {
			continue
		}
		if err := send(p); err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
		fmt.Println("sent", p.Name)
		time.Sleep(300 * time.Millisecond)
	}
	return nil
}

func sender(webhook, token, channel string) (func(bot.PreviewEmbed) error, error) {
	switch {
	case webhook != "":
		m := webhookURL.FindStringSubmatch(webhook)
		if m == nil {
			return nil, errors.New("unrecognized webhook URL")
		}
		sess, err := discordgo.New("")
		if err != nil {
			return nil, err
		}
		return func(p bot.PreviewEmbed) error {
			_, err := sess.WebhookExecute(m[1], m[2], true, &discordgo.WebhookParams{
				Username: "AutoMuteUs preview",
				Content:  "**" + p.Name + "**",
				Embeds:   []*discordgo.MessageEmbed{p.Embed},
			})
			return err
		}, nil
	case token != "" && channel != "":
		sess, err := discordgo.New("Bot " + token)
		if err != nil {
			return nil, err
		}
		return func(p bot.PreviewEmbed) error {
			_, err := sess.ChannelMessageSendComplex(channel, &discordgo.MessageSend{
				Content: "**" + p.Name + "**",
				Embed:   p.Embed,
			})
			return err
		}, nil
	default:
		return nil, errors.New("provide -webhook, or -token and -channel")
	}
}
