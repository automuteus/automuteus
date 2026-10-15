package broker

import (
	"context"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/capture"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/go-redis/redis/v8"
)

// Run a takeover immediately before release's first Redis write. With separate GET/RPUSH commands, this is after
// the old holder was read; with an atomic release script, it is before the ownership check and write together.
type beforeReleaseWrite struct {
	once sync.Once
	run  func()
}

func (h *beforeReleaseWrite) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	switch cmd.Name() {
	case "rpush", "evalsha", "eval":
		h.once.Do(h.run)
	}
	return ctx, nil
}

func (*beforeReleaseWrite) AfterProcess(context.Context, redis.Cmder) error { return nil }
func (*beforeReleaseWrite) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}
func (*beforeReleaseWrite) AfterProcessPipeline(context.Context, []redis.Cmder) error { return nil }

func TestRelease_ClaimDuringDisconnectLeavesNewCaptureConnected(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	old, next := NewBroker(mr.Addr(), "", ""), NewBroker(mr.Addr(), "", "")
	t.Cleanup(func() { old.client.Close() })
	t.Cleanup(func() { next.client.Close() })
	const code = "ABCDEFGH"
	old.claim(ctx, &fakeConn{}, "old", code)
	connected := capture.ConnectionStatus{Connected: true, Kind: capture.KindMod, Version: "1.0.0"}.Payload()
	tookOver := false
	old.client.AddHook(&beforeReleaseWrite{run: func() {
		next.claim(ctx, &fakeConn{}, "new", code)
		if err := task.PushJob(ctx, next.client, code, task.ConnectionJob, connected); err != nil {
			t.Fatal(err)
		}
		if err := next.client.Set(ctx, rediskey.CaptureMuteReady(code), "1", CaptureReadyTTL).Err(); err != nil {
			t.Fatal(err)
		}
		tookOver = true
	}})

	if old.release(ctx, "old") {
		t.Error("old disconnect unlinked the newly connected Capture")
	}
	if !tookOver {
		t.Fatal("release did not reach the takeover hook")
	}
	jobs, err := mr.List(rediskey.JobNamespace + code)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Errorf("queue contains %d jobs, want only the newcomer's connection: %v", len(jobs), jobs)
	} else if job, err := task.ParseJob(jobs[0]); err != nil || job.JobType != task.ConnectionJob || job.Payload != connected {
		t.Errorf("connection job = %+v, %v; want the newcomer's identity", job, err)
	}
	if !mr.Exists(rediskey.CaptureMuteReady(code)) {
		t.Error("old disconnect deleted the newcomer's mute readiness")
	}
}
