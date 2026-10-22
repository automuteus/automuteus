// Command helpbot answers questions in the official server's #help forum with Claude, using the facts
// in knowledge.md. See HELPBOT_PLAN.md for the design and cmd/helpbot/README.md for setup.
//
// "helpbot eval" runs the questions in eval/questions.txt through one or more models instead.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/automuteus/automuteus/v8/pkg/logging"
	"github.com/bwmarrin/discordgo"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	logging.Setup(os.Stdout)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	if len(os.Args) > 1 && os.Args[1] == "eval" {
		err = runEval(ctx, os.Args[2:])
	} else {
		err = run(ctx)
	}
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := configFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	log.Printf("helpbot %s-%s (model=%s effort=%s budget=$%.2f/day)", version, commit, cfg.model, cfg.effort, cfg.dailyBudget)

	s, err := discordgo.New("Bot " + cfg.discordToken)
	if err != nil {
		return err
	}
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent
	// The gateway sends the guild after READY; startup checks need it in state for permissions.
	guildReady := make(chan struct{})
	var once sync.Once
	s.AddHandler(func(_ *discordgo.Session, g *discordgo.GuildCreate) {
		if g.ID == cfg.guildID {
			once.Do(func() { close(guildReady) })
		}
	})
	if err := s.Open(); err != nil {
		return fmt.Errorf("connect to Discord: %w", err)
	}
	defer s.Close()
	select {
	case <-guildReady:
	case <-time.After(30 * time.Second):
		return fmt.Errorf("the bot isn't in guild %s, or Discord didn't send it within 30s", cfg.guildID)
	case <-ctx.Done():
		return nil
	}

	m := newMetrics()
	ans := newAnswerer(cfg.model, cfg.effort)
	b := &bot{s: s, cfg: cfg, ask: ans.ask, limiter: newLimiter(cfg.threadsPerUser, cfg.dailyBudget, time.Now), metrics: m, locks: map[string]*sync.Mutex{}}
	if err := b.resolveForum(); err != nil {
		return err
	}
	s.AddHandler(b.onMessage)
	s.AddHandler(b.onInteraction)
	log.Printf("helpbot: answering in forum %s as %s", cfg.forumID, s.State.User.Username)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if !s.DataReady {
			http.Error(w, "not connected to Discord", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	server := &http.Server{Addr: net.JoinHostPort("", cfg.port), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()

	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
