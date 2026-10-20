package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// requiredPermissions are what the bot needs in the forum: read and answer threads, and apply tags and
// archive (Manage Threads).
const requiredPermissions = discordgo.PermissionViewChannel | discordgo.PermissionSendMessagesInThreads |
	discordgo.PermissionReadMessageHistory | discordgo.PermissionManageThreads

type bot struct {
	s       *discordgo.Session
	cfg     config
	ask     func(context.Context, []turn) (result, error)
	limiter *limiter
	metrics *metrics

	solvedTagID, humanTagID string

	// locks serialises work per thread, so a burst of messages gets one answer.
	locksMu sync.Mutex
	locks   map[string]*sync.Mutex
}

// resolveForum checks the forum, tags, and roles at startup, so a misconfigured bot refuses to start
// instead of silently never escalating.
func (b *bot) resolveForum() error {
	forum, err := b.s.Channel(b.cfg.forumID)
	if err != nil {
		return fmt.Errorf("fetch forum channel %s: %w", b.cfg.forumID, err)
	}
	if forum.Type != discordgo.ChannelTypeGuildForum || forum.GuildID != b.cfg.guildID {
		return fmt.Errorf("channel %s is not a forum in guild %s", b.cfg.forumID, b.cfg.guildID)
	}
	for _, tag := range forum.AvailableTags {
		switch {
		case strings.EqualFold(tag.Name, b.cfg.solvedTag):
			b.solvedTagID = tag.ID
		case strings.EqualFold(tag.Name, b.cfg.humanTag):
			b.humanTagID = tag.ID
		}
	}
	if b.solvedTagID == "" || b.humanTagID == "" {
		return fmt.Errorf("forum %q needs tags named %q and %q", forum.Name, b.cfg.solvedTag, b.cfg.humanTag)
	}
	roles, err := b.s.GuildRoles(b.cfg.guildID)
	if err != nil {
		return fmt.Errorf("fetch guild roles: %w", err)
	}
	for _, id := range b.cfg.staffRoleIDs {
		if !slices.ContainsFunc(roles, func(r *discordgo.Role) bool { return r.ID == id }) {
			return fmt.Errorf("staff role %s isn't in guild %s", id, b.cfg.guildID)
		}
	}
	perms, err := b.s.UserChannelPermissions(b.s.State.User.ID, b.cfg.forumID)
	if err != nil {
		return fmt.Errorf("check permissions in the forum: %w", err)
	}
	if missing := requiredPermissions &^ perms; missing != 0 {
		return fmt.Errorf("the bot is missing permissions in the forum (bitmask %d); it needs View Channel, Send Messages in Threads, Read Message History, and Manage Threads", missing)
	}
	return nil
}

func (b *bot) onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.GuildID != b.cfg.guildID || m.Author == nil || m.Author.Bot {
		return
	}
	ch, err := s.State.Channel(m.ChannelID)
	if err != nil {
		// Threads the gateway hasn't told us about yet, such as one just unarchived by this message.
		if ch, err = s.Channel(m.ChannelID); err != nil {
			b.metrics.errors.WithLabelValues("fetch_channel").Inc()
			log.Printf("helpbot: fetch channel %s: %v", m.ChannelID, err)
			return
		}
	}
	if !ch.IsThread() || ch.ParentID != b.cfg.forumID || m.Author.ID != ch.OwnerID {
		return
	}
	if m.Member != nil && !m.Member.JoinedAt.IsZero() && time.Since(m.Member.JoinedAt) < b.cfg.minMemberAge {
		b.skip(ch.ID, "new_member")
		return
	}
	go b.handle(m.Message, ch.ID)
}

func (b *bot) handle(trigger *discordgo.Message, threadID string) {
	time.Sleep(b.cfg.debounce)
	lock := b.lock(threadID)
	lock.Lock()
	defer lock.Unlock()

	view, err := b.view(threadID)
	if err != nil {
		b.metrics.errors.WithLabelValues("fetch_thread").Inc()
		log.Printf("helpbot: thread %s: %v", threadID, err)
		return
	}
	d := decide(view, trigger.ID, []string{b.solvedTagID, b.humanTagID}, limits{minChars: b.cfg.minChars, maxReplies: b.cfg.maxReplies})
	if d.skip != "" {
		b.skip(threadID, d.skip)
		return
	}
	if reason := b.limiter.check(view.ownerID, d.first); reason != "" {
		b.skip(threadID, reason)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	stopTyping := b.typing(ctx, threadID)
	res, err := b.ask(ctx, d.turns)
	stopTyping()
	if res.Model != "" {
		b.metrics.recordUsage(res.Model, res.Usage)
		b.limiter.record(view.ownerID, d.first, cost(res.Model, res.Usage))
	}
	if err != nil {
		stage := "api"
		if errors.Is(err, errNoAnswer) {
			stage = "no_answer"
		}
		b.metrics.errors.WithLabelValues(stage).Inc()
		log.Printf("helpbot: thread %s: ask: %v", threadID, err)
		// Better a human sees it than the poster waits on nothing.
		b.post(threadID, []*discordgo.MessageSend{{
			Content:         "Sorry, I couldn't answer this one. I've tagged the thread so someone from the AutoMuteUs team can take a look.",
			AllowedMentions: noMentions(),
			Reference:       replyRef(threadID, d.replyTo),
		}})
		b.handoff(threadID, "error")
		return
	}

	if !b.post(threadID, answerMessages(res.Answer, res.NeedsHuman, threadID, d.replyTo)) {
		return
	}
	b.metrics.answers.WithLabelValues(res.Model, fmt.Sprint(res.NeedsHuman)).Inc()
	log.Printf("helpbot: thread %s: answered (model=%s first=%t handoff=%t reason=%s in=%d out=%d cache_read=%d cache_write=%d $%.4f)",
		threadID, res.Model, d.first, res.NeedsHuman, res.Reason, res.Usage.Input, res.Usage.Output, res.Usage.CacheRead, res.Usage.CacheWrite, cost(res.Model, res.Usage))
	if res.NeedsHuman {
		b.handoff(threadID, res.Reason)
	}
}

// view rebuilds the thread from Discord: its tags, its last 100 messages, and whether staff posted.
func (b *bot) view(threadID string) (threadView, error) {
	ch, err := b.s.Channel(threadID)
	if err != nil {
		return threadView{}, fmt.Errorf("fetch thread: %w", err)
	}
	msgs, err := b.s.ChannelMessages(threadID, 100, "", "", "")
	if err != nil {
		return threadView{}, fmt.Errorf("fetch messages: %w", err)
	}
	slices.Reverse(msgs) // Discord returns newest first
	v := threadView{id: ch.ID, ownerID: ch.OwnerID, title: ch.Name, tags: ch.AppliedTags, messages: msgs, botID: b.s.State.User.ID}
	checked := map[string]bool{}
	for _, m := range msgs {
		if m.Author == nil || m.Author.Bot || m.Author.ID == v.ownerID || checked[m.Author.ID] {
			continue
		}
		checked[m.Author.ID] = true
		staff, err := b.isStaff(m.Author.ID)
		if err != nil {
			return v, fmt.Errorf("look up member %s: %w", m.Author.ID, err)
		}
		if staff {
			v.staffPosted = true
			break
		}
	}
	return v, nil
}

func (b *bot) isStaff(userID string) (bool, error) {
	member, err := b.s.State.Member(b.cfg.guildID, userID)
	if err != nil {
		if member, err = b.s.GuildMember(b.cfg.guildID, userID); err != nil {
			var rest *discordgo.RESTError
			if errors.As(err, &rest) && rest.Message != nil && rest.Message.Code == discordgo.ErrCodeUnknownMember {
				return false, nil // left the server
			}
			return false, err
		}
		_ = b.s.State.MemberAdd(member)
	}
	return b.hasStaffRole(member.Roles), nil
}

func (b *bot) hasStaffRole(roles []string) bool {
	return slices.ContainsFunc(roles, func(r string) bool { return slices.Contains(b.cfg.staffRoleIDs, r) })
}

func (b *bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent || i.GuildID != b.cfg.guildID || i.Member == nil || i.Member.User == nil {
		return
	}
	id := i.MessageComponentData().CustomID
	if id != buttonSolved && id != buttonHuman {
		return
	}
	ch, err := s.Channel(i.ChannelID)
	if err != nil {
		b.metrics.errors.WithLabelValues("fetch_channel").Inc()
		log.Printf("helpbot: button: fetch channel %s: %v", i.ChannelID, err)
		return
	}
	if i.Member.User.ID != ch.OwnerID && !b.hasStaffRole(i.Member.Roles) {
		b.respond(i, "Only the person who asked can use these buttons.", true)
		return
	}
	b.metrics.buttons.WithLabelValues(strings.TrimPrefix(id, "helpbot:")).Inc()
	log.Printf("helpbot: thread %s: button %s", ch.ID, id)
	if id == buttonSolved {
		b.respond(i, "✅ Glad that helped! I've marked this thread as solved. If the problem comes back, open a new post.", false)
		b.dropButtons(i)
		if b.addTag(ch.ID, b.solvedTagID) {
			archived := true
			if _, err := s.ChannelEditComplex(ch.ID, &discordgo.ChannelEdit{Archived: &archived}); err != nil {
				b.metrics.errors.WithLabelValues("archive").Inc()
				log.Printf("helpbot: thread %s: archive: %v", ch.ID, err)
			}
		}
		return
	}
	b.respond(i, "🙋 I've tagged this thread for the AutoMuteUs team. Someone will take a look when they can; please don't ping Moderators.", false)
	b.dropButtons(i)
	b.handoff(ch.ID, "button")
}

func (b *bot) respond(i *discordgo.InteractionCreate, content string, ephemeral bool) {
	data := &discordgo.InteractionResponseData{Content: content, AllowedMentions: noMentions()}
	if ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}
	if err := b.s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: data}); err != nil {
		b.metrics.errors.WithLabelValues("respond").Inc()
		log.Printf("helpbot: respond to button: %v", err)
	}
}

// dropButtons removes the buttons from the answer that was clicked, so they can't be pressed twice.
func (b *bot) dropButtons(i *discordgo.InteractionCreate) {
	if i.Message == nil {
		return
	}
	empty := []discordgo.MessageComponent{}
	if _, err := b.s.ChannelMessageEditComplex(&discordgo.MessageEdit{ID: i.Message.ID, Channel: i.ChannelID, Components: empty}); err != nil {
		log.Printf("helpbot: remove buttons: %v", err)
	}
}

func (b *bot) handoff(threadID, reason string) {
	if b.addTag(threadID, b.humanTagID) {
		b.metrics.handoffs.WithLabelValues(reason).Inc()
	}
}

// addTag applies a forum tag, keeping the ones already there.
func (b *bot) addTag(threadID, tagID string) bool {
	ch, err := b.s.Channel(threadID)
	if err == nil {
		if slices.Contains(ch.AppliedTags, tagID) {
			return true
		}
		tags := append(slices.Clone(ch.AppliedTags), tagID)
		_, err = b.s.ChannelEditComplex(threadID, &discordgo.ChannelEdit{AppliedTags: &tags})
	}
	if err != nil {
		// Usually a missing Manage Threads permission, or a thread already at Discord's 5-tag limit.
		b.metrics.errors.WithLabelValues("tag").Inc()
		log.Printf("helpbot: thread %s: apply tag %s: %v", threadID, tagID, err)
		return false
	}
	return true
}

func (b *bot) post(threadID string, sends []*discordgo.MessageSend) bool {
	for _, send := range sends {
		_, err := b.s.ChannelMessageSendComplex(threadID, send)
		if err != nil && send.Reference != nil {
			// Most likely the poster deleted the message being replied to. discordgo v0.27 can't ask
			// Discord to ignore that, so post without the reply; a later message still gets answered.
			log.Printf("helpbot: thread %s: post as reply: %v; posting without the reply", threadID, err)
			send.Reference = nil
			_, err = b.s.ChannelMessageSendComplex(threadID, send)
		}
		if err != nil {
			b.metrics.errors.WithLabelValues("post").Inc()
			log.Printf("helpbot: thread %s: post: %v", threadID, err)
			return false
		}
	}
	return true
}

func (b *bot) skip(threadID, reason string) {
	b.metrics.skips.WithLabelValues(reason).Inc()
	log.Printf("helpbot: thread %s: skipped (%s)", threadID, reason)
}

// typing shows "typing…" until stopped; Discord clears it after about 10 seconds, so it is renewed.
func (b *bot) typing(ctx context.Context, threadID string) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(8 * time.Second)
		defer ticker.Stop()
		for {
			_ = b.s.ChannelTyping(threadID)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return cancel
}

func (b *bot) lock(threadID string) *sync.Mutex {
	b.locksMu.Lock()
	defer b.locksMu.Unlock()
	l, ok := b.locks[threadID]
	if !ok {
		l = &sync.Mutex{}
		b.locks[threadID] = l
	}
	return l
}
