package task

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
)

// The job queue is a per-connect-code Redis list. Galactus (internal/broker) pushes capture events onto it and the
// bot's per-game subscriber pops them in order, woken by a pub/sub notify. Acks flow the other way on a second channel.

const testCode = "ABCDEFGH"

func newQueue(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func listKey(code string) string { return rediskey.JobNamespace + code }

// receive waits for one message on the subscription, failing the test on timeout.
func receive(t *testing.T, sub *redis.PubSub) *redis.Message {
	t.Helper()
	select {
	case msg := <-sub.Channel():
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pub/sub message")
		return nil
	}
}

// subscribe opens a subscription and waits until Redis confirms it, so a following publish is not lost.
func subscribe(t *testing.T, client *redis.Client, open func(context.Context, *redis.Client, string) *redis.PubSub) *redis.PubSub {
	t.Helper()
	sub := open(context.Background(), client, testCode)
	if _, err := sub.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	return sub
}

func TestJobType_String(t *testing.T) {
	cases := map[JobType]string{
		ConnectionJob: "connection",
		LobbyJob:      "lobby",
		StateJob:      "state",
		PlayerJob:     "player",
		GameOverJob:   "gameover",
		JobType(99):   "unknown",
	}
	for jt, want := range cases {
		if got := jt.String(); got != want {
			t.Errorf("JobType(%d).String() = %q, want %q", jt, got, want)
		}
	}
}

func TestPushJob_AppendsJSONToPerCodeListWithTTL(t *testing.T) {
	mr, client := newQueue(t)

	if err := PushJob(context.Background(), client, testCode, PlayerJob, `{"Name":"red"}`); err != nil {
		t.Fatal(err)
	}

	entries, err := mr.List(listKey(testCode))
	if err != nil || len(entries) != 1 {
		t.Fatalf("list = %v, %v; want one entry", entries, err)
	}
	var stored map[string]interface{}
	if err := json.Unmarshal([]byte(entries[0]), &stored); err != nil {
		t.Fatalf("entry is not JSON: %v", err)
	}
	if stored["type"] != float64(PlayerJob) || stored["payload"] != `{"Name":"red"}` {
		t.Fatalf("stored entry = %v", stored)
	}
	if ttl := mr.TTL(listKey(testCode)); ttl != JobTTLSeconds*time.Second {
		t.Fatalf("TTL = %v, want %v", ttl, JobTTLSeconds*time.Second)
	}
}

// The TTL is set only when the push creates the list. Later pushes do not refresh it, so a queue that is never
// fully drained expires an hour after its first job regardless of activity. Draining it lets the next push start a
// fresh hour.
func TestPushJob_TTLSetOnCreationNotRefreshedUntilDrained(t *testing.T) {
	mr, client := newQueue(t)
	ctx := context.Background()

	_ = PushJob(ctx, client, testCode, StateJob, "1")
	mr.FastForward(100 * time.Second)
	_ = PushJob(ctx, client, testCode, StateJob, "2")
	if ttl := mr.TTL(listKey(testCode)); ttl != (JobTTLSeconds-100)*time.Second {
		t.Fatalf("TTL after second push = %v; expected it untouched at %v", ttl, (JobTTLSeconds-100)*time.Second)
	}

	if _, err := PopJob(ctx, client, testCode); err != nil {
		t.Fatal(err)
	}
	if _, err := PopJob(ctx, client, testCode); err != nil {
		t.Fatal(err)
	}
	if mr.Exists(listKey(testCode)) {
		t.Fatal("drained list should be gone")
	}

	_ = PushJob(ctx, client, testCode, StateJob, "3")
	if ttl := mr.TTL(listKey(testCode)); ttl != JobTTLSeconds*time.Second {
		t.Fatalf("TTL after re-creation = %v, want a fresh %v", ttl, JobTTLSeconds*time.Second)
	}
}

func TestPushJob_NotifiesSubscribersForThatCode(t *testing.T) {
	_, client := newQueue(t)
	sub := subscribe(t, client, Subscribe)

	if err := PushJob(context.Background(), client, "OTHERCOD", StateJob, "1"); err != nil {
		t.Fatal(err)
	}
	if err := PushJob(context.Background(), client, testCode, StateJob, "1"); err != nil {
		t.Fatal(err)
	}

	msg := receive(t, sub)
	if msg.Channel != listKey(testCode)+":notify" {
		t.Fatalf("notified on %q", msg.Channel)
	}
	// the publish passes a Go bool, which go-redis encodes as "1"; consumers treat the message purely as a wake signal
	if msg.Payload != "1" {
		t.Fatalf("payload = %q, want %q", msg.Payload, "1")
	}
	select {
	case extra := <-sub.Channel():
		t.Fatalf("unexpected extra notification: %+v", extra)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestPushJob_ReturnsRedisError(t *testing.T) {
	mr, client := newQueue(t)
	mr.SetError("boom")
	if err := PushJob(context.Background(), client, testCode, StateJob, "1"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want the Redis error", err)
	}
}

func TestPushPop_IsFIFOAndPayloadRoundTripsAsString(t *testing.T) {
	_, client := newQueue(t)
	ctx := context.Background()

	pushed := []Job{
		{ConnectionJob, "true"},
		{LobbyJob, `{"LobbyCode":"QWERTY"}`},
		{StateJob, "2"},
		{PlayerJob, `{"Name":"red","Color":0}`},
		{GameOverJob, `{"GameOverReason":1}`},
	}
	for _, j := range pushed {
		if err := PushJob(ctx, client, testCode, j.JobType, j.Payload.(string)); err != nil {
			t.Fatal(err)
		}
	}

	for i, want := range pushed {
		got, err := PopJob(ctx, client, testCode)
		if err != nil {
			t.Fatalf("pop %d: %v", i, err)
		}
		if got.JobType != want.JobType {
			t.Fatalf("pop %d: type = %v, want %v", i, got.JobType, want.JobType)
		}
		// consumers type-assert Payload to string; the JSON round trip must preserve that
		payload, ok := got.Payload.(string)
		if !ok || payload != want.Payload {
			t.Fatalf("pop %d: payload = %#v, want string %q", i, got.Payload, want.Payload)
		}
	}
}

func TestPopJob_EmptyQueueReturnsRedisNil(t *testing.T) {
	_, client := newQueue(t)
	job, err := PopJob(context.Background(), client, testCode)
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("err = %v, want redis.Nil", err)
	}
	if job != (Job{}) {
		t.Fatalf("job = %+v, want zero value", job)
	}
}

func TestPopJob_CorruptEntryReturnsErrorAndConsumesIt(t *testing.T) {
	mr, client := newQueue(t)
	ctx := context.Background()
	mr.Push(listKey(testCode), "not json")
	_ = PushJob(ctx, client, testCode, StateJob, "1")

	if _, err := PopJob(ctx, client, testCode); err == nil {
		t.Fatal("expected a JSON error for the corrupt entry")
	}
	// the bad entry is gone; the queue keeps working
	job, err := PopJob(ctx, client, testCode)
	if err != nil || job.JobType != StateJob {
		t.Fatalf("next pop = %+v, %v", job, err)
	}
}

func TestQueues_AreIsolatedPerConnectCode(t *testing.T) {
	_, client := newQueue(t)
	ctx := context.Background()
	_ = PushJob(ctx, client, testCode, StateJob, "a")
	_ = PushJob(ctx, client, "OTHERCOD", StateJob, "b")

	job, _ := PopJob(ctx, client, "OTHERCOD")
	if job.Payload != "b" {
		t.Fatalf("got %+v from the other queue", job)
	}
	if _, err := PopJob(ctx, client, "OTHERCOD"); !errors.Is(err, redis.Nil) {
		t.Fatal("other queue should now be empty")
	}
	job, _ = PopJob(ctx, client, testCode)
	if job.Payload != "a" {
		t.Fatalf("got %+v from the first queue", job)
	}
}

func TestAck_PublishesOnAckChannelOnly(t *testing.T) {
	_, client := newQueue(t)
	ackSub := subscribe(t, client, AckSubscribe)
	notifySub := subscribe(t, client, Subscribe)

	Ack(context.Background(), client, testCode)

	msg := receive(t, ackSub)
	if msg.Channel != listKey(testCode)+":ack" || msg.Payload != "1" {
		t.Fatalf("ack message = %+v", msg)
	}
	select {
	case extra := <-notifySub.Channel():
		t.Fatalf("ack leaked onto the notify channel: %+v", extra)
	case <-time.After(50 * time.Millisecond):
	}
}

// --- modify ---------------------------------------------------------------------------------------------------------

func TestNewModifyTask_CarriesParametersWithUniqueHexIDs(t *testing.T) {
	hexID := regexp.MustCompile(`^[0-9a-f]{16}$`)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		task := NewModifyTask(1, 2, PatchParams{Mute: true, Deaf: false})
		if task.GuildID != 1 || task.UserID != 2 || !task.Parameters.Mute || task.Parameters.Deaf {
			t.Fatalf("task fields = %+v", task)
		}
		if !hexID.MatchString(task.TaskID) {
			t.Fatalf("TaskID %q is not 16 hex chars", task.TaskID)
		}
		if seen[task.TaskID] {
			t.Fatalf("duplicate TaskID %q", task.TaskID)
		}
		seen[task.TaskID] = true
	}
}

func TestModifyTask_JSONShape(t *testing.T) {
	b, err := json.Marshal(ModifyTask{GuildID: 1, UserID: 2, Parameters: PatchParams{Deaf: true}, TaskID: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"guildID":1,"userID":2,"parameters":{"deaf":true,"mute":false},"taskID":"abc"}`
	if string(b) != want {
		t.Fatalf("json = %s\nwant   %s", b, want)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fakeDiscord(t *testing.T, status int, captured *http.Request, body *[]byte) *discordgo.Session {
	t.Helper()
	sess, err := discordgo.New("Bot fake")
	if err != nil {
		t.Fatal(err)
	}
	sess.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*captured = *r
		b, _ := io.ReadAll(r.Body)
		*body = b
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Request:    r,
		}, nil
	})}
	return sess
}

func TestApplyMuteDeaf_PatchesGuildMemberVoiceState(t *testing.T) {
	var req http.Request
	var body []byte
	sess := fakeDiscord(t, http.StatusNoContent, &req, &body)

	if err := ApplyMuteDeaf(sess, "1", "2", false, true); err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPatch {
		t.Fatalf("method = %s, want PATCH", req.Method)
	}
	if !strings.HasSuffix(req.URL.Path, "/guilds/1/members/2") {
		t.Fatalf("path = %s", req.URL.Path)
	}
	if got := req.Header.Get("Authorization"); got != "Bot fake" {
		t.Fatalf("Authorization = %q", got)
	}
	var params PatchParams
	if err := json.Unmarshal(body, &params); err != nil {
		t.Fatalf("body %q is not PatchParams: %v", body, err)
	}
	if params.Mute || !params.Deaf {
		t.Fatalf("body = %+v, want mute=false deaf=true", params)
	}
}

func TestApplyMuteDeaf_ReturnsRESTErrorOnFailure(t *testing.T) {
	var req http.Request
	var body []byte
	sess := fakeDiscord(t, http.StatusForbidden, &req, &body)

	err := ApplyMuteDeaf(sess, "1", "2", true, true)
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) {
		t.Fatalf("err = %v, want *discordgo.RESTError", err)
	}
	if restErr.Response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", restErr.Response.StatusCode)
	}
}
