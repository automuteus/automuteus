package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/go-redis/redis/v8"
)

// TestSIGTERMAnnouncesShutdown builds the real galactus binary, runs it against an in-process Redis, sends it the
// signal Kubernetes and docker stop send, and checks that a critical notice reaches the bots and the process exits
// cleanly within a typical termination grace period.
func TestSIGTERMAnnouncesShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the galactus binary")
	}
	bin := filepath.Join(t.TempDir(), "galactus")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	mr := miniredis.RunT(t)
	port := freePort(t)
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "REDIS_ADDR="+mr.Addr(), "BROKER_PORT="+port, "DRAIN_SECONDS=1")
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	waitForHTTP(t, "http://127.0.0.1:"+port+"/")
	if code := httpStatus(t, "http://127.0.0.1:"+port+"/ready"); code != http.StatusOK {
		t.Fatalf("/ready before shutdown = %d", code)
	}

	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	sub := notice.Subscribe(ctx, client)
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	// during the drain window the process is still up and serving, but reports not-ready so the orchestrator stops
	// routing new capture clients here; liveness stays healthy so it is not killed early
	drainDeadline := time.Now().Add(900 * time.Millisecond)
	for httpStatus(t, "http://127.0.0.1:"+port+"/ready") != http.StatusServiceUnavailable {
		if time.Now().After(drainDeadline) {
			t.Fatalf("/ready never reported draining after SIGTERM; output:\n%s", output.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if code := httpStatus(t, "http://127.0.0.1:"+port+"/"); code != http.StatusOK {
		t.Errorf("liveness during drain = %d, want 200", code)
	}

	select {
	case msg := <-sub.Channel():
		e, err := notice.DecodeEvent([]byte(msg.Payload))
		if err != nil || e.Shutdown == nil || e.NoticeChanged {
			t.Fatalf("published event = %+v, %v", e, err)
		}
		// no capture clients were connected, so the shutdown names no games
		if len(e.Shutdown.ConnectCodes) != 0 {
			t.Fatalf("shutdown with no clients should name no games, got %+v", e.Shutdown.ConnectCodes)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("no notice published after SIGTERM; output:\n%s", output.String())
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		if err != nil {
			t.Fatalf("galactus exited with %v; output:\n%s", err, output.String())
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("galactus did not exit within the grace period; output:\n%s", output.String())
	}

	if active, err := notice.Active(ctx, client); err != nil || active != nil {
		t.Fatalf("a shutdown must not leave a platform-wide active notice, got %+v, %v", active, err)
	}
}

// TestDockerfileKeepsGalactusAsPID1 guards the container contract the shutdown notice depends on: the binary must be
// the container's entrypoint in exec form, so the orchestrator's SIGTERM reaches it rather than a shell.
func TestDockerfileKeepsGalactusAsPID1(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile.galactus"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?m)^ENTRYPOINT \["\./galactus"\]`).Match(b) {
		t.Error("Dockerfile.galactus must use exec-form ENTRYPOINT so galactus is PID 1 and receives SIGTERM")
	}
	if regexp.MustCompile(`(?m)^(ENTRYPOINT|CMD) [^\[]`).Match(b) {
		t.Error("shell-form ENTRYPOINT/CMD would put a shell at PID 1 and swallow SIGTERM")
	}
	if !bytes.Contains(b, []byte("STOPSIGNAL SIGTERM")) {
		t.Error("Dockerfile.galactus should declare STOPSIGNAL SIGTERM")
	}
}

func httpStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

func waitForHTTP(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("galactus never became ready")
}
