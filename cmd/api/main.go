// api serves AutoMuteUs HTTP endpoints independently of Discord bot shards.
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

	"github.com/automuteus/automuteus/v8/internal/api"
	pgstorage "github.com/automuteus/automuteus/v8/pkg/storage"
	"github.com/automuteus/automuteus/v8/storage"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Set by the same release tag/commit ldflags as the bot and Galactus.
var (
	version = "dev"
	commit  = "none"
)

type config struct {
	api         api.Config
	redis       redis.Options
	postgresURL string
	port        string
}

func configFromEnv(getenv func(string) string) (config, error) {
	c := config{api: api.Config{Version: version, Commit: commit, ServerURL: getenv("API_SERVER_URL"),
		AdminPassword: getenv("API_ADMIN_PASS"), CaptureHost: getenv("HOST"), Official: getenv("AUTOMUTEUS_OFFICIAL") != ""},
		redis: redis.Options{Addr: getenv("REDIS_ADDR"), Username: getenv("REDIS_USER"), Password: getenv("REDIS_PASS")}, port: getenv("API_PORT")}
	for _, key := range []string{"REDIS_ADDR", "POSTGRES_ADDR", "POSTGRES_USER", "POSTGRES_PASS"} {
		if getenv(key) == "" {
			return c, fmt.Errorf("%s is required", key)
		}
	}
	c.postgresURL = pgstorage.ConstructPsqlConnectURL(getenv("POSTGRES_ADDR"), getenv("POSTGRES_USER"), getenv("POSTGRES_PASS"))
	if c.port == "" {
		c.port = "5000"
	}
	port, err := strconv.Atoi(c.port)
	if err != nil || port < 1 || port > 65535 {
		return c, errors.New("API_PORT must be between 1 and 65535")
	}
	if c.api.CaptureHost == "" {
		c.api.CaptureHost = "http://localhost:8123"
	}
	return c, nil
}

// @title AutoMuteUs
// @version dev
// @description AutoMuteUs API, served independently of Discord bot shards.
// @BasePath /
// @securityDefinitions.basic BasicAuth
func main() {
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
	log.Printf("api %s-%s", version, commit)
	startupCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	client := redis.NewClient(&cfg.redis)
	defer client.Close()
	if err := client.Ping(startupCtx).Err(); err != nil {
		return fmt.Errorf("connect Redis: %w", err)
	}
	pool, err := pgxpool.Connect(startupCtx, cfg.postgresURL)
	if err != nil {
		return fmt.Errorf("connect Postgres: %w", err)
	}
	defer pool.Close()
	if err := storage.ApplySchemas(startupCtx, pool, cfg.api.Official); err != nil {
		return err
	}
	server := &http.Server{Addr: net.JoinHostPort("", cfg.port), Handler: api.NewRouter(cfg.api, api.NewStore(client, pool, cfg.api)),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	log.Printf("API listening on %s", server.Addr)
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			server.Close()
			return err
		}
		return nil
	}
}
