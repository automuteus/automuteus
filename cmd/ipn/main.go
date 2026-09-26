// Command ipn receives PayPal Instant Payment Notifications at /paypal-ipn and keeps guild premium in step with them.
// The payment tables in storage/payments.sql must be applied and granted first; the listener never runs DDL.
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
	"strconv"
	"syscall"
	"time"

	"github.com/automuteus/automuteus/v8/internal/ipn"
	"github.com/automuteus/automuteus/v8/pkg/logging"
	"github.com/automuteus/automuteus/v8/pkg/notice"
	"github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	version = "dev"
	commit  = "none"
)

type config struct {
	postgresURL string
	receiver    string
	port        string
	sandbox     bool
	// redis is optional: with an address, premium changes are announced to the API's stats caches.
	redis redis.Options
}

func configFromEnv(getenv func(string) string) (config, error) {
	c := config{receiver: getenv("IPN_EMAIL"), port: getenv("IPN_PORT"), sandbox: getenv("IPN_SANDBOX") != "",
		redis: redis.Options{Addr: getenv("REDIS_ADDR"), Username: getenv("REDIS_USER"), Password: getenv("REDIS_PASS")}}
	for _, key := range []string{"POSTGRES_ADDR", "IPN_POSTGRES_USER", "IPN_POSTGRES_PASS", "IPN_EMAIL"} {
		if getenv(key) == "" {
			return c, fmt.Errorf("%s is required", key)
		}
	}
	c.postgresURL = storage.ConstructPsqlConnectURL(getenv("POSTGRES_ADDR"), getenv("IPN_POSTGRES_USER"), getenv("IPN_POSTGRES_PASS"))
	if c.port == "" {
		c.port = "3000"
	}
	if port, err := strconv.Atoi(c.port); err != nil || port < 1 || port > 65535 {
		return c, errors.New("IPN_PORT must be between 1 and 65535")
	}
	return c, nil
}

func main() {
	logging.Setup(os.Stdout)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := configFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	log.Printf("ipn %s-%s (sandbox=%t)", version, commit, cfg.sandbox)
	startupCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	pool, err := pgxpool.Connect(startupCtx, cfg.postgresURL)
	if err != nil {
		return fmt.Errorf("connect Postgres: %w", err)
	}
	defer pool.Close()
	if err := ipn.CheckSchema(startupCtx, pool); err != nil {
		return err
	}

	listener := &ipn.Listener{DB: pool, Verifier: ipn.NewPayPalVerifier(cfg.sandbox), Receiver: cfg.receiver, Sandbox: cfg.sandbox}
	if cfg.redis.Addr != "" {
		client := redis.NewClient(&cfg.redis)
		defer client.Close()
		if err := client.Ping(startupCtx).Err(); err != nil {
			return fmt.Errorf("connect Redis: %w", err)
		}
		listener.Announce = func(ctx context.Context, guildIDs ...string) error {
			return notice.AnnounceStatsChanged(ctx, client, guildIDs...)
		}
	} else {
		log.Println("No REDIS_ADDR; premium changes are not announced to the API, whose stats pages update on their cache TTL")
	}
	mux := http.NewServeMux()
	mux.Handle("/paypal-ipn", listener)
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("ok"))
	})

	go reconcile(ctx, listener)

	server := &http.Server{Addr: net.JoinHostPort("", cfg.port), Handler: mux,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	log.Printf("IPN listening on %s", server.Addr)
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		// Let in-flight notifications commit; PayPal retries any that do not.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			server.Close()
			return err
		}
		return nil
	}
}

func reconcile(ctx context.Context, l *ipn.Listener) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		if err := l.Reconcile(runCtx); err != nil {
			log.Printf("ipn: reconcile: %v", err)
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
